package migrate

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/pionus/pionus-go/internal/model"
	"github.com/pionus/pionus-go/internal/store"
)

var titleRe = regexp.MustCompile(`(?m)^#\s+(.+)`)

// MarkdownToSQLite imports legacy markdown files into the store. It is
// idempotent: existing slugs are skipped.
func MarkdownToSQLite(mdDir string, db store.Store) error {
	entries, err := os.ReadDir(mdDir)
	if err != nil {
		return fmt.Errorf("read markdown dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".md")
		data, err := os.ReadFile(filepath.Join(mdDir, entry.Name()))
		if err != nil {
			slog.Warn("skip file", "name", entry.Name(), "err", err)
			continue
		}

		title := parseTitle(data)
		if title == "" {
			title = name
		}

		created := parseCreatedTime(name)
		post := &model.Post{
			Slug:        name,
			Title:       title,
			Content:     string(data),
			Excerpt:     model.MakeExcerpt(string(data), 200),
			Type:        model.PostTypePost,
			Status:      model.StatusPublish,
			CreatedAt:   created,
			PublishedAt: &created,
		}

		if err := db.CreatePost(post); err != nil {
			if errors.Is(err, store.ErrConflict) {
				slog.Info("skip existing", "slug", name)
				continue
			}
			slog.Warn("skip article", "slug", name, "err", err)
			continue
		}
		slog.Info("migrated", "slug", name, "title", title)
	}
	return nil
}

func parseTitle(data []byte) string {
	m := titleRe.FindSubmatch(data)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(string(m[1]))
}

func parseCreatedTime(name string) time.Time {
	for _, layout := range []string{"20060102150405", "20060102"} {
		if t, err := time.Parse(layout, name); err == nil {
			return t
		}
	}
	return time.Now()
}
