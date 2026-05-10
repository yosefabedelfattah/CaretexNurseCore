package services

import (
	"context"
	"time"

	"github.com/caretex/caretexnursing.core/internal/dto"
	"github.com/caretex/caretexnursing.core/internal/integrations/caretx"
	"github.com/caretex/caretexnursing.core/internal/models"
	"github.com/caretex/caretexnursing.core/internal/repositories"
	apperr "github.com/caretex/caretexnursing.core/pkg/errors"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

type ResidentService interface {
	Create(ctx context.Context, orgID, actorID uuid.UUID, in dto.CreateResidentRequest) (*models.Resident, error)
	Update(ctx context.Context, orgID, actorID, id uuid.UUID, in dto.UpdateResidentRequest) (*models.Resident, error)
	Delete(ctx context.Context, orgID, id uuid.UUID) error
	Get(ctx context.Context, orgID, id uuid.UUID) (*models.Resident, error)
	List(ctx context.Context, orgID uuid.UUID, filter dto.ResidentListFilter) ([]models.Resident, int64, error)
}

type residentService struct {
	repo   repositories.ResidentRepository
	caretx caretx.Client
}

func NewResidentService(repo repositories.ResidentRepository, c caretx.Client) ResidentService {
	return &residentService{repo: repo, caretx: c}
}

func (s *residentService) Create(ctx context.Context, orgID, actorID uuid.UUID, in dto.CreateResidentRequest) (*models.Resident, error) {
	exists, err := s.repo.ExistsMRN(ctx, orgID, in.MRN, nil)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, apperr.ErrConflict
	}
	r := &models.Resident{
		Base:           models.Base{ID: uuid.New(), CreatedBy: &actorID, UpdatedBy: &actorID},
		OrganizationID: orgID,
		MRN:            in.MRN,
		FirstName:      in.FirstName,
		LastName:       in.LastName,
		DOB:            in.DOB,
		Gender:         in.Gender,
		RoomNumber:     in.RoomNumber,
		Phone:          in.Phone,
		Email:          in.Email,
		PhotoURL:       in.PhotoURL,
		Notes:          in.Notes,
	}
	if in.DepartmentID != nil && *in.DepartmentID != "" {
		if depID, err := uuid.Parse(*in.DepartmentID); err == nil {
			r.DepartmentID = &depID
		}
	}
	if err := s.repo.Create(ctx, r); err != nil {
		return nil, err
	}

	// Replace m2m relations if provided.
	if len(in.StatusIDs) > 0 {
		if err := s.repo.ReplaceStatuses(ctx, r.ID, parseUUIDs(in.StatusIDs)); err != nil {
			return nil, err
		}
	}
	if len(in.AttributeIDs) > 0 {
		if err := s.repo.ReplaceAttributes(ctx, r.ID, parseUUIDs(in.AttributeIDs)); err != nil {
			return nil, err
		}
	}

	go s.pushToCaretx(r)
	return s.repo.FindByID(ctx, orgID, r.ID, true)
}

func (s *residentService) Update(ctx context.Context, orgID, actorID, id uuid.UUID, in dto.UpdateResidentRequest) (*models.Resident, error) {
	r, err := s.repo.FindByID(ctx, orgID, id, false)
	if err != nil {
		return nil, err
	}
	if in.MRN != nil && *in.MRN != r.MRN {
		exists, err := s.repo.ExistsMRN(ctx, orgID, *in.MRN, &id)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, apperr.ErrConflict
		}
		r.MRN = *in.MRN
	}
	if in.FirstName != nil {
		r.FirstName = *in.FirstName
	}
	if in.LastName != nil {
		r.LastName = *in.LastName
	}
	if in.DOB != nil {
		r.DOB = in.DOB
	}
	if in.Gender != nil {
		r.Gender = *in.Gender
	}
	if in.RoomNumber != nil {
		r.RoomNumber = *in.RoomNumber
	}
	if in.Phone != nil {
		r.Phone = *in.Phone
	}
	if in.Email != nil {
		r.Email = *in.Email
	}
	if in.PhotoURL != nil {
		r.PhotoURL = *in.PhotoURL
	}
	if in.Notes != nil {
		r.Notes = *in.Notes
	}
	if in.DepartmentID != nil {
		if *in.DepartmentID == "" {
			r.DepartmentID = nil
		} else if depID, err := uuid.Parse(*in.DepartmentID); err == nil {
			r.DepartmentID = &depID
		}
	}
	r.UpdatedBy = &actorID

	if err := s.repo.Update(ctx, r); err != nil {
		return nil, err
	}
	if in.StatusIDs != nil {
		if err := s.repo.ReplaceStatuses(ctx, r.ID, parseUUIDs(*in.StatusIDs)); err != nil {
			return nil, err
		}
	}
	if in.AttributeIDs != nil {
		if err := s.repo.ReplaceAttributes(ctx, r.ID, parseUUIDs(*in.AttributeIDs)); err != nil {
			return nil, err
		}
	}

	go s.pushToCaretx(r)
	return s.repo.FindByID(ctx, orgID, r.ID, true)
}

func (s *residentService) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	r, err := s.repo.FindByID(ctx, orgID, id, false)
	if err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, orgID, id); err != nil {
		return err
	}
	if r.CaretxID != "" {
		go func(extID string) {
			if err := s.caretx.DeleteResident(context.Background(), extID); err != nil {
				log.Warn().Err(err).Str("caretx_id", extID).Msg("caretx delete failed")
			}
		}(r.CaretxID)
	}
	return nil
}

func (s *residentService) Get(ctx context.Context, orgID, id uuid.UUID) (*models.Resident, error) {
	return s.repo.FindByID(ctx, orgID, id, true)
}

func (s *residentService) List(ctx context.Context, orgID uuid.UUID, filter dto.ResidentListFilter) ([]models.Resident, int64, error) {
	return s.repo.List(ctx, orgID, filter)
}

// --- helpers ---

func (s *residentService) pushToCaretx(r *models.Resident) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	id, err := s.caretx.UpsertResident(ctx, r)
	if err != nil {
		log.Warn().Err(err).Str("resident_id", r.ID.String()).Msg("caretx upsert failed (will be retried by worker)")
		return
	}
	if id != "" {
		log.Debug().Str("resident_id", r.ID.String()).Str("caretx_id", id).Msg("caretx upsert ok")
	}
}

func parseUUIDs(ss []string) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(ss))
	for _, s := range ss {
		if id, err := uuid.Parse(s); err == nil {
			out = append(out, id)
		}
	}
	return out
}
