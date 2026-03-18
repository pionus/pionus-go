package handler

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pionus/arry"
	"github.com/pionus/pionus-go/internal/model"
	"github.com/pionus/pionus-go/internal/store"
)

var slugRe = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

type errorResponse struct {
	Error string `json:"error"`
}

type listResponse struct {
	Articles []*model.Article `json:"articles"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	Limit    int              `json:"limit"`
}

func jsonError(ctx arry.Context, code int, msg string) {
	ctx.JSON(code, errorResponse{Error: msg})
}

func ListArticles(db store.Store) arry.Handler {
	return func(ctx arry.Context) {
		page, _ := strconv.Atoi(ctx.QueryDefault("page", "1"))
		limit, _ := strconv.Atoi(ctx.QueryDefault("limit", "20"))
		if page < 1 {
			page = 1
		}
		if limit < 1 || limit > 100 {
			limit = 20
		}

		articles, total, err := db.ListArticles(page, limit)
		if err != nil {
			slog.Error("list articles", "err", err)
			jsonError(ctx, http.StatusInternalServerError, "failed to list articles")
			return
		}
		if articles == nil {
			articles = []*model.Article{}
		}

		ctx.JSON(http.StatusOK, listResponse{
			Articles: articles,
			Total:    total,
			Page:     page,
			Limit:    limit,
		})
	}
}

func GetArticle(db store.Store) arry.Handler {
	return func(ctx arry.Context) {
		slug := ctx.Param("slug")

		article, err := db.GetArticle(slug)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) || strings.Contains(err.Error(), "not found") {
				jsonError(ctx, http.StatusNotFound, "article not found")
				return
			}
			slog.Error("get article", "slug", slug, "err", err)
			jsonError(ctx, http.StatusInternalServerError, "failed to get article")
			return
		}

		ctx.JSON(http.StatusOK, article)
	}
}

func CreateArticle(db store.Store) arry.Handler {
	return func(ctx arry.Context) {
		if !isAuthed(ctx) {
			jsonError(ctx, http.StatusForbidden, "forbidden")
			return
		}

		var a model.Article
		if err := ctx.Decode(&a); err != nil {
			jsonError(ctx, http.StatusBadRequest, "invalid JSON body")
			return
		}

		if strings.TrimSpace(a.Title) == "" {
			jsonError(ctx, http.StatusBadRequest, "title is required")
			return
		}
		if strings.TrimSpace(a.Slug) == "" {
			jsonError(ctx, http.StatusBadRequest, "slug is required")
			return
		}
		if !slugRe.MatchString(a.Slug) {
			jsonError(ctx, http.StatusBadRequest, "slug must contain only letters, numbers, hyphens, and underscores")
			return
		}
		if strings.TrimSpace(a.Content) == "" {
			jsonError(ctx, http.StatusBadRequest, "content is required")
			return
		}
		if a.Author == "" {
			a.Author = "Secbone"
		}
		if a.CreatedAt.IsZero() {
			a.CreatedAt = time.Now()
		}

		if err := db.CreateArticle(&a); err != nil {
			if strings.Contains(err.Error(), "UNIQUE") {
				jsonError(ctx, http.StatusConflict, "slug already exists")
				return
			}
			slog.Error("create article", "err", err)
			jsonError(ctx, http.StatusInternalServerError, "failed to create article")
			return
		}

		ctx.JSON(http.StatusCreated, a)
	}
}

func UpdateArticle(db store.Store) arry.Handler {
	return func(ctx arry.Context) {
		if !isAuthed(ctx) {
			jsonError(ctx, http.StatusForbidden, "forbidden")
			return
		}

		slug := ctx.Param("slug")

		var a model.Article
		if err := ctx.Decode(&a); err != nil {
			jsonError(ctx, http.StatusBadRequest, "invalid JSON body")
			return
		}

		if strings.TrimSpace(a.Title) == "" {
			jsonError(ctx, http.StatusBadRequest, "title is required")
			return
		}
		if strings.TrimSpace(a.Content) == "" {
			jsonError(ctx, http.StatusBadRequest, "content is required")
			return
		}

		if err := db.UpdateArticle(slug, &a); err != nil {
			if strings.Contains(err.Error(), "not found") {
				jsonError(ctx, http.StatusNotFound, "article not found")
				return
			}
			slog.Error("update article", "slug", slug, "err", err)
			jsonError(ctx, http.StatusInternalServerError, "failed to update article")
			return
		}

		updated, _ := db.GetArticle(slug)
		ctx.JSON(http.StatusOK, updated)
	}
}

func DeleteArticle(db store.Store) arry.Handler {
	return func(ctx arry.Context) {
		if !isAuthed(ctx) {
			jsonError(ctx, http.StatusForbidden, "forbidden")
			return
		}

		slug := ctx.Param("slug")

		if err := db.DeleteArticle(slug); err != nil {
			if strings.Contains(err.Error(), "not found") {
				jsonError(ctx, http.StatusNotFound, "article not found")
				return
			}
			slog.Error("delete article", "slug", slug, "err", err)
			jsonError(ctx, http.StatusInternalServerError, "failed to delete article")
			return
		}

		ctx.JSON(http.StatusOK, map[string]string{"message": "deleted"})
	}
}

func isAuthed(ctx arry.Context) bool {
	v := ctx.Get("auth")
	if v == nil {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}
