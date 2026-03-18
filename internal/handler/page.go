package handler

import (
	"bytes"
	"database/sql"
	"errors"
	"html/template"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/pionus/arry"
	"github.com/pionus/pionus-go/internal/model"
	"github.com/pionus/pionus-go/internal/store"
	"github.com/yuin/goldmark"
)

// YearGroup holds articles grouped by year for the article list page.
type YearGroup struct {
	Year     int
	Articles []*model.Article
}

func Index(db store.Store) arry.Handler {
	return func(ctx arry.Context) {
		articles, _, err := db.ListArticles(1, 5)
		if err != nil {
			slog.Error("index page", "err", err)
		}
		ctx.Render(200, "cover.html", map[string]interface{}{
			"Articles":    articles,
			"CurrentPath": "/",
		})
	}
}

func ArticleList(db store.Store) arry.Handler {
	return func(ctx arry.Context) {
		pageStr := ctx.Query("page")
		page := 1
		if pageStr != "" {
			if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
				page = p
			}
		}
		limit := 50

		articles, total, err := db.ListArticles(page, limit)
		if err != nil {
			slog.Error("article list page", "err", err)
			ctx.Reply(500)
			return
		}

		totalPages := int(math.Ceil(float64(total) / float64(limit)))

		// Group articles by year
		var groups []YearGroup
		for _, a := range articles {
			year := a.CreatedAt.Year()
			if len(groups) == 0 || groups[len(groups)-1].Year != year {
				groups = append(groups, YearGroup{Year: year})
			}
			groups[len(groups)-1].Articles = append(groups[len(groups)-1].Articles, a)
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
	md := goldmark.New()

	return func(ctx arry.Context) {
		slug := ctx.Param("slug")
		article, err := db.GetArticle(slug)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) || strings.Contains(err.Error(), "not found") {
				ctx.Reply(404)
				return
			}
			slog.Error("article page", "slug", slug, "err", err)
			ctx.Reply(500)
			return
		}

		// Render markdown to HTML
		var buf bytes.Buffer
		if err := md.Convert([]byte(article.Content), &buf); err != nil {
			slog.Error("markdown render", "slug", slug, "err", err)
		}

		// Calculate reading time (~200 Chinese chars or words per minute)
		wordCount := utf8.RuneCountInString(article.Content)
		readingTime := int(math.Ceil(float64(wordCount) / 200.0))
		if readingTime < 1 {
			readingTime = 1
		}

		// Get adjacent articles
		prev, next, _ := db.GetAdjacentArticles(slug)

		ctx.Render(200, "article.html", map[string]interface{}{
			"Article":         article,
			"RenderedContent": template.HTML(buf.String()),
			"ReadingTime":     readingTime,
			"Prev":            prev,
			"Next":            next,
			"CurrentPath":     "/article",
		})
	}
}
