// Package server wires the public site, API and admin area into an arry app.
package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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
	"github.com/pionus/pionus-go/internal/plugin"
	"github.com/pionus/pionus-go/internal/store"
	"github.com/pionus/pionus-go/plugins"
	"github.com/pionus/pionus-go/web"
)

// App is the HTTP application plus extension lifecycle.
type App struct {
	*arry.Arry
	plugins *plugin.Manager
}

// StopPlugins stops extension workers. Call it after the HTTP server exits.
func (a *App) StopPlugins(ctx context.Context) error {
	if a == nil || a.plugins == nil {
		return nil
	}
	return a.plugins.Stop(ctx)
}

// New builds the configured HTTP application.
func New(cfg *config.Config, db store.Store) (*App, error) {
	if err := os.MkdirAll(cfg.Upload.Dir, 0755); err != nil {
		return nil, err
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

	secret := cfg.Security.Secret
	if secret == "" {
		buf := make([]byte, 32)
		if _, err := rand.Read(buf); err != nil {
			return nil, err
		}
		secret = hex.EncodeToString(buf)
		slog.Warn("security.secret is empty; using an ephemeral secret; password-post cookies will not survive a restart")
	}
	postGuard := handler.NewPostGuard(secret, cfg.Session.Secure)

	app := arry.New()
	if err := app.SetTrustedProxies(cfg.Server.TrustedProxies); err != nil {
		return nil, err
	}
	app.Use(middlewares.CORS())
	app.Use(middlewares.Gzip)
	if cfg.Server.AccessLog == "" || cfg.Server.AccessLog == "-" {
		app.Use(middlewares.Logger())
	} else {
		os.MkdirAll("logs", 0755)
		app.Use(middlewares.LoggerToFile(cfg.Server.AccessLog))
	}
	app.Use(middlewares.PanicWithHandler(func(ctx arry.Context, err interface{}) {
		ctx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "internal server error",
		})
	}))
	app.Use(session.Middleware())

	themeTemplates, themeAssets, err := web.ThemeFilesystems(cfg.Theme)
	if err != nil {
		return nil, err
	}
	if cfg.Theme != "" {
		slog.Info("using external theme", "dir", cfg.Theme)
	}
	app.StaticFSWithConfig("/assets", themeAssets, arry.StaticConfig{MaxAge: time.Hour})
	app.StaticWithConfig("/uploads", arry.StaticConfig{Dir: cfg.Upload.Dir, MaxAge: 24 * time.Hour})
	app.ViewsWithConfig(web.EngineConfigWithFS(themeTemplates))

	pluginManager, err := plugin.NewManager(plugins.Factories(), cfg.Plugins, plugin.Env{
		Config: cfg,
		Store:  db,
		App:    app,
		Log:    slog.Default(),
	})
	if err != nil {
		return nil, err
	}
	if err := pluginManager.Init(context.Background()); err != nil {
		return nil, err
	}
	deps := handler.NewDeps(db, pluginManager)

	if cfg.AuthToken == "" {
		slog.Warn("PIONUS_AUTH_TOKEN is empty: token-authenticated API writes are disabled")
	}
	if !cfg.Session.Secure {
		slog.Warn("session cookies are not marked Secure; enable TLS or set session.secure in config")
	}

	router := app.Router()

	router.Get("/health", func(ctx arry.Context) {
		pingCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := db.Ping(pingCtx); err != nil {
			ctx.JSON(http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
			return
		}
		ctx.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	commentCSRF := middlewares.CSRFWithConfig(middlewares.CSRFConfig{
		CookieName: "pionus_csrf",
		Secure:     cfg.Session.Secure,
		MaxAge:     12 * 60 * 60,
	})
	withCommentForm := func(next arry.Handler) arry.Handler {
		return commentCSRF(middlewares.BodyLimit(64 << 10)(next))
	}

	router.Get("/", handler.Index(deps))
	router.Get("/article", handler.ArticleList(deps))
	router.Get("/article/:slug", withCommentForm(handler.ArticleDetail(deps, postGuard)))
	router.Post("/article/:slug/unlock", withCommentForm(handler.UnlockPost(deps, postGuard)))
	router.Post("/article/:slug/comment", withCommentForm(handler.PostComment(deps, postGuard)))
	router.Get("/category/:slug", handler.CategoryArchive(deps))
	router.Get("/tag/:slug", handler.TagArchive(deps, postGuard))
	router.Post("/tag/:slug/unlock", withCommentForm(handler.UnlockTag(deps, postGuard)))
	router.Get("/archives/:year", handler.DateArchive(deps))
	router.Get("/archives/:year/:month", handler.DateArchive(deps))
	router.Get("/archives/:year/:month/:day", handler.DateArchive(deps))
	router.Get("/search", handler.Search(deps))
	router.Get("/notes", handler.NotesPage(deps))
	router.Get("/notes/:id", handler.NoteDetail(deps))
	router.Get("/categories", handler.CategoriesPage(deps))
	router.Get("/tags", handler.TagsPage(deps))
	router.Get("/feed", handler.Feed(deps))
	router.Get("/feed/notes", handler.NotesFeed(deps))
	router.Get("/feed/comments", handler.CommentsFeed(deps))
	router.Get("/sitemap.xml", handler.Sitemap(deps))
	router.Get("/robots.txt", handler.Robots(deps))

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
	api.Get("/posts", handler.ListPosts(deps))
	api.Get("/posts/:slug", handler.GetPost(deps, postGuard))
	api.Post("/posts", handler.CreatePost(deps))
	api.Put("/posts/:slug", handler.UpdatePost(deps))
	api.Delete("/posts/:slug", handler.DeletePost(deps))
	api.Get("/articles", handler.ListPosts(deps))
	api.Get("/articles/:slug", handler.GetPost(deps, postGuard))
	api.Post("/articles", handler.CreatePost(deps))
	api.Put("/articles/:slug", handler.UpdatePost(deps))
	api.Delete("/articles/:slug", handler.DeletePost(deps))
	router.Graft("/api", api)

	router.Get("/*", handler.NotFound(deps))

	adminArea := admin.New(admin.Options{
		Store:       db,
		Auth:        session,
		Preview:     postGuard,
		Events:      pluginManager,
		UploadDir:   cfg.Upload.Dir,
		MaxUploadMB: cfg.Upload.MaxSizeMB,
	})
	adminArea.Register(app)

	if err := pluginManager.Start(context.Background()); err != nil {
		return nil, err
	}

	return &App{Arry: app, plugins: pluginManager}, nil
}
