package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/pionus/arry"
	"github.com/pionus/pionus-go/internal/content"
	"github.com/pionus/pionus-go/internal/model"
	"github.com/pionus/pionus-go/internal/store"
)

type errorResponse struct {
	Error string `json:"error"`
}

type listResponse struct {
	Posts []*model.Post `json:"posts"`
	Total int           `json:"total"`
	Page  int           `json:"page"`
	Limit int           `json:"limit"`
}

func jsonError(ctx arry.Context, code int, msg string) {
	ctx.JSON(code, errorResponse{Error: msg})
}

func pageParams(ctx arry.Context, defaultLimit int) (page, limit int) {
	page, _ = strconv.Atoi(ctx.QueryDefault("page", "1"))
	limit, _ = strconv.Atoi(ctx.QueryDefault("limit", strconv.Itoa(defaultLimit)))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = defaultLimit
	}
	return page, limit
}

func ListPosts(db store.Store) arry.Handler {
	return func(ctx arry.Context) {
		page, limit := pageParams(ctx, 20)
		now := time.Now()

		f := store.PostFilter{
			Type:            model.PostTypePost,
			Statuses:        []string{model.StatusPublish},
			PublishedBefore: &now,
			Page:            page,
			Limit:           limit,
			Query:           strings.TrimSpace(ctx.Query("q")),
			CategorySlug:    ctx.Query("category"),
			TagSlug:         ctx.Query("tag"),
		}

		posts, total, err := db.ListPosts(f)
		if err != nil {
			slog.Error("list posts", "err", err)
			jsonError(ctx, http.StatusInternalServerError, "failed to list posts")
			return
		}

		ctx.JSON(http.StatusOK, listResponse{
			Posts: posts,
			Total: total,
			Page:  page,
			Limit: limit,
		})
	}
}

func GetPost(db store.Store, guard *PostGuard) arry.Handler {
	return func(ctx arry.Context) {
		slug := ctx.Param("slug")

		p, err := db.GetPost(slug)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				jsonError(ctx, http.StatusNotFound, "post not found")
				return
			}
			slog.Error("get post", "slug", slug, "err", err)
			jsonError(ctx, http.StatusInternalServerError, "failed to get post")
			return
		}

		if !p.PubliclyVisible(time.Now()) && !ctx.IsAuthed() {
			jsonError(ctx, http.StatusNotFound, "post not found")
			return
		}
		if p.PostPassword != "" && !ctx.IsAuthed() && !guard.Allowed(ctx, p) {
			jsonError(ctx, http.StatusForbidden, "post password required")
			return
		}

		ctx.JSON(http.StatusOK, p)
	}
}

func CreatePost(db store.Store) arry.Handler {
	return func(ctx arry.Context) {
		if !ctx.IsAuthed() {
			jsonError(ctx, http.StatusForbidden, "forbidden")
			return
		}

		var in content.PostInput
		if err := ctx.Decode(&in); err != nil {
			jsonError(ctx, http.StatusBadRequest, "invalid JSON body")
			return
		}

		p, err := in.Build(nil)
		if err != nil {
			jsonError(ctx, http.StatusBadRequest, err.Error())
			return
		}

		if err := db.CreatePost(p); err != nil {
			if errors.Is(err, store.ErrConflict) {
				jsonError(ctx, http.StatusConflict, "slug already exists")
				return
			}
			slog.Error("create post", "err", err)
			jsonError(ctx, http.StatusInternalServerError, "failed to create post")
			return
		}

		created, err := db.GetPost(p.Slug)
		if err != nil {
			created = p
		}
		ctx.JSON(http.StatusCreated, created)
	}
}

func UpdatePost(db store.Store) arry.Handler {
	return func(ctx arry.Context) {
		if !ctx.IsAuthed() {
			jsonError(ctx, http.StatusForbidden, "forbidden")
			return
		}

		slug := ctx.Param("slug")

		existing, err := db.GetPost(slug)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				jsonError(ctx, http.StatusNotFound, "post not found")
				return
			}
			slog.Error("load post for update", "slug", slug, "err", err)
			jsonError(ctx, http.StatusInternalServerError, "failed to update post")
			return
		}

		var in content.PostInput
		if err := ctx.Decode(&in); err != nil {
			jsonError(ctx, http.StatusBadRequest, "invalid JSON body")
			return
		}

		p, err := in.Build(existing)
		if err != nil {
			jsonError(ctx, http.StatusBadRequest, err.Error())
			return
		}

		if err := db.UpdatePost(slug, p); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				jsonError(ctx, http.StatusNotFound, "post not found")
				return
			}
			if errors.Is(err, store.ErrConflict) {
				jsonError(ctx, http.StatusConflict, "slug already exists")
				return
			}
			slog.Error("update post", "slug", slug, "err", err)
			jsonError(ctx, http.StatusInternalServerError, "failed to update post")
			return
		}

		updated, err := db.GetPost(slug)
		if err != nil {
			updated = p
		}
		ctx.JSON(http.StatusOK, updated)
	}
}

func DeletePost(db store.Store) arry.Handler {
	return func(ctx arry.Context) {
		if !ctx.IsAuthed() {
			jsonError(ctx, http.StatusForbidden, "forbidden")
			return
		}

		slug := ctx.Param("slug")
		if err := db.DeletePost(slug); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				jsonError(ctx, http.StatusNotFound, "post not found")
				return
			}
			slog.Error("delete post", "slug", slug, "err", err)
			jsonError(ctx, http.StatusInternalServerError, "failed to delete post")
			return
		}

		ctx.JSON(http.StatusOK, map[string]string{"message": "deleted"})
	}
}
