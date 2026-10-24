// Command pionus-migrate imports a Typecho export, repairs links and audits
// images in one repeatable pass.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/pionus/pionus-go/internal/config"
	"github.com/pionus/pionus-go/internal/imageaudit"
	"github.com/pionus/pionus-go/internal/store"
	"github.com/pionus/pionus-go/internal/typecho"
)

type report struct {
	Import *typecho.Report             `json:"import,omitempty"`
	Links  *typecho.FixReport          `json:"links,omitempty"`
	Images *imageaudit.Report          `json:"images,omitempty"`
	Pages  *typecho.PageOverrideReport `json:"pages,omitempty"`
	Verify *verifyReport               `json:"verify,omitempty"`
}

type verifyReport struct {
	Posts      int `json:"posts"`
	Pages      int `json:"pages"`
	Comments   int `json:"comments"`
	Notes      int `json:"notes"`
	Categories int `json:"categories"`
	Tags       int `json:"tags"`
	OldLinks   int `json:"remaining_old_links"`
}

func main() {
	configPath := flag.String("config", "config.yaml", "config file path")
	exportPath := flag.String("export", "", "Typecho JSON export from tools/typecho-export.php")
	sourceUploads := flag.String("source-uploads", "", "old Typecho usr/uploads directory")
	destUploads := flag.String("dest-uploads", "", "new uploads directory (defaults to upload.dir)")
	dryRun := flag.Bool("dry-run", false, "report what would change without writing")
	importDrafts := flag.Bool("drafts", false, "also import Typecho post_draft autosave rows")
	skipImport := flag.Bool("skip-import", false, "only repair links and audit (no JSON import)")
	skipLinks := flag.Bool("skip-links", false, "do not repair old links")
	skipImages := flag.Bool("skip-images", false, "do not audit images")
	onlineAudit := flag.Bool("online-audit", false, "check external image URLs during audit")
	pagesDir := flag.String("pages", "migration/pages", "directory of <slug>.md page overrides (custom theme pages)")
	forcePages := flag.Bool("force-pages", false, "apply page overrides even with -skip-import (can overwrite edits)")
	backup := flag.Bool("backup", true, "create a timestamped SQLite backup before writing")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fatal("load config", err)
	}
	if cfg.Timezone != "" {
		loc, err := time.LoadLocation(cfg.Timezone)
		if err != nil {
			fatal("load timezone", err)
		}
		time.Local = loc
	}

	db, err := store.NewSQLite(cfg.Database.Path)
	if err != nil {
		fatal("open database", err)
	}
	defer db.Close()

	uploads := *destUploads
	if uploads == "" {
		uploads = cfg.Upload.Dir
	}

	if !*dryRun && *backup {
		backupPath := cfg.Database.Path + ".bak-" + time.Now().Format("20060102-150405")
		if err := db.Backup(backupPath); err != nil {
			fatal("backup database", err)
		}
		slog.Info("database backed up", "path", backupPath)
	}

	pageOverrides, err := typecho.LoadPageOverrides(*pagesDir)
	if err != nil {
		fatal("load page overrides", err)
	}
	if *skipImport && !*forcePages {
		pageOverrides = nil
	}

	var out report

	if !*skipImport {
		if *exportPath == "" {
			fatal("missing -export", fmt.Errorf("pass the JSON produced by tools/typecho-export.php, or use -skip-import"))
		}
		f, err := os.Open(*exportPath)
		if err != nil {
			fatal("open export", err)
		}
		importReport, err := typecho.Import(f, typecho.Options{
			Store:         db,
			SourceUploads: *sourceUploads,
			DestUploads:   uploads,
			DryRun:        *dryRun,
			ImportDrafts:  *importDrafts,
			PageOverrides: pageOverrides,
		})
		f.Close()
		if err != nil {
			fatal("import typecho", err)
		}
		out.Import = importReport
		printImport(importReport)
	} else if len(pageOverrides) > 0 {
		pageReport, err := typecho.ApplyPageOverrides(db, pageOverrides, *dryRun)
		if err != nil {
			fatal("apply page overrides", err)
		}
		out.Pages = pageReport
		slog.Info("page overrides", "applied", len(pageReport.Applied), "missing", pageReport.Missing)
	}

	if !*skipLinks {
		linkReport, err := typecho.FixLinks(db, *dryRun)
		if err != nil {
			fatal("fix links", err)
		}
		out.Links = linkReport
		slog.Info("link repair",
			"posts", linkReport.PostsScanned,
			"updated", linkReport.PostsUpdated,
			"comments", linkReport.CommentsScanned,
			"updated_comments", linkReport.CommentsUpdated,
			"remaining_old_links", len(linkReport.RemainingOldLinks))
	}

	if !*skipImages {
		imageReport, err := imageaudit.Audit(db, uploads, *onlineAudit)
		if err != nil {
			fatal("audit images", err)
		}
		out.Images = imageReport
		slog.Info("image audit",
			"references", imageReport.References,
			"local", imageReport.Local,
			"missing", imageReport.Missing,
			"external", imageReport.External,
			"unreachable", imageReport.Unreachable)
	}

	verify := verifyDatabase(db)
	if out.Links != nil {
		verify.OldLinks = len(out.Links.RemainingOldLinks)
	}
	out.Verify = verify
	slog.Info("database check",
		"posts", verify.Posts,
		"pages", verify.Pages,
		"comments", verify.Comments,
		"notes", verify.Notes,
		"categories", verify.Categories,
		"tags", verify.Tags,
		"remaining_old_links", verify.OldLinks)

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(&out); err != nil {
		fatal("write report", err)
	}

	slog.Info("migration pass complete",
		"dry_run", *dryRun,
		"db", cfg.Database.Path,
		"uploads", uploads)
}

func verifyDatabase(db *store.SQLiteStore) *verifyReport {
	v := &verifyReport{}
	if _, total, err := db.ListPosts(store.PostFilter{Type: "post", Page: 1, Limit: 1}); err == nil {
		v.Posts = total
	}
	if _, total, err := db.ListPosts(store.PostFilter{Type: "page", Page: 1, Limit: 1}); err == nil {
		v.Pages = total
	}
	if _, total, err := db.ListComments(store.CommentFilter{Page: 1, Limit: 1}); err == nil {
		v.Comments = total
	}
	if total, err := db.CountNotes(""); err == nil {
		v.Notes = total
	}
	if categories, err := db.ListCategories(); err == nil {
		v.Categories = len(categories)
	}
	if tags, err := db.ListTags(); err == nil {
		v.Tags = len(tags)
	}
	return v
}

func printImport(r *typecho.Report) {
	slog.Info("import complete",
		"users", r.Users,
		"categories", r.Categories,
		"tags", r.Tags,
		"posts", r.Posts,
		"pages", r.Pages,
		"comments", r.Comments,
		"notes", r.Notes,
		"pages_overridden", r.PagesOverridden,
		"attachments", r.Attachments,
		"updated", r.Updated,
		"skipped_drafts", r.SkippedDrafts,
		"skipped_attachments", r.SkippedAttachments)
	if len(r.MissingPages) > 0 {
		slog.Warn("page overrides without a matching page", "slugs", r.MissingPages)
	}
	for _, warning := range r.Warnings {
		slog.Warn(warning)
	}
}

func fatal(message string, err error) {
	slog.Error(message, "err", err)
	os.Exit(1)
}
