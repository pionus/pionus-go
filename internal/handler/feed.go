package handler

import (
	"encoding/xml"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/pionus/arry"
	"github.com/pionus/pionus-go/internal/model"
	"github.com/pionus/pionus-go/internal/store"
)

func siteBaseURL(ctx arry.Context, deps Deps) string {
	if site := loadSite(deps); site.URL != "" {
		return site.URL
	}
	scheme := "http"
	if ctx.Request().TLS != nil || strings.EqualFold(ctx.Header("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return scheme + "://" + ctx.Request().Host
}

func Feed(deps Deps) arry.Handler {
	return func(ctx arry.Context) {
		base := siteBaseURL(ctx, deps)
		site := loadSite(deps)
		now := time.Now()

		posts, _, err := deps.Posts.ListPosts(store.PostFilter{
			Type:                model.PostTypePost,
			Statuses:            []string{model.StatusPublish},
			PublishedBefore:     &now,
			Page:                1,
			Limit:               20,
			WithTaxonomy:        true,
			ExcludeHiddenTagged: true,
		})
		if err != nil {
			slog.Error("feed posts", "err", err)
			ctx.Reply(http.StatusInternalServerError)
			return
		}

		doc := rssDocument{
			Version: "2.0",
			Channel: rssChannel{
				Title:         site.Title,
				Link:          base,
				Description:   site.Description,
				Language:      site.Locale,
				LastBuildDate: now.Format(time.RFC1123Z),
				Generator:     "Pionus",
			},
		}
		for _, p := range posts {
			if p.PostPassword != "" {
				continue
			}
			link := base + "/article/" + p.Slug
			item := rssItem{
				Title:       p.Title,
				Link:        link,
				GUID:        link,
				PubDate:     p.DisplayTime().Format(time.RFC1123Z),
				Description: p.Excerpt,
			}
			for _, c := range p.Categories {
				item.Categories = append(item.Categories, c.Name)
			}
			doc.Channel.Items = append(doc.Channel.Items, item)
		}

		ctx.SetHeader("Cache-Control", "public, max-age=300")
		writeXML(ctx, doc, "application/rss+xml; charset=utf-8")
	}
}

func NotesFeed(deps Deps) arry.Handler {
	return func(ctx arry.Context) {
		base := siteBaseURL(ctx, deps)
		site := loadSite(deps)

		notes, _, err := deps.Notes.ListNotes(store.NoteFilter{
			Statuses: []string{model.StatusPublish},
			Page:     1,
			Limit:    30,
		})
		if err != nil {
			slog.Error("notes feed", "err", err)
			ctx.Reply(http.StatusInternalServerError)
			return
		}

		doc := rssDocument{
			Version: "2.0",
			Channel: rssChannel{
				Title:         site.Title + " - 微语",
				Link:          base + "/notes",
				Description:   site.Description,
				Language:      site.Locale,
				LastBuildDate: time.Now().Format(time.RFC1123Z),
				Generator:     "Pionus",
			},
		}
		for _, n := range notes {
			link := base + "/notes/" + strconv.FormatInt(n.ID, 10)
			doc.Channel.Items = append(doc.Channel.Items, rssItem{
				Title:       "微语",
				Link:        link,
				GUID:        link,
				PubDate:     n.CreatedAt.Format(time.RFC1123Z),
				Description: n.Content,
			})
		}

		ctx.SetHeader("Cache-Control", "public, max-age=300")
		writeXML(ctx, doc, "application/rss+xml; charset=utf-8")
	}
}

func CommentsFeed(deps Deps) arry.Handler {
	return func(ctx arry.Context) {
		base := siteBaseURL(ctx, deps)
		site := loadSite(deps)

		comments, _, err := deps.Comments.ListComments(store.CommentFilter{
			Status: model.CommentApproved,
			Page:   1,
			Limit:  50,
		})
		if err != nil {
			slog.Error("comments feed", "err", err)
			ctx.Reply(http.StatusInternalServerError)
			return
		}

		doc := rssDocument{
			Version: "2.0",
			Channel: rssChannel{
				Title:         site.Title + " - 评论",
				Link:          base,
				Description:   site.Description,
				Language:      site.Locale,
				LastBuildDate: time.Now().Format(time.RFC1123Z),
				Generator:     "Pionus",
			},
		}
		for _, c := range comments {
			if c.PostSlug == "" || c.PostLocked || c.PostHidden {
				continue
			}
			link := base + "/article/" + c.PostSlug + "#comment-" + strconv.FormatInt(c.ID, 10)
			doc.Channel.Items = append(doc.Channel.Items, rssItem{
				Title:       c.Author + " 评论了《" + c.PostTitle + "》",
				Link:        link,
				GUID:        link,
				PubDate:     c.CreatedAt.Format(time.RFC1123Z),
				Description: model.PlainText(c.Content),
			})
		}

		ctx.SetHeader("Cache-Control", "public, max-age=300")
		writeXML(ctx, doc, "application/rss+xml; charset=utf-8")
	}
}

func Sitemap(deps Deps) arry.Handler {
	return func(ctx arry.Context) {
		base := siteBaseURL(ctx, deps)
		now := time.Now()

		doc := urlSet{XMLNS: "http://www.sitemaps.org/schemas/sitemap/0.9"}
		add := func(loc, lastmod string) {
			doc.URLs = append(doc.URLs, sitemapURL{Loc: loc, LastMod: lastmod})
		}

		add(base+"/", now.Format("2006-01-02"))
		add(base+"/article", now.Format("2006-01-02"))
		add(base+"/notes", now.Format("2006-01-02"))
		add(base+"/categories", now.Format("2006-01-02"))
		add(base+"/tags", now.Format("2006-01-02"))

		page := 1
		for {
			posts, total, err := deps.Posts.ListPosts(store.PostFilter{
				Statuses:            []string{model.StatusPublish},
				PublishedBefore:     &now,
				Page:                page,
				Limit:               200,
				ExcludeHiddenTagged: true,
			})
			if err != nil {
				slog.Error("sitemap posts", "err", err)
				break
			}
			for _, p := range posts {
				if p.PostPassword != "" {
					continue
				}
				loc := base + "/article/" + p.Slug
				if p.Type == model.PostTypePage {
					loc = base + "/article/" + p.Slug
				}
				add(loc, p.UpdatedAt.Format("2006-01-02"))
			}
			if page*200 >= total || len(posts) == 0 {
				break
			}
			page++
		}

		categories, _ := deps.Taxonomy.ListCategories()
		for _, c := range categories {
			add(base+"/category/"+url.PathEscape(c.Slug), now.Format("2006-01-02"))
		}
		tags, _ := deps.Taxonomy.ListTags()
		for _, t := range tags {
			if t.Hidden {
				continue
			}
			add(base+"/tag/"+url.PathEscape(t.Slug), now.Format("2006-01-02"))
		}

		ctx.SetHeader("Cache-Control", "public, max-age=300")
		writeXML(ctx, doc, "application/xml; charset=utf-8")
	}
}

func Robots(deps Deps) arry.Handler {
	return func(ctx arry.Context) {
		base := siteBaseURL(ctx, deps)
		body := "User-agent: *\nAllow: /\nDisallow: /admin/\nDisallow: /search\nSitemap: " + base + "/sitemap.xml\n"
		ctx.SetHeader("Cache-Control", "public, max-age=3600")
		ctx.SetContentType("text/plain; charset=utf-8")
		ctx.Status(http.StatusOK)
		ctx.Response().Write([]byte(body))
	}
}

func writeXML(ctx arry.Context, doc any, contentType string) {
	data, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		slog.Error("marshal xml", "err", err)
		ctx.Reply(http.StatusInternalServerError)
		return
	}
	body := append([]byte(xml.Header), data...)
	ctx.SetContentType(contentType)
	ctx.Status(http.StatusOK)
	ctx.Response().Write(body)
}

type rssDocument struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title         string    `xml:"title"`
	Link          string    `xml:"link"`
	Description   string    `xml:"description"`
	Language      string    `xml:"language,omitempty"`
	LastBuildDate string    `xml:"lastBuildDate"`
	Generator     string    `xml:"generator"`
	Items         []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string   `xml:"title"`
	Link        string   `xml:"link"`
	GUID        string   `xml:"guid"`
	PubDate     string   `xml:"pubDate"`
	Description string   `xml:"description"`
	Categories  []string `xml:"category,omitempty"`
}

type urlSet struct {
	XMLName xml.Name     `xml:"urlset"`
	XMLNS   string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

type sitemapURL struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}
