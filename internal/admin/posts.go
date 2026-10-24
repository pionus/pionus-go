package admin

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/pionus/arry"
	"github.com/pionus/pionus-go/internal/auth"
	"github.com/pionus/pionus-go/internal/content"
	"github.com/pionus/pionus-go/internal/model"
	"github.com/pionus/pionus-go/internal/store"
)

func (a *Admin) postList(ctx arry.Context) {
	page, limit := pageOf(ctx, 20)
	status := ctx.Query("status")
	typeFilter := ctx.Query("type")
	query := strings.TrimSpace(ctx.Query("q"))

	f := store.PostFilter{Page: page, Limit: limit, Query: query}
	if status != "" && status != "all" {
		f.Statuses = []string{status}
	}
	if typeFilter != "" && typeFilter != "all" {
		f.Type = typeFilter
	}

	posts, total, err := a.store.ListPosts(f)
	if err != nil {
		logErr("admin list posts", "err", err)
		ctx.Reply(http.StatusInternalServerError)
		return
	}

	totalPages := (total + limit - 1) / limit
	if totalPages < 1 {
		totalPages = 1
	}

	data := a.base(ctx, "文章")
	data["Posts"] = posts
	data["Total"] = total
	data["Page"] = page
	data["TotalPages"] = totalPages
	data["HasPrev"] = page > 1
	data["HasNext"] = page < totalPages
	data["PrevPage"] = page - 1
	data["NextPage"] = page + 1
	data["Status"] = status
	data["Type"] = typeFilter
	data["Query"] = query
	a.render(ctx, http.StatusOK, "posts.html", data)
}

func (a *Admin) postNew(ctx arry.Context) {
	p := &model.Post{
		Type:         model.PostTypePost,
		Status:       model.StatusDraft,
		AllowComment: true,
	}
	a.renderEditor(ctx, http.StatusOK, p, true, "", nil, "")
}

func (a *Admin) postEdit(ctx arry.Context) {
	id, ok := parseID(ctx)
	if !ok {
		return
	}

	p, err := a.store.GetPostByID(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			ctx.Reply(http.StatusNotFound)
			return
		}
		logErr("admin load post", "id", id, "err", err)
		ctx.Reply(http.StatusInternalServerError)
		return
	}

	catIDs := make([]int64, 0, len(p.Categories))
	for _, c := range p.Categories {
		catIDs = append(catIDs, c.ID)
	}
	names := make([]string, 0, len(p.Tags))
	for _, t := range p.Tags {
		names = append(names, t.Name)
	}

	a.renderEditor(ctx, http.StatusOK, p, false, "", catIDs, strings.Join(names, ", "))
}

func (a *Admin) postCreate(ctx arry.Context) {
	in, err := a.postInputFromForm(ctx.Request())
	if err != nil {
		a.renderEditor(ctx, http.StatusBadRequest, postFromInput(in), true, friendlyError(err), in.CategoryIDs, tagsValue(ctx))
		return
	}
	if user := auth.CurrentUser(ctx); user != nil {
		in.AuthorID = &user.ID
	}

	p, err := in.Build(nil)
	if err != nil {
		a.renderEditor(ctx, http.StatusBadRequest, postFromInput(in), true, friendlyError(err), in.CategoryIDs, tagsValue(ctx))
		return
	}
	if err := a.store.CreatePost(p); err != nil {
		msg := "保存失败，请稍后再试。"
		if errors.Is(err, store.ErrConflict) {
			msg = "该 slug 已被使用。"
		}
		logErr("admin create post", "err", err)
		a.renderEditor(ctx, http.StatusBadRequest, postFromInput(in), true, msg, in.CategoryIDs, tagsValue(ctx))
		return
	}

	a.redirect(ctx, fmt.Sprintf("%s/posts/%d/edit?saved=1", auth.AdminBase, p.ID))
}

func (a *Admin) postUpdate(ctx arry.Context) {
	id, ok := parseID(ctx)
	if !ok {
		return
	}

	existing, err := a.store.GetPostByID(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			ctx.Reply(http.StatusNotFound)
			return
		}
		logErr("admin load post", "id", id, "err", err)
		ctx.Reply(http.StatusInternalServerError)
		return
	}

	in, err := a.postInputFromForm(ctx.Request())
	if err != nil {
		a.renderEditor(ctx, http.StatusBadRequest, failedPost(in, existing), false, friendlyError(err), in.CategoryIDs, tagsValue(ctx))
		return
	}

	p, err := in.Build(existing)
	if err != nil {
		a.renderEditor(ctx, http.StatusBadRequest, failedPost(in, existing), false, friendlyError(err), in.CategoryIDs, tagsValue(ctx))
		return
	}
	if err := a.store.UpdatePost(existing.Slug, p); err != nil {
		logErr("admin update post", "id", id, "err", err)
		a.renderEditor(ctx, http.StatusBadRequest, failedPost(in, existing), false, "保存失败，请稍后再试。", in.CategoryIDs, tagsValue(ctx))
		return
	}

	a.redirect(ctx, fmt.Sprintf("%s/posts/%d/edit?saved=1", auth.AdminBase, id))
}

func (a *Admin) postDelete(ctx arry.Context) {
	id, ok := parseID(ctx)
	if !ok {
		return
	}

	p, err := a.store.GetPostByID(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			ctx.Reply(http.StatusNotFound)
			return
		}
		logErr("admin load post for delete", "id", id, "err", err)
		ctx.Reply(http.StatusInternalServerError)
		return
	}
	if err := a.store.DeletePost(p.Slug); err != nil {
		logErr("admin delete post", "id", id, "err", err)
		ctx.Reply(http.StatusInternalServerError)
		return
	}
	a.redirect(ctx, auth.AdminBase+"/posts?deleted=1")
}

func (a *Admin) preview(ctx arry.Context) {
	html := content.RenderMarkdown(ctx.Request().PostFormValue("content"))
	ctx.SetContentType("text/html; charset=utf-8")
	ctx.Status(http.StatusOK)
	ctx.Response().Write([]byte(html))
}

func (a *Admin) renderEditor(ctx arry.Context, code int, p *model.Post, isNew bool, errMsg string, selectedCatIDs []int64, tagsValue string) {
	categories, err := a.store.ListCategories()
	if err != nil {
		logErr("admin list categories", "err", err)
	}
	allTags, err := a.store.ListTags()
	if err != nil {
		logErr("admin list tags", "err", err)
	}

	title := "编辑文章"
	if isNew {
		title = "写文章"
	}

	data := a.base(ctx, title)
	data["Post"] = p
	data["IsNew"] = isNew
	data["Categories"] = categories
	data["AllTags"] = allTags
	data["SelectedCategoryIDs"] = selectedCatIDs
	data["TagsValue"] = tagsValue
	data["Error"] = errMsg
	a.render(ctx, code, "editor.html", data)
}

func (a *Admin) postInputFromForm(r *http.Request) (*content.PostInput, error) {
	in := &content.PostInput{
		Slug:     strings.TrimSpace(r.PostFormValue("slug")),
		Title:    strings.TrimSpace(r.PostFormValue("title")),
		Content:  r.PostFormValue("content"),
		Excerpt:  strings.TrimSpace(r.PostFormValue("excerpt")),
		Cover:    strings.TrimSpace(r.PostFormValue("cover")),
		Type:     r.PostFormValue("type"),
		Status:   r.PostFormValue("status"),
		Password: r.PostFormValue("password"),
	}

	allow := r.PostFormValue("allow_comment") != ""
	pinned := r.PostFormValue("pinned") != ""
	in.AllowComment = &allow
	in.Pinned = &pinned

	if raw := strings.TrimSpace(r.PostFormValue("published_at")); raw != "" {
		t, err := time.ParseInLocation("2006-01-02T15:04", raw, time.Local)
		if err != nil {
			return in, fmt.Errorf("发布时间格式不正确")
		}
		in.PublishedAt = &t
	}

	catIDs := make([]int64, 0)
	for _, raw := range r.PostForm["category_ids"] {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			return in, fmt.Errorf("分类参数不正确")
		}
		catIDs = append(catIDs, id)
	}
	in.CategoryIDs = catIDs

	tagIDs := make([]int64, 0)
	for _, name := range splitTags(r.PostFormValue("tags")) {
		tag, err := a.store.UpsertTag(name, "")
		if err != nil {
			return in, fmt.Errorf("保存标签 %q 失败", name)
		}
		tagIDs = append(tagIDs, tag.ID)
	}
	in.TagIDs = tagIDs

	return in, nil
}

// postFromInput builds a display-only post when validation failed, so the
// editor can keep the user's input.
func postFromInput(in *content.PostInput) *model.Post {
	p := &model.Post{
		Slug:        in.Slug,
		Title:       in.Title,
		Content:     in.Content,
		Excerpt:     in.Excerpt,
		Cover:       in.Cover,
		Type:        in.Type,
		Status:      in.Status,
		PublishedAt: in.PublishedAt,
	}
	if in.AllowComment != nil {
		p.AllowComment = *in.AllowComment
	}
	if in.Pinned != nil {
		p.Pinned = *in.Pinned
	}
	if p.Type == "" {
		p.Type = model.PostTypePost
	}
	if p.Status == "" {
		p.Status = model.StatusDraft
	}
	return p
}

func failedPost(in *content.PostInput, existing *model.Post) *model.Post {
	p := postFromInput(in)
	if existing != nil {
		p.ID = existing.ID
		p.Slug = existing.Slug
		p.CreatedAt = existing.CreatedAt
	}
	return p
}

func tagsValue(ctx arry.Context) string {
	return strings.TrimSpace(ctx.Request().PostFormValue("tags"))
}

func splitTags(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '，' || r == '\n' || r == '\r'
	})

	seen := make(map[string]bool, len(parts))
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		name := strings.TrimSpace(part)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
		if len(out) >= 50 {
			break
		}
	}
	return out
}
