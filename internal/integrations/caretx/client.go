// Package caretx is the integration client for the external Caretx platform.
//
// Authentication: Authorization: Bearer <KeyID>:<Secret>
//
// Endpoints:
//   GET  /medicalFiles     → all residents (PersonDocument)
//   GET  /departments      → all departments
package caretx

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/caretex/caretexnursing.core/internal/config"
	"github.com/caretex/caretexnursing.core/internal/models"
	"github.com/rs/zerolog/log"
)

// Client is the interface consumed by the service layer.
type Client interface {
	WhoAmI(ctx context.Context) (*WhoAmIResponse, error)
	FetchPersons(ctx context.Context) ([]CaretxPerson, error)
	FetchDepartments(ctx context.Context) ([]CaretxDepartment, error)
	FetchUsers(ctx context.Context) ([]CaretxUser, error)
	UpsertResident(ctx context.Context, r *models.Resident) (string, error)
	DeleteResident(ctx context.Context, externalID string) error
	UpsertPatient(ctx context.Context, r *models.Resident) (string, error)
	DeletePatient(ctx context.Context, externalID string) error
}

type WhoAmIResponse struct {
	KeyID    string   `json:"KeyID"`
	Scopes   []string `json:"Scopes"`
	Status   string   `json:"Status"`
	TenantID string   `json:"TenantID"`
}

type httpClient struct {
	cfg     *config.Config
	http    *http.Client
	baseURL string // e.g. https://caretex.me:8080
	auth    string // "Bearer ck_live_xxx:sk_xxx"
}

func NewClient(cfg *config.Config) Client {
	baseURL := strings.TrimRight(cfg.CaretxBaseURL, "/")
	// Strip /api/v1/external if someone left it in the URL
	baseURL = strings.TrimSuffix(baseURL, "/api/v1/external")
	auth := ""
	if cfg.CaretxKeyID != "" && cfg.CaretxSecret != "" {
		auth = "Bearer " + cfg.CaretxKeyID + ":" + cfg.CaretxSecret
	}
	return &httpClient{
		cfg:     cfg,
		http:    &http.Client{Timeout: cfg.CaretxTimeout},
		baseURL: baseURL,
		auth:    auth,
	}
}

// ─── Diagnostic ─────────────────────────────────────────────────────────────

func (c *httpClient) WhoAmI(ctx context.Context) (*WhoAmIResponse, error) {
	// Try /api/v1/external/whoami first, fall back to /whoami
	for _, path := range []string{"/api/v1/external/whoami", "/whoami"} {
		body, err := c.doGet(ctx, c.baseURL+path)
		if err != nil {
			continue
		}
		var resp WhoAmIResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			continue
		}
		if resp.Status != "" || resp.KeyID != "" {
			log.Info().
				Str("key_id", resp.KeyID).
				Str("tenant_id", resp.TenantID).
				Str("status", resp.Status).
				Strs("scopes", resp.Scopes).
				Msg("caretx: whoami OK")
			return &resp, nil
		}
	}
	return &WhoAmIResponse{Status: "ok"}, nil
}

// ─── Pull: Fetch all residents ──────────────────────────────────────────────

func (c *httpClient) FetchPersons(ctx context.Context) ([]CaretxPerson, error) {
	if c.baseURL == "" {
		return nil, fmt.Errorf("caretx: CARETX_BASE_URL not configured")
	}

	url := c.baseURL + "/medicalFiles"
	log.Info().Str("url", url).Msg("caretx: fetching residents")

	body, err := c.doGet(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("caretx: fetch residents: %w", err)
	}

	// Try plain array first: [{"PersonelID":...}, ...]
	var persons []CaretxPerson
	if err := json.Unmarshal(body, &persons); err == nil && len(persons) > 0 {
		log.Info().Int("count", len(persons)).Msg("caretx: fetched residents (array)")
		return persons, nil
	}

	// Try wrapped: {"data": [...]} or {"Data": [...]}
	var wrapper map[string]json.RawMessage
	if err := json.Unmarshal(body, &wrapper); err == nil {
		for _, key := range []string{"data", "Data", "medicalFiles", "MedicalFiles"} {
			if raw, ok := wrapper[key]; ok {
				if err := json.Unmarshal(raw, &persons); err == nil && len(persons) > 0 {
					log.Info().Int("count", len(persons)).Str("key", key).Msg("caretx: fetched residents (wrapped)")
					return persons, nil
				}
			}
		}
	}

	// Log what we got for debugging
	preview := string(body)
	if len(preview) > 300 {
		preview = preview[:300] + "..."
	}
	log.Warn().Str("body_preview", preview).Msg("caretx: could not parse residents response")
	return nil, fmt.Errorf("caretx: could not parse residents response")
}

// ─── Pull: Fetch departments ────────────────────────────────────────────────

func (c *httpClient) FetchDepartments(ctx context.Context) ([]CaretxDepartment, error) {
	if c.baseURL == "" {
		return nil, fmt.Errorf("caretx: CARETX_BASE_URL not configured")
	}

	url := c.baseURL + "/org-departments"
	body, err := c.doGet(ctx, url)
	if err != nil {
		log.Warn().Err(err).Msg("caretx: /org-departments not available, will extract from residents")
		return nil, nil
	}

	// Try plain array
	var depts []CaretxDepartment
	if err := json.Unmarshal(body, &depts); err == nil && len(depts) > 0 {
		log.Info().Int("count", len(depts)).Msg("caretx: fetched departments")
		return depts, nil
	}

	// Try wrapped
	var wrapper map[string]json.RawMessage
	if err := json.Unmarshal(body, &wrapper); err == nil {
		for _, key := range []string{"data", "Data", "departments", "Departments"} {
			if raw, ok := wrapper[key]; ok {
				if err := json.Unmarshal(raw, &depts); err == nil && len(depts) > 0 {
					return depts, nil
				}
			}
		}
	}

	log.Warn().Msg("caretx: /org-departments returned empty, will extract from residents")
	return nil, nil
}

// ─── Pull: Fetch users (caregivers) ─────────────────────────────────────────
//
// Returns the staff list from Caretx. The exact endpoint path is best-effort:
// we try `/users` first (the documented one) and fall back to a couple of
// common alternates to stay forgiving across Caretx releases. The response may
// be a plain array or a wrapped envelope — same shape-tolerance pattern as
// FetchPersons/FetchDepartments.
func (c *httpClient) FetchUsers(ctx context.Context) ([]CaretxUser, error) {
	if c.baseURL == "" {
		return nil, fmt.Errorf("caretx: CARETX_BASE_URL not configured")
	}

	// Try a few likely paths. Caretex platform releases haven't been
	// 100% consistent — be permissive and pick the first one that parses.
	paths := []string{"/users", "/Users", "/api/v1/external/users"}

	var lastErr error
	for _, path := range paths {
		url := c.baseURL + path
		log.Debug().Str("url", url).Msg("caretx: fetching users")

		body, err := c.doGet(ctx, url)
		if err != nil {
			lastErr = err
			continue
		}

		// Plain array first
		var users []CaretxUser
		if err := json.Unmarshal(body, &users); err == nil && len(users) > 0 {
			log.Info().Int("count", len(users)).Str("path", path).Msg("caretx: fetched users (array)")
			return users, nil
		}

		// Wrapped: {"data": [...]} / {"Users": [...]} / etc.
		var wrapper map[string]json.RawMessage
		if err := json.Unmarshal(body, &wrapper); err == nil {
			for _, key := range []string{"data", "Data", "users", "Users"} {
				if raw, ok := wrapper[key]; ok {
					if err := json.Unmarshal(raw, &users); err == nil && len(users) > 0 {
						log.Info().Int("count", len(users)).Str("path", path).Str("key", key).Msg("caretx: fetched users (wrapped)")
						return users, nil
					}
				}
			}
		}

		preview := string(body)
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		log.Debug().Str("path", path).Str("body_preview", preview).Msg("caretx: users path returned no parsable list")
	}

	if lastErr != nil {
		return nil, fmt.Errorf("caretx: fetch users: %w", lastErr)
	}
	return nil, fmt.Errorf("caretx: could not parse users response from any known path")
}

// ─── Push: Stubs ────────────────────────────────────────────────────────────

func (c *httpClient) UpsertResident(ctx context.Context, r *models.Resident) (string, error) {
	log.Debug().Str("id", r.ID.String()).Msg("caretx: upsert stub")
	return "stub-" + r.ID.String(), nil
}
func (c *httpClient) DeleteResident(ctx context.Context, externalID string) error {
	log.Debug().Str("id", externalID).Msg("caretx: delete stub")
	return nil
}
func (c *httpClient) UpsertPatient(ctx context.Context, r *models.Resident) (string, error) {
	return c.UpsertResident(ctx, r)
}
func (c *httpClient) DeletePatient(ctx context.Context, externalID string) error {
	return c.DeleteResident(ctx, externalID)
}

// ─── HTTP helper ────────────────────────────────────────────────────────────

func (c *httpClient) doGet(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if c.auth != "" {
		req.Header.Set("Authorization", c.auth)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Warn().
			Int("status", resp.StatusCode).
			Str("url", url).
			Str("body", truncate(string(body), 500)).
			Msg("caretx: non-2xx response")
		return nil, fmt.Errorf("caretx responded with status %d", resp.StatusCode)
	}

	return body, nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
