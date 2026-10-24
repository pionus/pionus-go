package handler

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/pionus/arry"
	"github.com/pionus/pionus-go/internal/model"
	"github.com/pionus/pionus-go/internal/store"
)

// siteInfo is the subset of options the public theme needs.
type siteInfo struct {
	Title       string
	Description string
	Author      string
	URL         string
	Keywords    string
	Locale      string
}

func loadSite(db store.Store) siteInfo {
	get := func(name, def string) string {
		v, err := db.GetOption(name)
		if err != nil || v == "" {
			return def
		}
		return v
	}
	return siteInfo{
		Title:       get("site.title", "Pionus"),
		Description: get("site.description", ""),
		Author:      get("site.author", ""),
		URL:         strings.TrimRight(get("site.url", ""), "/"),
		Keywords:    get("site.keywords", ""),
		Locale:      get("site.locale", "zh-CN"),
	}
}

// baseData builds the data every public template needs.
func baseData(ctx arry.Context, db store.Store, title string) map[string]any {
	site := loadSite(db)
	if title == "" {
		title = site.Title
	}

	pages, _, _ := db.ListPosts(store.PostFilter{
		Type:     model.PostTypePage,
		Statuses: []string{model.StatusPublish},
		Page:     1,
		Limit:    10,
	})
	navPages := make([]*model.Post, 0, len(pages))
	for _, p := range pages {
		// 微语 is now a dedicated timeline at /notes.
		if p.Slug == "t" || p.Template == "t.php" {
			continue
		}
		navPages = append(navPages, p)
	}

	return map[string]any{
		"Site":         site,
		"Title":        title,
		"SiteURL":      site.URL,
		"CurrentPath":  ctx.Request().URL.Path,
		"Year":         time.Now().Year(),
		"SearchQuery":  ctx.Query("q"),
		"NavPages":     navPages,
		"CanonicalURL": canonicalURL(ctx, site),
	}
}

func canonicalURL(ctx arry.Context, site siteInfo) string {
	base := site.URL
	if base == "" {
		scheme := "http"
		if ctx.Request().TLS != nil || strings.EqualFold(ctx.Header("X-Forwarded-Proto"), "https") {
			scheme = "https"
		}
		base = scheme + "://" + ctx.Request().Host
	}
	return base + ctx.Request().URL.RequestURI()
}

// pageURL builds a pagination link, keeping extra query parameters.
func pageURL(base string, page int, extra url.Values) string {
	q := url.Values{}
	for k, v := range extra {
		q[k] = v
	}
	if page > 1 {
		q.Set("page", strconv.Itoa(page))
	}
	if encoded := q.Encode(); encoded != "" {
		return base + "?" + encoded
	}
	return base
}

// listData runs a public post query and builds archive data.
func listData(ctx arry.Context, db store.Store, f store.PostFilter, title, base string, extra url.Values, groupByYear bool) (map[string]any, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.Limit < 1 {
		f.Limit = 20
	}
	now := time.Now()
	f.PublishedBefore = &now
	if len(f.Statuses) == 0 {
		f.Statuses = []string{model.StatusPublish}
	}
	if f.Type == "" {
		f.Type = model.PostTypePost
	}
	f.WithTaxonomy = true

	posts, total, err := db.ListPosts(f)
	if err != nil {
		return nil, err
	}

	totalPages := (total + f.Limit - 1) / f.Limit
	if totalPages < 1 {
		totalPages = 1
	}

	data := baseData(ctx, db, title)
	data["Posts"] = posts
	data["Total"] = total
	data["Page"] = f.Page
	data["TotalPages"] = totalPages
	data["HasPrev"] = f.Page > 1
	data["HasNext"] = f.Page < totalPages
	data["PrevURL"] = pageURL(base, f.Page-1, extra)
	data["NextURL"] = pageURL(base, f.Page+1, extra)
	data["BaseURL"] = base
	data["ExtraQuery"] = extra

	if groupByYear {
		groups := make([]YearGroup, 0)
		for _, p := range posts {
			year := p.DisplayTime().Year()
			if len(groups) == 0 || groups[len(groups)-1].Year != year {
				groups = append(groups, YearGroup{Year: year})
			}
			groups[len(groups)-1].Posts = append(groups[len(groups)-1].Posts, p)
		}
		data["Groups"] = groups
	}

	return data, nil
}

func parsePage(ctx arry.Context) int {
	page, _ := strconv.Atoi(ctx.Query("page"))
	if page < 1 {
		page = 1
	}
	return page
}

func perPage(db store.Store, def int) int {
	raw, err := db.GetOption("posts.per_page")
	if err != nil {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > 100 {
		return def
	}
	return n
}

func notFound(ctx arry.Context, db store.Store, message string) {
	data := baseData(ctx, db, "页面没找到")
	data["Message"] = message
	ctx.Render(http.StatusNotFound, "404.html", data)
}
