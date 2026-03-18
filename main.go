package main

import (
	"flag"
	"log/slog"
	"net/http"
	"os"

	"github.com/pionus/arry"
	"github.com/pionus/arry/middlewares"
	"github.com/pionus/pionus-go/internal/handler"
	"github.com/pionus/pionus-go/internal/migrate"
	"github.com/pionus/pionus-go/internal/store"
)

func main() {
	doMigrate := flag.Bool("migrate", false, "migrate markdown files to SQLite")
	configPath := flag.String("config", "config.yaml", "config file path")
	flag.Parse()

	cfg, err := LoadConfig(*configPath)
	if err != nil {
		slog.Error("load config", "err", err)
		os.Exit(1)
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
	app.Use(middlewares.Auth(cfg.AuthToken))

	themePath := "./theme/" + cfg.Theme
	app.Static("/assets", themePath+"/assets")
	app.Static("/node_modules", themePath+"/node_modules")
	app.Static("/web_modules", themePath+"/web_modules")
	app.Views(themePath + "/pages")

	router := app.Router()

	// Health check
	router.Get("/health", func(ctx arry.Context) {
		ctx.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	// SSR pages
	router.Get("/", handler.Index(db))
	router.Get("/article", handler.ArticleList(db))
	router.Get("/article/:slug", handler.ArticleDetail(db))

	// REST API
	router.Get("/api/articles", handler.ListArticles(db))
	router.Get("/api/articles/:slug", handler.GetArticle(db))
	router.Post("/api/articles", handler.CreateArticle(db))
	router.Put("/api/articles/:slug", handler.UpdateArticle(db))
	router.Delete("/api/articles/:slug", handler.DeleteArticle(db))

	slog.Info("starting server", "addr", cfg.Server.Addr)
	if cfg.TLS.Cert != "" {
		// StartTLS uses autocert with domain whitelist
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
