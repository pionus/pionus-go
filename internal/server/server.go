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
	"github.com/pionus/pionus-go/internal/store"
	"github.com/pionus/pionus-go/web"
)

// New builds the configured HTTP application.
func New(cfg *config.Config, db store.Store) (*arry.Arry, error) {
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

	app.StaticFSWithConfig("/assets", web.AssetsFS(), arry.StaticConfig{MaxAge: time.Hour})
	app.StaticWithConfig("/uploads", arry.StaticConfig{Dir: cfg.Upload.Dir, MaxAge: 24 * time.Hour})
	app.ViewsWithConfig(web.EngineConfig())

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

	router.Get("/", handler.Index(db))
	router.Get("/article", handler.ArticleList(db))
	router.Get("/article/:slug", withCommentForm(handler.ArticleDetail(db, postGuard)))
	router.Post("/article/:slug/unlock", withCommentForm(handler.UnlockPost(db, postGuard)))
	router.Post("/article/:slug/comment", withCommentForm(handler.PostComment(db, postGuard)))
	router.Get("/category/:slug", handler.CategoryArchive(db))
	router.Get("/tag/:slug", handler.TagArchive(db))
	router.Get("/archives/:year", handler.DateArchive(db))
	router.Get("/archives/:year/:month", handler.DateArchive(db))
	router.Get("/archives/:year/:month/:day", handler.DateArchive(db))
	router.Get("/search", handler.Search(db))
	router.Get("/notes", handler.NotesPage(db))
	router.Get("/notes/:id", handler.NoteDetail(db))
	router.Get("/categories", handler.CategoriesPage(db))
	router.Get("/tags", handler.TagsPage(db))
	router.Get("/feed", handler.Feed(db))
	router.Get("/feed/notes", handler.NotesFeed(db))
	router.Get("/feed/comments", handler.CommentsFeed(db))
	router.Get("/sitemap.xml", handler.Sitemap(db))
	router.Get("/robots.txt", handler.Robots(db))

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
	api.Get("/posts/:slug", handler.GetPost(db, postGuard))
	api.Post("/posts", handler.CreatePost(db))
	api.Put("/posts/:slug", handler.UpdatePost(db))
	api.Delete("/posts/:slug", handler.DeletePost(db))
	api.Get("/articles", handler.ListPosts(db))
	api.Get("/articles/:slug", handler.GetPost(db, postGuard))
	api.Post("/articles", handler.CreatePost(db))
	api.Put("/articles/:slug", handler.UpdatePost(db))
	api.Delete("/articles/:slug", handler.DeletePost(db))
	router.Graft("/api", api)

	router.Get("/*", handler.NotFound(db))

	adminArea := admin.New(admin.Options{
		Store:       db,
		Auth:        session,
		UploadDir:   cfg.Upload.Dir,
		MaxUploadMB: cfg.Upload.MaxSizeMB,
	})
	adminArea.Register(app)

	return app, nil
}
