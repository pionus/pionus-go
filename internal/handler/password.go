package handler

import (
	"crypto/subtle"
	"errors"
	"net/http"

	"github.com/pionus/arry"
	"github.com/pionus/pionus-go/internal/auth"
	"github.com/pionus/pionus-go/internal/model"
	"github.com/pionus/pionus-go/internal/store"
)

// UnlockPost verifies a password-protected post and sets an access cookie.
func UnlockPost(deps Deps, guard *PostGuard) arry.Handler {
	return func(ctx arry.Context) {
		slug := ctx.Param("slug")
		post, err := deps.Posts.GetPost(slug)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				notFound(ctx, deps, "文章不存在")
				return
			}
			ctx.Reply(http.StatusInternalServerError)
			return
		}
		hidden := HiddenTags(post)
		if !post.IsLocked() && len(hidden) == 0 {
			ctx.Redirect(http.StatusSeeOther, "/article/"+slug)
			return
		}
		if err := ctx.Request().ParseForm(); err != nil {
			renderPassword(ctx, deps, post, "表单无法解析。", http.StatusBadRequest)
			return
		}

		password := ctx.Request().PostFormValue("password")
		granted := false
		if post.IsLocked() && subtle.ConstantTimeCompare([]byte(password), []byte(post.PostPassword)) == 1 {
			guard.Grant(ctx, post)
			granted = true
		}
		for _, tag := range hidden {
			if tag.PasswordHash != "" && auth.CheckPassword(tag.PasswordHash, password) {
				guard.GrantTag(ctx, tag)
				granted = true
				break
			}
		}
		if !granted {
			renderPassword(ctx, deps, post, "密码不正确。", http.StatusUnauthorized)
			return
		}

		ctx.Redirect(http.StatusSeeOther, "/article/"+slug)
	}
}

func renderPassword(ctx arry.Context, deps Deps, post *model.Post, errMsg string, status int) {
	safe := *post
	// The password page must not leak the body through meta description tags.
	safe.Content = ""
	safe.Excerpt = ""
	data := baseData(ctx, deps, safe.Title)
	data["Post"] = &safe
	data["Error"] = errMsg
	data["UnlockAction"] = "/article/" + post.Slug + "/unlock"
	data["CSRF"] = csrfToken(ctx)
	data["NoIndex"] = true
	ctx.Render(status, "password.html", data)
}

// UnlockTag verifies a hidden-tag password and sets its access cookie.
func UnlockTag(deps Deps, guard *PostGuard) arry.Handler {
	return func(ctx arry.Context) {
		slug := ctx.Param("slug")
		tag, err := deps.Taxonomy.GetTag(slug)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				notFound(ctx, deps, "标签不存在")
				return
			}
			ctx.Reply(http.StatusInternalServerError)
			return
		}
		if !tag.Hidden {
			ctx.Redirect(http.StatusSeeOther, "/tag/"+slug)
			return
		}
		if err := ctx.Request().ParseForm(); err != nil {
			renderTagPassword(ctx, deps, tag, "表单无法解析。", http.StatusBadRequest)
			return
		}
		if !auth.CheckPassword(tag.PasswordHash, ctx.Request().PostFormValue("password")) {
			renderTagPassword(ctx, deps, tag, "密码不正确。", http.StatusUnauthorized)
			return
		}
		guard.GrantTag(ctx, tag)
		ctx.Redirect(http.StatusSeeOther, "/tag/"+slug)
	}
}

func renderTagPassword(ctx arry.Context, deps Deps, tag *model.Tag, errMsg string, status int) {
	data := baseData(ctx, deps, tag.Name)
	data["Tag"] = tag
	data["Error"] = errMsg
	data["UnlockAction"] = "/tag/" + tag.Slug + "/unlock"
	data["CSRF"] = csrfToken(ctx)
	data["NoIndex"] = true
	ctx.Render(status, "tag-password.html", data)
}
