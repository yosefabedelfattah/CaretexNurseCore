package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/caretex/caretexnursing.core/internal/config"
	"github.com/caretex/caretexnursing.core/internal/handlers"
	"github.com/caretex/caretexnursing.core/internal/integrations/caretx"
	"github.com/caretex/caretexnursing.core/internal/middleware"
	"github.com/caretex/caretexnursing.core/internal/models"
	"github.com/caretex/caretexnursing.core/internal/repositories"
	"github.com/caretex/caretexnursing.core/internal/services"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// @title Caretex Nurse API
// @version 1.0
// @description Healthcare nursing management API
// @BasePath /api/v1
func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})

	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load config")
	}
	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
		log.Logger = log.Output(os.Stderr)
	}
	zerolog.SetGlobalLevel(parseLogLevel(cfg.LogLevel))

	db, err := openDB(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database")
	}

	if cfg.AppEnv != "production" {
		if err := db.AutoMigrate(
			&models.Organization{},
			&models.User{},
			&models.Role{},
			&models.UserRole{},
			&models.RefreshToken{},
			&models.Department{},
			&models.ResidentStatusCode{},
			&models.ResidentAttribute{},
			&models.Resident{},
			&models.ResidentStatusAssignment{},
			&models.ResidentAttributeAssignment{},
			&models.TreatmentPlan{},
			&models.AssignedTask{},
		); err != nil {
			log.Fatal().Err(err).Msg("auto-migrate failed")
		}
		if err := seedDevData(db); err != nil {
			log.Warn().Err(err).Msg("dev seed failed")
		}
	}

	caretxClient := caretx.NewClient(cfg)

	userRepo := repositories.NewUserRepository(db)
	refreshRepo := repositories.NewRefreshTokenRepository(db)
	deptRepo := repositories.NewDepartmentRepository(db)
	catalogRepo := repositories.NewCatalogRepository(db)
	residentRepo := repositories.NewResidentRepository(db)
	taskRepo := repositories.NewTaskRepository(db)

	authService := services.NewAuthService(userRepo, refreshRepo, cfg)
	residentService := services.NewResidentService(residentRepo, caretxClient)
	caretxSyncService := services.NewCaretxSyncService(db, caretxClient)

	authHandler := handlers.NewAuthHandler(authService)
	residentHandler := handlers.NewResidentHandler(residentService, userRepo, deptRepo)
	deptHandler := handlers.NewDepartmentHandler(deptRepo)
	catalogHandler := handlers.NewCatalogHandler(catalogRepo)
	healthHandler := handlers.NewHealthHandler(db)
	syncHandler := handlers.NewSyncHandler(caretxSyncService, caretxClient)
	taskHandler := handlers.NewTaskHandler(taskRepo)
	userHandler := handlers.NewUserHandler(userRepo)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestID())
	r.Use(middleware.Logger())
	r.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.AllowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-Request-ID"},
		ExposeHeaders:    []string{"X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	r.GET("/healthz", healthHandler.Liveness)
	r.GET("/readyz", healthHandler.Readiness)

	v1 := r.Group("/api/v1")
	{
		auth := v1.Group("/auth")
		{
			auth.POST("/login", authHandler.Login)
			auth.POST("/refresh", authHandler.Refresh)
		}

		protected := v1.Group("")
		protected.Use(middleware.Auth(cfg))
		{
			protected.POST("/auth/logout", authHandler.Logout)
			protected.GET("/me", authHandler.Me)

			// Catalogs
			protected.GET("/departments", deptHandler.List)
			protected.GET("/departments/rooms", deptHandler.Rooms)
			protected.GET("/catalog/statuses", catalogHandler.ListStatuses)
			protected.GET("/catalog/attributes", catalogHandler.ListAttributes)

			// Residents
			residents := protected.Group("/residents")
			{
				residents.GET("", middleware.RequirePermission("residents:read"), residentHandler.List)
				residents.POST("", middleware.RequirePermission("residents:write"), residentHandler.Create)
				residents.GET("/:id", middleware.RequirePermission("residents:read"), residentHandler.Get)
				residents.PUT("/:id", middleware.RequirePermission("residents:write"), residentHandler.Update)
				residents.DELETE("/:id", middleware.RequirePermission("residents:delete"), residentHandler.Delete)
			}

			// Tasks
			tasks := protected.Group("/tasks")
			{
				tasks.GET("", taskHandler.List)
				tasks.POST("", taskHandler.Create)
				tasks.GET("/:id", taskHandler.Get)
				tasks.PUT("/:id", taskHandler.Update)
				tasks.PATCH("/:id/progress", taskHandler.Progress)
				tasks.DELETE("/:id", taskHandler.Delete)
			}

			// Users (staff list — used by caregiver pickers)
			protected.GET("/users", userHandler.List)

			// Caretex sync (admin only — pull data from external Caretex platform)
			sync := protected.Group("/sync")
			{
				sync.GET("/caretx/whoami", middleware.RequirePermission("residents:read"), syncHandler.WhoAmI)
				sync.POST("/caretx", middleware.RequirePermission("residents:write"), syncHandler.SyncAll)
				sync.POST("/caretx/departments", middleware.RequirePermission("residents:write"), syncHandler.SyncDepartments)
				sync.POST("/caretx/residents", middleware.RequirePermission("residents:write"), syncHandler.SyncResidents)
				sync.POST("/caretx/users", middleware.RequirePermission("residents:write"), syncHandler.SyncUsers)
			}
		}
	}

	srv := &http.Server{
		Addr:              ":" + cfg.AppPort,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// ── Auto-sync from Caretex on startup ────────────────────────────
	// Runs in background so it doesn't block the server from starting.
	// Also starts a periodic sync ticker if CARETX_SYNC_INTERVAL > 0.
	syncCtx, syncCancel := context.WithCancel(context.Background())
	log.Info().
		Str("base_url", cfg.CaretxBaseURL).
		Str("key_id", cfg.CaretxKeyID).
		Msg("auto-sync: Caretex config loaded")

	if cfg.CaretxBaseURL != "" && cfg.CaretxKeyID != "" {
		// Get the org ID — try Organization table first, fall back to first User's org
		var orgID uuid.UUID
		var defaultOrg models.Organization
		if err := db.First(&defaultOrg).Error; err == nil {
			orgID = defaultOrg.ID
			log.Info().Str("org_id", orgID.String()).Msg("auto-sync: found organization")
		} else {
			// Fallback: get org from the first user
			var firstUser models.User
			if err2 := db.First(&firstUser).Error; err2 == nil && firstUser.OrganizationID != uuid.Nil {
				orgID = firstUser.OrganizationID
				log.Info().Str("org_id", orgID.String()).Msg("auto-sync: using org from first user")
			} else {
				// Last resort: create a default org ID
				orgID = uuid.MustParse("00000000-0000-0000-0000-000000000001")
				log.Warn().Str("org_id", orgID.String()).Msg("auto-sync: no org found, using default")
			}
		}

		// Initial sync on startup (wait 3s for DB to settle)
		go func() {
			time.Sleep(3 * time.Second)
			log.Info().Msg("auto-sync: initial Caretex sync starting")
			result, err := caretxSyncService.SyncAll(syncCtx, orgID)
			if err != nil {
				log.Error().Err(err).Msg("auto-sync: initial sync failed")
			} else {
				log.Info().
					Int("dept_created", result.DepartmentsCreated).
					Int("dept_updated", result.DepartmentsUpdated).
					Int("res_created", result.ResidentsCreated).
					Int("res_updated", result.ResidentsUpdated).
					Int("res_archived", result.ResidentsArchived).
					Int("errors", result.Errors).
					Msg("auto-sync: initial sync complete")
			}
		}()

		// Periodic background sync (every 5 minutes)
		syncInterval := 5 * time.Minute
		go func() {
			ticker := time.NewTicker(syncInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					log.Debug().Msg("auto-sync: periodic Caretex sync starting")
					result, err := caretxSyncService.SyncAll(syncCtx, orgID)
					if err != nil {
						log.Warn().Err(err).Msg("auto-sync: periodic sync failed")
					} else {
						log.Debug().
							Int("res_created", result.ResidentsCreated).
							Int("res_updated", result.ResidentsUpdated).
							Msg("auto-sync: periodic sync complete")
					}
				case <-syncCtx.Done():
					log.Info().Msg("auto-sync: stopped")
					return
				}
			}
		}()
		log.Info().Dur("interval", syncInterval).Msg("auto-sync: periodic sync enabled")
	} else {
		log.Info().Msg("auto-sync: Caretex not configured (CARETX_BASE_URL or CARETX_KEY_ID empty)")
	}

	go func() {
		log.Info().Str("addr", srv.Addr).Str("env", cfg.AppEnv).Msg("server starting")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal().Err(err).Msg("server failed")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info().Msg("shutting down")
	syncCancel() // stop background sync
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("forced shutdown")
	}
}

func openDB(cfg *config.Config) (*gorm.DB, error) {
	dsn := cfg.DatabaseDSN()
	gormCfg := &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Warn)}
	db, err := gorm.Open(postgres.Open(dsn), gormCfg)
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(cfg.DBMaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.DBMaxIdleConns)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	return db, nil
}

func parseLogLevel(s string) zerolog.Level {
	lv, err := zerolog.ParseLevel(s)
	if err != nil {
		return zerolog.InfoLevel
	}
	return lv
}
