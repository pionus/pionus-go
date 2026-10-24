package typecho

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pionus/pionus-go/internal/model"
	"github.com/pionus/pionus-go/internal/store"
)

// PageOverrideReport summarizes custom page content applied during migration.
type PageOverrideReport struct {
	Applied []string `json:"applied,omitempty"`
	Missing []string `json:"missing,omitempty"`
	DryRun  bool     `json:"dry_run,omitempty"`
}

// LoadPageOverrides reads <dir>/<slug>.md files into a slug -> content map.
func LoadPageOverrides(dir string) (map[string]string, error) {
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dir)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	overrides := make(map[string]string)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		slug := strings.TrimSuffix(entry.Name(), ".md")
		overrides[slug] = string(data)
	}
	return overrides, nil
}

// ApplyPageOverrides replaces the body of imported pages by slug. It is used
// for content that lived in old PHP theme templates (friend links, microblog
// shells) rather than in typecho_contents.
func ApplyPageOverrides(db store.Store, overrides map[string]string, dryRun bool) (*PageOverrideReport, error) {
	report := &PageOverrideReport{DryRun: dryRun}
	if len(overrides) == 0 {
		return report, nil
	}

	slugs := make([]string, 0, len(overrides))
	for slug := range overrides {
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)

	for _, slug := range slugs {
		content := overrides[slug]
		p, err := db.GetPost(slug)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				report.Missing = append(report.Missing, slug)
				continue
			}
			return nil, fmt.Errorf("load page %q: %w", slug, err)
		}
		if p.Type != model.PostTypePage {
			report.Missing = append(report.Missing, slug)
			continue
		}
		if !dryRun {
			if err := db.UpdatePostContent(p.ID, content, model.MakeExcerpt(content, 200)); err != nil {
				return nil, fmt.Errorf("override page %q: %w", slug, err)
			}
		}
		report.Applied = append(report.Applied, slug)
	}
	return report, nil
}
