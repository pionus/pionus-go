package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/pionus/arry"
	"github.com/pionus/arry/middlewares"
	"github.com/pionus/pionus-go/internal/admin"
	"github.com/pionus/pionus-go/internal/auth"
	"github.com/pionus/pionus-go/internal/config"
	"github.com/pionus/pionus-go/internal/handler"
	"github.com/pionus/pionus-go/internal/imageaudit"
	"github.com/pionus/pionus-go/internal/migrate"
	"github.com/pionus/pionus-go/internal/model"
	"github.com/pionus/pionus-go/internal/store"
	"github.com/pionus/pionus-go/web"
)

func main() {
	doMigrate := flag.Bool("migrate", false, "migrate markdown files to SQLite")
	configPath := flag.String("config", "config.yaml", "config file path")
	createAdmin := flag.String("create-admin", "", "create or reset an admin account with this username")
	adminPassword := flag.String("admin-password", "", "password for -create-admin (or PIONUS_ADMIN_PASSWORD)")
	adminEmail := flag.String("admin-email", "", "email for -create-admin")
	auditImages := flag.Bool("audit-images", false, "report missing local images and external image hosts")
	auditOnline := flag.Bool("audit-images-online", false, "also check external image URLs while auditing")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("load config", "err", err)
		os.Exit(1)
	}
	if cfg.Timezone != "" {
		loc, err := time.LoadLocation(cfg.Timezone)
		if err != nil {
			slog.Error("load timezone", "timezone", cfg.Timezone, "err", err)
			os.Exit(1)
		}
		time.Local = loc
	}

	db, err := store.NewSQLite(cfg.Database.Path)
	if err != nil {
		slog.Error("open database", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	if *doMigrate {
		slog.Info("migrating markdowns to SQLite")
		if err := migrate.MarkdownToSQLite("markdowns", db); err != nil {
			slog.Error("migration failed", "err", err)
			os.Exit(1)
		}
		slog.Info("migration complete")
		return
	}

	if *createAdmin != "" {
		if err := upsertAdmin(db, *createAdmin, *adminPassword, *adminEmail); err != nil {
			slog.Error("create admin", "err", err)
			os.Exit(1)
		}
		return
	}

	if *auditImages {
		report, err := imageaudit.Audit(db, cfg.Upload.Dir, *auditOnline)
		if err != nil {
			slog.Error("audit images", "err", err)
			os.Exit(1)
		}
		out, _ := json.MarshalIndent(report, "", "  ")
		fmt.Println(string(out))
		return
	}

	if err := os.MkdirAll(cfg.Upload.Dir, 0755); err != nil {
		slog.Error("create upload dir", "err", err)
		os.Exit(1)
	}

	_ = db.DeleteExpiredSessions(time.Now())
	go func() {
		ticker := time.NewTicker(6 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			_ = db.DeleteExpiredSessions(time.Now())
		}
	}()

	session := auth.NewManager(
		db,
		cfg.Session.CookieName,
		time.Duration(cfg.Session.TTLHours)*time.Hour,
		cfg.Session.Secure,
	)

	app := arry.New()
	app.Use(middlewares.CORS())
	app.Use(middlewares.Gzip)
	os.MkdirAll("logs", 0755)
	app.Use(middlewares.LoggerToFile("logs/access.log"))
	app.Use(middlewares.PanicWithHandler(func(ctx arry.Context, err interface{}) {
		ctx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "internal server error",
		})
	}))
	app.Use(session.Middleware())

	app.StaticFS("/assets", web.AssetsFS())
	app.Static("/uploads", cfg.Upload.Dir)
	app.ViewsWithConfig(web.EngineConfig())

	if cfg.AuthToken == "" {
		slog.Warn("PIONUS_AUTH_TOKEN is empty: token-authenticated API writes are disabled")
	}
	if !cfg.Session.Secure {
		slog.Warn("session cookies are not marked Secure; enable TLS or set session.secure in config")
	}

	router := app.Router()

	// Health check
	router.Get("/health", func(ctx arry.Context) {
		pingCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := db.Ping(pingCtx); err != nil {
			ctx.JSON(http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
			return
		}
		ctx.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	// Public pages. The article page also sets the CSRF cookie used by the
	// native comment form.
	commentCSRF := middlewares.CSRFWithConfig(middlewares.CSRFConfig{
		CookieName: "pionus_csrf",
		Secure:     cfg.Session.Secure,
		MaxAge:     12 * 60 * 60,
	})
	withCommentForm := func(next arry.Handler) arry.Handler {
		return commentCSRF(middlewares.BodyLimit(64 << 10)(next))
	}

	router.Get("/", handler.Index(db))
	router.Get("/article", handler.ArticleList(db))
	router.Get("/article/:slug", withCommentForm(handler.ArticleDetail(db)))
	router.Post("/article/:slug/comment", withCommentForm(handler.PostComment(db)))
	router.Get("/category/:slug", handler.CategoryArchive(db))
	router.Get("/tag/:slug", handler.TagArchive(db))
	router.Get("/archives/:year", handler.DateArchive(db))
	router.Get("/archives/:year/:month", handler.DateArchive(db))
	router.Get("/archives/:year/:month/:day", handler.DateArchive(db))
	router.Get("/search", handler.Search(db))
	router.Get("/notes", handler.NotesPage(db))
	router.Get("/categories", handler.CategoriesPage(db))
	router.Get("/tags", handler.TagsPage(db))
	router.Get("/feed", handler.Feed(db))
	router.Get("/feed/comments", handler.CommentsFeed(db))
	router.Get("/sitemap.xml", handler.Sitemap(db))
	router.Get("/robots.txt", handler.Robots(db))

	// JSON API. A session login also authorizes writes; otherwise a bearer
	// token is required.
	tokenGuard := middlewares.Auth(cfg.AuthToken,
		http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete)
	apiAuth := func(next arry.Handler) arry.Handler {
		guarded := tokenGuard(next)
		return func(ctx arry.Context) {
			if auth.CurrentUser(ctx) != nil {
				ctx.Set("auth", true)
				next(ctx)
				return
			}
			guarded(ctx)
		}
	}

	api := arry.NewRouter(apiAuth)
	api.Get("/posts", handler.ListPosts(db))
	api.Get("/posts/:slug", handler.GetPost(db))
	api.Post("/posts", handler.CreatePost(db))
	api.Put("/posts/:slug", handler.UpdatePost(db))
	api.Delete("/posts/:slug", handler.DeletePost(db))

	// Legacy article endpoints kept as aliases.
	api.Get("/articles", handler.ListPosts(db))
	api.Get("/articles/:slug", handler.GetPost(db))
	api.Post("/articles", handler.CreatePost(db))
	api.Put("/articles/:slug", handler.UpdatePost(db))
	api.Delete("/articles/:slug", handler.DeletePost(db))
	router.Graft("/api", api)

	// Themed 404 for unmatched paths (must be registered last).
	router.Get("/*", handler.NotFound(db))

	// Admin area
	adminArea := admin.New(admin.Options{
		Store:       db,
		Auth:        session,
		UploadDir:   cfg.Upload.Dir,
		MaxUploadMB: cfg.Upload.MaxSizeMB,
	})
	adminArea.Register(app)

	slog.Info("starting server", "addr", cfg.Server.Addr, "admin", "/admin")
	if cfg.TLS.Cert != "" {
		// StartTLS uses autocert with the configured domain whitelist.
		if err := app.StartTLS(cfg.Server.Addr, cfg.TLS.Cert); err != nil {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	} else {
		if err := app.Start(cfg.Server.Addr); err != nil {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}
}

// upsertAdmin creates an owner account or resets an existing user's password.
// It allows bootstrapping an installation without visiting /admin/setup.
func upsertAdmin(db store.Store, username, password, email string) error {
	if !auth.ValidUsername(username) {
		return errInvalidUsername
	}
	if password == "" {
		password = os.Getenv("PIONUS_ADMIN_PASSWORD")
	}
	if len(password) < 8 {
		return errWeakPassword
	}
	if email == "" {
		email = username + "@example.com"
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}

	existing, err := db.GetUserByUsername(username)
	if err == nil {
		existing.PasswordHash = hash
		existing.Role = model.RoleOwner
		if err := db.UpdateUser(existing); err != nil {
			return err
		}
		_ = db.DeleteUserSessions(existing.ID)
		slog.Info("admin password reset", "username", username)
		return nil
	}

	user := &model.User{
		Username:     username,
		Email:        email,
		PasswordHash: hash,
		ScreenName:   username,
		Role:         model.RoleOwner,
	}
	if err := db.CreateUser(user); err != nil {
		return err
	}
	slog.Info("admin created", "username", username)
	return nil
}

type cliError string

func (e cliError) Error() string { return string(e) }

const (
	errInvalidUsername = cliError("username must be 3-32 letters, numbers, underscores or hyphens")
	errWeakPassword    = cliError("password must be at least 8 characters (use -admin-password or PIONUS_ADMIN_PASSWORD)")
)
