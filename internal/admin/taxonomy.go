package admin

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/pionus/arry"
	"github.com/pionus/pionus-go/internal/auth"
	"github.com/pionus/pionus-go/internal/model"
	"github.com/pionus/pionus-go/internal/store"
)

func (a *Admin) categoryList(ctx arry.Context) {
	categories, err := a.store.ListCategories()
	if err != nil {
		logErr("admin list categories", "err", err)
		ctx.Reply(http.StatusInternalServerError)
		return
	}

	data := a.base(ctx, "分类")
	data["Categories"] = categories
	a.render(ctx, http.StatusOK, "categories.html", data)
}

func (a *Admin) categoryCreate(ctx arry.Context) {
	form := ctx.Request()
	name := strings.TrimSpace(form.PostFormValue("name"))
	slug := strings.TrimSpace(form.PostFormValue("slug"))
	description := strings.TrimSpace(form.PostFormValue("description"))
	parentID := parseOptionalID(form.PostFormValue("parent_id"))
	sortOrder, _ := strconv.Atoi(form.PostFormValue("sort_order"))

	if name == "" {
		a.redirect(ctx, auth.AdminBase+"/categories")
		return
	}
	if slug == "" {
		slug = model.Slugify(name)
	}

	c := &model.Category{
		Name:        name,
		Slug:        slug,
		Description: description,
		ParentID:    parentID,
		SortOrder:   sortOrder,
	}
	if err := a.store.CreateCategory(c); err != nil {
		logErr("admin create category", "err", err)
		a.redirect(ctx, auth.AdminBase+"/categories")
		return
	}
	a.redirect(ctx, auth.AdminBase+"/categories?saved=1")
}

func (a *Admin) categoryUpdate(ctx arry.Context) {
	form := ctx.Request()
	original := strings.TrimSpace(form.PostFormValue("original_slug"))
	name := strings.TrimSpace(form.PostFormValue("name"))
	slug := strings.TrimSpace(form.PostFormValue("slug"))

	if original == "" || name == "" {
		a.redirect(ctx, auth.AdminBase+"/categories")
		return
	}
	if slug == "" {
		slug = model.Slugify(name)
	}

	c := &model.Category{
		Name:        name,
		Slug:        slug,
		Description: strings.TrimSpace(form.PostFormValue("description")),
		ParentID:    parseOptionalID(form.PostFormValue("parent_id")),
		SortOrder:   atoiDefault(form.PostFormValue("sort_order"), 0),
	}
	if err := a.store.UpdateCategory(original, c); err != nil {
		logErr("admin update category", "slug", original, "err", err)
		a.redirect(ctx, auth.AdminBase+"/categories")
		return
	}
	a.redirect(ctx, auth.AdminBase+"/categories?saved=1")
}

func (a *Admin) categoryDelete(ctx arry.Context) {
	slug := strings.TrimSpace(ctx.Request().PostFormValue("slug"))
	if slug != "" {
		if err := a.store.DeleteCategory(slug); err != nil && !errors.Is(err, store.ErrNotFound) {
			logErr("admin delete category", "slug", slug, "err", err)
		}
	}
	a.redirect(ctx, auth.AdminBase+"/categories?deleted=1")
}

func (a *Admin) tagList(ctx arry.Context) {
	tags, err := a.store.ListTags()
	if err != nil {
		logErr("admin list tags", "err", err)
		ctx.Reply(http.StatusInternalServerError)
		return
	}

	data := a.base(ctx, "标签")
	data["Tags"] = tags
	switch ctx.Query("error") {
	case "password":
		data["Error"] = "隐藏标签必须设置访问密码。"
	case "save":
		data["Error"] = "保存标签失败，请稍后再试。"
	}
	a.render(ctx, http.StatusOK, "tags.html", data)
}

func (a *Admin) tagCreate(ctx arry.Context) {
	form := ctx.Request()
	name := strings.TrimSpace(form.PostFormValue("name"))
	slug := strings.TrimSpace(form.PostFormValue("slug"))
	if name == "" {
		a.redirect(ctx, auth.AdminBase+"/tags")
		return
	}
	if slug == "" {
		slug = model.Slugify(name)
	}
	hidden := form.PostFormValue("hidden") != ""
	password := form.PostFormValue("password")
	hash := ""
	if password != "" {
		var err error
		hash, err = auth.HashPassword(password)
		if err != nil {
			logErr("admin hash tag password", "err", err)
			a.redirect(ctx, auth.AdminBase+"/tags?error=save")
			return
		}
	}
	if hidden && hash == "" {
		a.redirect(ctx, auth.AdminBase+"/tags?error=password")
		return
	}
	t := &model.Tag{Name: name, Slug: slug, Hidden: hidden, PasswordHash: hash}
	if err := a.store.CreateTag(t); err != nil {
		logErr("admin create tag", "err", err)
		a.redirect(ctx, auth.AdminBase+"/tags?error=save")
		return
	}
	a.redirect(ctx, auth.AdminBase+"/tags?saved=1")
}

func (a *Admin) tagUpdate(ctx arry.Context) {
	form := ctx.Request()
	original := strings.TrimSpace(form.PostFormValue("original_slug"))
	name := strings.TrimSpace(form.PostFormValue("name"))
	slug := strings.TrimSpace(form.PostFormValue("slug"))

	if original == "" || name == "" {
		a.redirect(ctx, auth.AdminBase+"/tags")
		return
	}
	if slug == "" {
		slug = model.Slugify(name)
	}

	existing, err := a.store.GetTag(original)
	if err != nil {
		logErr("admin load tag", "slug", original, "err", err)
		a.redirect(ctx, auth.AdminBase+"/tags?error=save")
		return
	}

	hidden := form.PostFormValue("hidden") != ""
	password := form.PostFormValue("password")
	hash := existing.PasswordHash
	if password != "" {
		hash, err = auth.HashPassword(password)
		if err != nil {
			logErr("admin hash tag password", "err", err)
			a.redirect(ctx, auth.AdminBase+"/tags?error=save")
			return
		}
	}
	if hidden && hash == "" {
		a.redirect(ctx, auth.AdminBase+"/tags?error=password")
		return
	}

	t := &model.Tag{Name: name, Slug: slug, Hidden: hidden, PasswordHash: hash}
	if err := a.store.UpdateTag(original, t); err != nil {
		logErr("admin update tag", "slug", original, "err", err)
		a.redirect(ctx, auth.AdminBase+"/tags?error=save")
		return
	}
	a.redirect(ctx, auth.AdminBase+"/tags?saved=1")
}

func (a *Admin) tagDelete(ctx arry.Context) {
	slug := strings.TrimSpace(ctx.Request().PostFormValue("slug"))
	if slug != "" {
		if err := a.store.DeleteTag(slug); err != nil && !errors.Is(err, store.ErrNotFound) {
			logErr("admin delete tag", "slug", slug, "err", err)
		}
	}
	a.redirect(ctx, auth.AdminBase+"/tags?deleted=1")
}

func parseOptionalID(raw string) *int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "0" {
		return nil
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return nil
	}
	return &id
}

func atoiDefault(raw string, def int) int {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return def
	}
	return n
}
