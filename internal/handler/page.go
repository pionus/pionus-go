package handler

import (
	"bytes"
	"errors"
	"html/template"
	"log/slog"
	"math"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/pionus/arry"
	"github.com/pionus/pionus-go/internal/model"
	"github.com/pionus/pionus-go/internal/store"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

// YearGroup holds posts grouped by year for the archive page.
type YearGroup struct {
	Year  int
	Posts []*model.Post
}

// Articles returns the posts in the group. It is kept as an alias so older
// templates keep working.
func (g YearGroup) Articles() []*model.Post {
	return g.Posts
}

var markdown = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithRendererOptions(html.WithUnsafe()),
)

func Index(db store.Store) arry.Handler {
	return func(ctx arry.Context) {
		now := time.Now()
		posts, _, err := db.ListPosts(store.PostFilter{
			Type:            model.PostTypePost,
			Statuses:        []string{model.StatusPublish},
			PublishedBefore: &now,
			Page:            1,
			Limit:           5,
		})
		if err != nil {
			slog.Error("index page", "err", err)
		}

		ctx.Render(200, "cover.html", map[string]interface{}{
			"Articles":    posts,
			"CurrentPath": "/",
		})
	}
}

func ArticleList(db store.Store) arry.Handler {
	return func(ctx arry.Context) {
		page := 1
		if raw := ctx.Query("page"); raw != "" {
			if p, err := strconv.Atoi(raw); err == nil && p > 0 {
				page = p
			}
		}
		const limit = 50
		now := time.Now()

		posts, total, err := db.ListPosts(store.PostFilter{
			Type:            model.PostTypePost,
			Statuses:        []string{model.StatusPublish},
			PublishedBefore: &now,
			Page:            page,
			Limit:           limit,
		})
		if err != nil {
			slog.Error("article list page", "err", err)
			ctx.Reply(500)
			return
		}

		totalPages := int(math.Ceil(float64(total) / float64(limit)))
		if totalPages < 1 {
			totalPages = 1
		}

		groups := make([]YearGroup, 0)
		for _, p := range posts {
			year := p.DisplayTime().Year()
			if len(groups) == 0 || groups[len(groups)-1].Year != year {
				groups = append(groups, YearGroup{Year: year})
			}
			groups[len(groups)-1].Posts = append(groups[len(groups)-1].Posts, p)
		}

		ctx.Render(200, "index.html", map[string]interface{}{
			"Groups":      groups,
			"Page":        page,
			"TotalPages":  totalPages,
			"HasPrev":     page > 1,
			"HasNext":     page < totalPages,
			"PrevPage":    page - 1,
			"NextPage":    page + 1,
			"CurrentPath": "/article",
		})
	}
}

func ArticleDetail(db store.Store) arry.Handler {
	return func(ctx arry.Context) {
		slug := ctx.Param("slug")

		post, err := db.GetPost(slug)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				ctx.Reply(404)
				return
			}
			slog.Error("article page", "slug", slug, "err", err)
			ctx.Reply(500)
			return
		}

		// Drafts and scheduled posts are only visible to authenticated users.
		if !post.IsPublished() && !ctx.IsAuthed() {
			ctx.Reply(404)
			return
		}
		if post.PublishedAt != nil && post.PublishedAt.After(time.Now()) && !ctx.IsAuthed() {
			ctx.Reply(404)
			return
		}

		var buf bytes.Buffer
		if err := markdown.Convert([]byte(post.Content), &buf); err != nil {
			slog.Error("markdown render", "slug", slug, "err", err)
		}

		plain := model.PlainText(post.Content)
		readingTime := int(math.Ceil(float64(utf8.RuneCountInString(plain)) / 200.0))
		if readingTime < 1 {
			readingTime = 1
		}

		prev, next, err := db.AdjacentPosts(slug)
		if err != nil {
			slog.Warn("adjacent posts", "slug", slug, "err", err)
		}

		_ = db.IncrementPostViews(slug)

		ctx.Render(200, "article.html", map[string]interface{}{
			"Article":         post,
			"RenderedContent": template.HTML(buf.String()),
			"ReadingTime":     readingTime,
			"Prev":            prev,
			"Next":            next,
			"CurrentPath":     "/article",
		})
	}
}
