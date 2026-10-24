package main

import (
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
	"github.com/pionus/pionus-go/internal/handler"
	"github.com/pionus/pionus-go/internal/migrate"
	"github.com/pionus/pionus-go/internal/model"
	"github.com/pionus/pionus-go/internal/store"
	"github.com/pionus/pionus-go/internal/typecho"
)

func main() {
	doMigrate := flag.Bool("migrate", false, "migrate markdown files to SQLite")
	configPath := flag.String("config", "config.yaml", "config file path")
	createAdmin := flag.String("create-admin", "", "create or reset an admin account with this username")
	adminPassword := flag.String("admin-password", "", "password for -create-admin (or PIONUS_ADMIN_PASSWORD)")
	adminEmail := flag.String("admin-email", "", "email for -create-admin")
	importTypecho := flag.String("import-typecho", "", "import a Typecho JSON export produced by tools/typecho-export.php")
	typechoUploads := flag.String("typecho-uploads", "", "path to the old Typecho usr/uploads directory")
	typechoDryRun := flag.Bool("typecho-dry-run", false, "report what -import-typecho would do without writing")
	typechoDrafts := flag.Bool("typecho-drafts", false, "also import Typecho post_draft autosave rows")
	flag.Parse()

	cfg, err := LoadConfig(*configPath)
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

	if *importTypecho != "" {
		if err := runTypechoImport(db, *importTypecho, *typechoUploads, cfg.Upload.Dir, *typechoDryRun, *typechoDrafts); err != nil {
			slog.Error("import typecho", "err", err)
			os.Exit(1)
		}
		return
	}

	if err := os.MkdirAll(cfg.Upload.Dir, 0755); err != nil {
		slog.Error("create upload dir", "err", err)
		os.Exit(1)
	}

	_ = db.DeleteExpiredSessions(time.Now())

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

	themePath := "./theme/" + cfg.Theme
	app.Static("/assets", themePath+"/assets")
	app.Static("/node_modules", themePath+"/node_modules")
	app.Static("/web_modules", themePath+"/web_modules")
	app.Static("/uploads", cfg.Upload.Dir)
	app.Views(themePath + "/pages")

	if cfg.AuthToken == "" {
		slog.Warn("PIONUS_AUTH_TOKEN is empty: token-authenticated API writes are disabled")
	}
	if !cfg.Session.Secure {
		slog.Warn("session cookies are not marked Secure; enable TLS or set session.secure in config")
	}

	router := app.Router()

	// Health check
	router.Get("/health", func(ctx arry.Context) {
		ctx.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	// SSR pages
	router.Get("/", handler.Index(db))
	router.Get("/article", handler.ArticleList(db))
	router.Get("/article/:slug", handler.ArticleDetail(db))

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

	// Legacy URL redirects (must be registered last; static routes win).
	router.Get("/*", handler.Redirects(db))

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

// runTypechoImport imports a Typecho JSON export.
func runTypechoImport(db store.Store, path, sourceUploads, destUploads string, dryRun, drafts bool) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open export: %w", err)
	}
	defer f.Close()

	report, err := typecho.Import(f, typecho.Options{
		Store:         db,
		SourceUploads: sourceUploads,
		DestUploads:   destUploads,
		DryRun:        dryRun,
		ImportDrafts:  drafts,
	})
	if err != nil {
		return err
	}

	out, _ := json.MarshalIndent(report, "", "  ")
	fmt.Println(string(out))
	return nil
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
