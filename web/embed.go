// Package web embeds the public theme and serves its assets.
package web

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/pionus/arry"
)

//go:embed theme
var embedded embed.FS

// ThemeFS returns the theme root (pages/ and assets/).
func ThemeFS() fs.FS {
	sub, err := fs.Sub(embedded, "theme")
	if err != nil {
		panic("web: theme: " + err.Error())
	}
	return sub
}

// AssetsFS returns theme/assets for StaticFS.
func AssetsFS() fs.FS {
	sub, err := fs.Sub(embedded, "theme/assets")
	if err != nil {
		panic("web: theme assets: " + err.Error())
	}
	return sub
}

// EngineConfig configures the embedded public HTML view engine.
func EngineConfig() arry.EngineConfig {
	return EngineConfigWithFS(ThemeFS())
}

// EngineConfigWithFS configures the public view engine with a template
// filesystem whose root contains pages/.
func EngineConfigWithFS(templateFS fs.FS) arry.EngineConfig {
	return arry.EngineConfig{
		Type:      arry.EngineHTML,
		Dir:       "pages",
		FS:        templateFS,
		Extension: "html",
		Cache:     true,
		FuncMap:   FuncMap(),
	}
}

// ThemeFilesystems resolves the template and asset filesystems. An empty dir
// uses the embedded theme. A non-empty dir is layered over the embedded theme,
// so a custom theme can override only the templates/assets it needs.
func ThemeFilesystems(dir string) (templates fs.FS, assets fs.FS, err error) {
	if dir == "" {
		return ThemeFS(), AssetsFS(), nil
	}

	info, err := os.Stat(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("theme directory: %w", err)
	}
	if !info.IsDir() {
		return nil, nil, fmt.Errorf("theme path %q is not a directory", dir)
	}

	root := os.DirFS(dir)
	templates = overlayFS{primary: root, fallback: ThemeFS()}
	assets = AssetsFS()
	if info, err := fs.Stat(root, "assets"); err == nil && info.IsDir() {
		if sub, err := fs.Sub(root, "assets"); err == nil {
			assets = overlayFS{primary: sub, fallback: AssetsFS()}
		}
	}
	return templates, assets, nil
}

// EngineConfigWithTheme loads a theme directory, falling back to embedded
// templates for files the theme does not provide.
func EngineConfigWithTheme(dir string) (arry.EngineConfig, error) {
	templates, _, err := ThemeFilesystems(dir)
	if err != nil {
		return arry.EngineConfig{}, err
	}
	return EngineConfigWithFS(templates), nil
}

// NewEngine creates the embedded public view engine.
func NewEngine() arry.Engine {
	return arry.NewEngineWithConfig(EngineConfig())
}

// NewEngineWithTheme creates a view engine for a theme directory.
func NewEngineWithTheme(dir string) (arry.Engine, error) {
	cfg, err := EngineConfigWithTheme(dir)
	if err != nil {
		return nil, err
	}
	return arry.NewEngineWithConfig(cfg), nil
}

// FuncMap contains helpers used by the public templates.
func FuncMap() template.FuncMap {
	return template.FuncMap{
		"date": func(t time.Time) string {
			if t.IsZero() {
				return ""
			}
			return t.Format("2006-01-02")
		},
		"dateTime": func(t time.Time) string {
			if t.IsZero() {
				return ""
			}
			return t.Format("2006-01-02 15:04")
		},
		"year":     func(t time.Time) int { return t.Year() },
		"contains": strings.Contains,
		"urlquery": url.QueryEscape,
		"add":      func(a, b int) int { return a + b },
		"sub":      func(a, b int) int { return a - b },
	}
}
