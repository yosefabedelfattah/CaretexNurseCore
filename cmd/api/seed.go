package main

import (
	"github.com/caretex/caretexnursing.core/internal/models"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// seedDevData inserts a default org + admin/nurse users + departments,
// statuses, attributes, and residents for local dev. Idempotent.
func seedDevData(db *gorm.DB) error {
	var existing models.User
	if err := db.Where("email = ?", "admin@caretex.local").First(&existing).Error; err == nil {
		return nil
	}

	orgID := uuid.New()
	if err := db.Create(&models.Organization{
		Base: models.Base{ID: orgID},
		Name: "Demo LTC Center",
		Code: "DEMO",
	}).Error; err != nil {
		return err
	}

	// No seed departments — real ones come from Caretex sync via /org-departments.

	// --- Roles ---
	roleAdmin := models.Role{
		Base: models.Base{ID: uuid.New()},
		Name: "admin",
		Permissions: models.PermissionList{
			"residents:read", "residents:write", "residents:delete",
			"departments:read", "catalog:read",
			"tasks:read", "tasks:write",
			"admin:integration",
		},
	}
	roleNurse := models.Role{
		Base: models.Base{ID: uuid.New()},
		Name: "nurse",
		Permissions: models.PermissionList{
			"residents:read", "residents:write",
			"departments:read", "catalog:read",
			"tasks:read", "tasks:write",
		},
	}
	if err := db.Create(&[]models.Role{roleAdmin, roleNurse}).Error; err != nil {
		return err
	}

	// --- Users ---
	hash, _ := bcrypt.GenerateFromPassword([]byte("Admin123!"), bcrypt.DefaultCost)
	admin := models.User{
		Base:           models.Base{ID: uuid.New()},
		OrganizationID: orgID,
		Email:          "admin@caretex.local",
		PasswordHash:   string(hash),
		FullName:       "Demo Admin",
		Status:         "active",
		Role:           "admin",
	}
	hash2, _ := bcrypt.GenerateFromPassword([]byte("Nurse123!"), bcrypt.DefaultCost)
	nurse := models.User{
		Base:           models.Base{ID: uuid.New()},
		OrganizationID: orgID,
		Email:          "nurse@caretex.local",
		PasswordHash:   string(hash2),
		FullName:       "Demo Nurse",
		Status:         "active",
		Role:           "nurse",
	}
	if err := db.Create(&[]models.User{admin, nurse}).Error; err != nil {
		return err
	}
	if err := db.Create(&[]models.UserRole{
		{Base: models.Base{ID: uuid.New()}, UserID: admin.ID, RoleID: roleAdmin.ID},
		{Base: models.Base{ID: uuid.New()}, UserID: nurse.ID, RoleID: roleNurse.ID},
	}).Error; err != nil {
		return err
	}

	// --- Demo caregivers ---
	// Visible in the assignment picker out of the box so dev/QA can test
	// caregiver assignment without first running the Caretx user sync.
	// PasswordHash "!" is bcrypt-incompatible — these accounts can't log in.
	demoCaregivers := []models.User{
		{Base: models.Base{ID: uuid.New()}, OrganizationID: orgID, Email: "yael.cohen@demo.local",   FullName: "יעל כהן",       PasswordHash: "!", Status: "active", Role: "nurse"},
		{Base: models.Base{ID: uuid.New()}, OrganizationID: orgID, Email: "amir.levi@demo.local",    FullName: "אמיר לוי",       PasswordHash: "!", Status: "active", Role: "aide"},
		{Base: models.Base{ID: uuid.New()}, OrganizationID: orgID, Email: "dana.peretz@demo.local",  FullName: "דנה פרץ",        PasswordHash: "!", Status: "active", Role: "aide"},
		{Base: models.Base{ID: uuid.New()}, OrganizationID: orgID, Email: "yoav.sela@demo.local",    FullName: "יואב סלע",       PasswordHash: "!", Status: "active", Role: "physio"},
	}
	if err := db.Create(&demoCaregivers).Error; err != nil {
		return err
	}

	// --- Status catalog (from screenshot) ---
	statuses := []models.ResidentStatusCode{
		{Base: models.Base{ID: uuid.New()}, OrganizationID: orgID, Code: "isolation", NameHe: "בידוד", NameEn: "Isolation", SortOrder: 1, Active: true},
		{Base: models.Base{ID: uuid.New()}, OrganizationID: orgID, Code: "hospitalized", NameHe: "באשפוז", NameEn: "Hospitalized", SortOrder: 2, Active: true},
		{Base: models.Base{ID: uuid.New()}, OrganizationID: orgID, Code: "on_vacation", NameHe: "דייר בחופשה", NameEn: "On vacation", SortOrder: 3, Active: true},
		{Base: models.Base{ID: uuid.New()}, OrganizationID: orgID, Code: "fall_risk", NameHe: "סיכון לנפילה", NameEn: "Fall risk", SortOrder: 4, Active: true},
		{Base: models.Base{ID: uuid.New()}, OrganizationID: orgID, Code: "independent_toiletries", NameHe: "עצמאי בשירותים", NameEn: "Independent in toiletries", SortOrder: 5, Active: true},
	}
	if err := db.Create(&statuses).Error; err != nil {
		return err
	}

	// --- Attributes catalog (from June 2024 release) ---
	attrs := []models.ResidentAttribute{
		{Base: models.Base{ID: uuid.New()}, OrganizationID: orgID, Code: "communication", NameHe: "יש/אין תקשורת עם דייר", NameEn: "Communication with resident", Category: "communication", SortOrder: 1, Active: true},
		{Base: models.Base{ID: uuid.New()}, OrganizationID: orgID, Code: "electric_mattress", NameHe: "מזרן חשמלי דינאמי", NameEn: "Electric dynamic mattress", Category: "equipment", SortOrder: 2, Active: true},
		{Base: models.Base{ID: uuid.New()}, OrganizationID: orgID, Code: "bed_shower", NameHe: "מקלחת במיטה", NameEn: "Bed shower", Category: "care", SortOrder: 3, Active: true},
		{Base: models.Base{ID: uuid.New()}, OrganizationID: orgID, Code: "heel_guard", NameHe: "מגן עקב", NameEn: "Heel guard", Category: "equipment", SortOrder: 4, Active: true},
		{Base: models.Base{ID: uuid.New()}, OrganizationID: orgID, Code: "wheelchair_no_brakes", NameHe: "ללא מעצורים בכיסא גלגלים", NameEn: "Wheelchair without brakes", Category: "equipment", SortOrder: 5, Active: true},
	}
	if err := db.Create(&attrs).Error; err != nil {
		return err
	}

	// No sample residents — real data comes from Caretex sync.
	return nil
}
