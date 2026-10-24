package handler

import (
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

var slugRe = regexp.MustCompile(`^[\p{L}\p{N}_-]+$`)

type errorResponse struct {
	Error string `json:"error"`
}

type listResponse struct {
	Posts []*model.Post `json:"posts"`
	Total int           `json:"total"`
	Page  int           `json:"page"`
	Limit int           `json:"limit"`
}

// postInput is the JSON/form payload for creating and updating a post.
// Pointer fields let updates distinguish "not provided" from zero values.
type postInput struct {
	Slug         string     `json:"slug"`
	Title        string     `json:"title"`
	Content      string     `json:"content"`
	Excerpt      string     `json:"excerpt"`
	Cover        string     `json:"cover"`
	Type         string     `json:"type"`
	Status       string     `json:"status"`
	AuthorID     *int64     `json:"author_id"`
	Password     string     `json:"password"`
	AllowComment *bool      `json:"allow_comment"`
	Pinned       *bool      `json:"pinned"`
	PublishedAt  *time.Time `json:"published_at"`
	CreatedAt    *time.Time `json:"created_at"`
	CategoryIDs  []int64    `json:"category_ids"`
	TagIDs       []int64    `json:"tag_ids"`
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

func GetPost(db store.Store) arry.Handler {
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

		if !p.IsPublished() && !ctx.IsAuthed() {
			jsonError(ctx, http.StatusNotFound, "post not found")
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

		var in postInput
		if err := ctx.Decode(&in); err != nil {
			jsonError(ctx, http.StatusBadRequest, "invalid JSON body")
			return
		}

		p, msg := postFromInput(&in, nil)
		if msg != "" {
			jsonError(ctx, http.StatusBadRequest, msg)
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

		var in postInput
		if err := ctx.Decode(&in); err != nil {
			jsonError(ctx, http.StatusBadRequest, "invalid JSON body")
			return
		}

		p, msg := postFromInput(&in, existing)
		if msg != "" {
			jsonError(ctx, http.StatusBadRequest, msg)
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

// postFromInput maps the request payload onto a model. When existing is nil
// the post is being created; otherwise unspecified fields keep their current
// values.
func postFromInput(in *postInput, existing *model.Post) (*model.Post, string) {
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return nil, "title is required"
	}

	if existing == nil {
		slug := strings.TrimSpace(in.Slug)
		if slug == "" {
			slug = model.Slugify(title)
		}
		if slug == "" || !slugRe.MatchString(slug) {
			return nil, "slug must contain only letters, numbers, hyphens, and underscores"
		}
		if strings.TrimSpace(in.Content) == "" {
			return nil, "content is required"
		}
		in.Slug = slug
	}

	p := &model.Post{
		Title:        title,
		Content:      in.Content,
		Excerpt:      strings.TrimSpace(in.Excerpt),
		Cover:        in.Cover,
		Type:         in.Type,
		Status:       in.Status,
		AuthorID:     in.AuthorID,
		PostPassword: in.Password,
		CategoryIDs:  in.CategoryIDs,
		TagIDs:       in.TagIDs,
	}

	if existing != nil {
		p.ID = existing.ID
		p.Slug = existing.Slug
		p.CreatedAt = existing.CreatedAt
		p.Views = existing.Views
		p.AuthorID = existing.AuthorID
		p.PublishedAt = existing.PublishedAt
		p.AllowComment = existing.AllowComment
		p.Pinned = existing.Pinned
		if p.Type == "" {
			p.Type = existing.Type
		}
		if p.Status == "" {
			p.Status = existing.Status
		}
		if in.AuthorID != nil {
			p.AuthorID = in.AuthorID
		}
	} else {
		p.Slug = in.Slug
		if p.Status == "" {
			p.Status = model.StatusPublish
		}
		p.AllowComment = true
	}

	if p.Type == "" {
		p.Type = model.PostTypePost
	}
	if p.Status == "" {
		p.Status = model.StatusDraft
	}
	if !validPostType(p.Type) {
		return nil, "invalid post type"
	}
	if !validPostStatus(p.Status) {
		return nil, "invalid post status"
	}

	if in.AllowComment != nil {
		p.AllowComment = *in.AllowComment
	}
	if in.Pinned != nil {
		p.Pinned = *in.Pinned
	}

	if in.PublishedAt != nil {
		p.PublishedAt = in.PublishedAt
	}
	if existing == nil && in.CreatedAt != nil {
		p.CreatedAt = *in.CreatedAt
	}

	if p.Excerpt == "" {
		p.Excerpt = model.MakeExcerpt(p.Content, 200)
	}

	return p, ""
}

func validPostType(t string) bool {
	return t == model.PostTypePost || t == model.PostTypePage
}

func validPostStatus(s string) bool {
	switch s {
	case model.StatusPublish, model.StatusDraft, model.StatusHidden, model.StatusWaiting:
		return true
	}
	return false
}
