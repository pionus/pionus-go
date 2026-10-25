// Package typechoredirects redirects common legacy Typecho URLs to Pionus
// routes. It is an optional compile-time plugin: enable it with
// plugins.enabled: [typecho-redirects].
package typechoredirects

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pionus/arry"
	"github.com/pionus/pionus-go/internal/model"
	"github.com/pionus/pionus-go/internal/plugin"
	"github.com/pionus/pionus-go/internal/store"
)

// Plugin holds the in-memory legacy URL map.
type Plugin struct {
	routes map[string]string
	log    *slog.Logger
}

// New constructs the plugin.
func New() plugin.Plugin { return &Plugin{} }

// Manifest implements plugin.Plugin.
func (p *Plugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:          "typecho-redirects",
		Name:        "Typecho URL redirects",
		Version:     "1.0.1",
		APIVersion:  plugin.APIVersion,
		Description: "301 redirects common Typecho permalinks, feeds and uploads.",
	}
}

// Init builds the redirect map from imported content and registers middleware.
func (p *Plugin) Init(_ context.Context, env *plugin.Env) error {
	if env.Store == nil {
		return errors.New("typecho-redirects: store is required")
	}
	routes, err := buildRoutes(env.Store)
	if err != nil {
		return err
	}
	p.routes = routes
	p.log = env.Log
	if env.App != nil {
		env.App.Use(p.middleware())
	}
	if p.log != nil {
		p.log.Info("legacy redirects ready", "routes", len(routes))
	}
	return nil
}

func buildRoutes(db store.Store) (map[string]string, error) {
	routes := make(map[string]string)
	add := func(source, target string) {
		source = normalizeSource(source)
		target = normalizeTarget(target)
		if source == "" || target == "" || source == target {
			return
		}
		routes[source] = target
	}

	page := 1
	for {
		posts, total, err := db.ListPosts(store.PostFilter{Page: page, Limit: 200})
		if err != nil {
			return nil, fmt.Errorf("list posts for redirects: %w", err)
		}
		for _, post := range posts {
			if post.Source != "typecho" || post.SourceID == "" || !post.PubliclyVisible(time.Now()) {
				continue
			}
			target := "/article/" + post.Slug
			cid := post.SourceID
			add("/archives/"+cid, target)
			add("/archives/"+cid+"/", target)
			add("/index.php/archives/"+cid, target)
			add("/index.php/archives/"+cid+"/", target)
			add("/?p="+cid, target)
			add("/index.php?p="+cid, target)
			add("/index.php/?p="+cid, target)

			// Typecho pages commonly use slug.html or index.php/slug.html.
			if post.Type == model.PostTypePage && post.Slug != "" {
				add("/"+post.Slug+".html", target)
				add("/index.php/"+post.Slug+".html", target)
				add("/"+post.Slug, target)
				add("/index.php/"+post.Slug, target)
			}

			// Old date archives: /archives/2020/01 and /archives/2020/01/02.
			if !post.CreatedAt.IsZero() {
				ym := post.CreatedAt.Format("2006/01")
				ymd := post.CreatedAt.Format("2006/01/02")
				for _, source := range []string{"/archives/" + ym, "/archives/" + ym + "/", "/index.php/archives/" + ym, "/index.php/archives/" + ym + "/"} {
					add(source, "/archives/"+ym)
				}
				for _, source := range []string{"/archives/" + ymd, "/archives/" + ymd + "/", "/index.php/archives/" + ymd, "/index.php/archives/" + ymd + "/"} {
					add(source, "/archives/"+ymd)
				}
			}
		}
		if page*200 >= total || len(posts) == 0 {
			break
		}
		page++
	}

	categories, err := db.ListCategories()
	if err != nil {
		return nil, fmt.Errorf("list categories for redirects: %w", err)
	}
	for _, category := range categories {
		target := "/category/" + category.Slug
		for _, source := range []string{
			"/category/" + category.Slug,
			"/category/" + category.Slug + "/",
			"/index.php/category/" + category.Slug,
			"/index.php/category/" + category.Slug + "/",
		} {
			add(source, target)
		}
	}

	tags, err := db.ListTags()
	if err != nil {
		return nil, fmt.Errorf("list tags for redirects: %w", err)
	}
	for _, tag := range tags {
		target := "/tag/" + tag.Slug
		for _, source := range []string{
			"/tag/" + tag.Slug,
			"/tag/" + tag.Slug + "/",
			"/index.php/tag/" + tag.Slug,
			"/index.php/tag/" + tag.Slug + "/",
		} {
			add(source, target)
		}
	}

	add("/index.php", "/")
	add("/index.php/", "/")
	for _, source := range []string{
		"/feed/", "/feed/atom", "/feed/atom/", "/feed/rss", "/feed/rss/",
		"/index.php/feed", "/index.php/feed/", "/index.php/feed/atom", "/index.php/feed/atom/",
	} {
		add(source, "/feed")
	}
	return routes, nil
}

func normalizeSource(source string) string {
	source = strings.TrimSpace(source)
	if source == "" {
		return ""
	}
	if !strings.HasPrefix(source, "/") {
		source = "/" + source
	}
	return source
}

func normalizeTarget(target string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return ""
	}
	if !strings.HasPrefix(target, "/") && !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
		target = "/" + target
	}
	return target
}

func (p *Plugin) middleware() arry.Middleware {
	return func(next arry.Handler) arry.Handler {
		return func(ctx arry.Context) {
			if ctx.Request().Method != http.MethodGet && ctx.Request().Method != http.MethodHead {
				next(ctx)
				return
			}
			if target, ok := p.target(ctx.Request()); ok {
				ctx.Redirect(http.StatusMovedPermanently, target)
				return
			}
			next(ctx)
		}
	}
}

func (p *Plugin) target(r *http.Request) (string, bool) {
	path := r.URL.Path
	if path == "" {
		path = "/"
	}

	// Common Typecho asset path.
	for _, prefix := range []string{"/usr/uploads/", "/index.php/usr/uploads/"} {
		if strings.HasPrefix(path, prefix) {
			return "/uploads/" + strings.TrimPrefix(path, prefix), true
		}
	}

	// Typecho search and feed variants that are not finite maps.
	for _, prefix := range []string{"/search/", "/index.php/search/"} {
		if strings.HasPrefix(path, prefix) {
			keywords := strings.Trim(strings.TrimPrefix(path, prefix), "/")
			if keywords != "" {
				return "/search?q=" + url.QueryEscape(keywords), true
			}
		}
	}
	switch path {
	case "/feed/atom", "/feed/atom/", "/feed/rss", "/feed/rss/",
		"/index.php/feed", "/index.php/feed/", "/index.php/feed/atom", "/index.php/feed/atom/":
		return "/feed", true
	case "/index.php", "/index.php/":
		query := r.URL.Query()
		if query.Get("p") == "" && query.Get("s") == "" && query.Get("feed") == "" {
			return "/", true
		}
	}
	if query := r.URL.Query(); query.Get("s") != "" {
		if path == "/" || path == "/index.php" || path == "/index.php/" {
			return "/search?q=" + url.QueryEscape(query.Get("s")), true
		}
	}
	if query := r.URL.Query(); query.Get("feed") != "" {
		if path == "/" || path == "/index.php" || path == "/index.php/" {
			return "/feed", true
		}
	}

	// Fast paths for the new site; avoid map lookups for normal traffic.
	if path == "/" && r.URL.RawQuery == "" {
		return "", false
	}
	if path == "/article" || strings.HasPrefix(path, "/article/") ||
		path == "/assets" || strings.HasPrefix(path, "/assets/") ||
		path == "/uploads" || strings.HasPrefix(path, "/uploads/") ||
		path == "/admin" || strings.HasPrefix(path, "/admin/") ||
		path == "/api" || strings.HasPrefix(path, "/api/") ||
		path == "/notes" || strings.HasPrefix(path, "/notes/") ||
		path == "/categories" || path == "/tags" ||
		path == "/feed" || path == "/search" ||
		path == "/health" || path == "/robots.txt" || path == "/sitemap.xml" {
		return "", false
	}
	if strings.HasPrefix(path, "/tag/") || strings.HasPrefix(path, "/category/") {
		if strings.HasSuffix(path, "/") {
			return strings.TrimSuffix(path, "/"), true
		}
		return "", false
	}

	if p.routes == nil {
		return "", false
	}
	candidates := make([]string, 0, 2)
	if r.URL.RawQuery != "" {
		candidates = append(candidates, path+"?"+r.URL.RawQuery)
	}
	candidates = append(candidates, path)
	for _, candidate := range candidates {
		if target, ok := p.routes[candidate]; ok && target != path {
			return target, true
		}
	}
	return "", false
}
