package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/pionus/pionus-go/internal/auth"
	"github.com/pionus/pionus-go/internal/config"
	"github.com/pionus/pionus-go/internal/imageaudit"
	"github.com/pionus/pionus-go/internal/migrate"
	"github.com/pionus/pionus-go/internal/model"
	"github.com/pionus/pionus-go/internal/server"
	"github.com/pionus/pionus-go/internal/store"
	"github.com/pionus/pionus-go/internal/version"
)

func main() {
	doMigrate := flag.Bool("migrate", false, "migrate markdown files to SQLite")
	configPath := flag.String("config", "config.yaml", "config file path")
	createAdmin := flag.String("create-admin", "", "create or reset an admin account with this username")
	adminPassword := flag.String("admin-password", "", "password for -create-admin (or PIONUS_ADMIN_PASSWORD)")
	adminEmail := flag.String("admin-email", "", "email for -create-admin")
	auditImages := flag.Bool("audit-images", false, "report missing local images and external image hosts")
	auditOnline := flag.Bool("audit-images-online", false, "also check external image URLs while auditing")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version.String())
		return
	}

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

	app, err := server.New(cfg, db)
	if err != nil {
		slog.Error("build server", "err", err)
		os.Exit(1)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := app.StopPlugins(ctx); err != nil {
			slog.Error("stop plugins", "err", err)
		}
	}()

	slog.Info("starting server", "version", version.Version, "addr", cfg.Server.Addr, "admin", "/admin")
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
