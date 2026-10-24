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

func siteBaseURL(ctx arry.Context, db store.Store) string {
	if site := loadSite(db); site.URL != "" {
		return site.URL
	}
	scheme := "http"
	if ctx.Request().TLS != nil || strings.EqualFold(ctx.Header("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return scheme + "://" + ctx.Request().Host
}

func Feed(db store.Store) arry.Handler {
	return func(ctx arry.Context) {
		base := siteBaseURL(ctx, db)
		site := loadSite(db)
		now := time.Now()

		posts, _, err := db.ListPosts(store.PostFilter{
			Type:            model.PostTypePost,
			Statuses:        []string{model.StatusPublish},
			PublishedBefore: &now,
			Page:            1,
			Limit:           20,
			WithTaxonomy:    true,
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

		writeXML(ctx, doc, "application/rss+xml; charset=utf-8")
	}
}

func CommentsFeed(db store.Store) arry.Handler {
	return func(ctx arry.Context) {
		base := siteBaseURL(ctx, db)
		site := loadSite(db)

		comments, _, err := db.ListComments(store.CommentFilter{
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
			if c.PostSlug == "" || c.PostLocked {
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

		writeXML(ctx, doc, "application/rss+xml; charset=utf-8")
	}
}

func Sitemap(db store.Store) arry.Handler {
	return func(ctx arry.Context) {
		base := siteBaseURL(ctx, db)
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
			posts, total, err := db.ListPosts(store.PostFilter{
				Statuses:        []string{model.StatusPublish},
				PublishedBefore: &now,
				Page:            page,
				Limit:           200,
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

		categories, _ := db.ListCategories()
		for _, c := range categories {
			add(base+"/category/"+url.PathEscape(c.Slug), now.Format("2006-01-02"))
		}
		tags, _ := db.ListTags()
		for _, t := range tags {
			add(base+"/tag/"+url.PathEscape(t.Slug), now.Format("2006-01-02"))
		}

		writeXML(ctx, doc, "application/xml; charset=utf-8")
	}
}

func Robots(db store.Store) arry.Handler {
	return func(ctx arry.Context) {
		base := siteBaseURL(ctx, db)
		body := "User-agent: *\nAllow: /\nDisallow: /admin/\nDisallow: /search\nSitemap: " + base + "/sitemap.xml\n"
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
