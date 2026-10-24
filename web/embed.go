// Package web embeds the public theme and serves its assets.
package web

import (
	"embed"
	"html/template"
	"io/fs"
	"net/url"
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

// EngineConfig configures the public HTML view engine.
func EngineConfig() arry.EngineConfig {
	return arry.EngineConfig{
		Type:      arry.EngineHTML,
		Dir:       "pages",
		FS:        ThemeFS(),
		Extension: "html",
		Cache:     true,
		FuncMap:   FuncMap(),
	}
}

// NewEngine creates the public view engine.
func NewEngine() arry.Engine {
	return arry.NewEngineWithConfig(EngineConfig())
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
