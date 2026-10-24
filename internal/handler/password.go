package handler

import (
	"crypto/subtle"
	"errors"
	"net/http"

	"github.com/pionus/arry"
	"github.com/pionus/pionus-go/internal/model"
	"github.com/pionus/pionus-go/internal/store"
)

// UnlockPost verifies a password-protected post and sets an access cookie.
func UnlockPost(db store.Store, guard *PostGuard) arry.Handler {
	return func(ctx arry.Context) {
		slug := ctx.Param("slug")
		post, err := db.GetPost(slug)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				notFound(ctx, db, "文章不存在")
				return
			}
			ctx.Reply(http.StatusInternalServerError)
			return
		}
		if !post.IsLocked() {
			ctx.Redirect(http.StatusSeeOther, "/article/"+slug)
			return
		}
		if err := ctx.Request().ParseForm(); err != nil {
			renderPassword(ctx, db, post, "表单无法解析。", http.StatusBadRequest)
			return
		}

		password := ctx.Request().PostFormValue("password")
		if subtle.ConstantTimeCompare([]byte(password), []byte(post.PostPassword)) != 1 {
			renderPassword(ctx, db, post, "密码不正确。", http.StatusUnauthorized)
			return
		}

		guard.Grant(ctx, post)
		ctx.Redirect(http.StatusSeeOther, "/article/"+slug)
	}
}

func renderPassword(ctx arry.Context, db store.Store, post *model.Post, errMsg string, status int) {
	data := baseData(ctx, db, post.Title)
	data["Post"] = post
	data["Error"] = errMsg
	data["UnlockAction"] = "/article/" + post.Slug + "/unlock"
	data["CSRF"] = csrfToken(ctx)
	data["NoIndex"] = true
	ctx.Render(status, "password.html", data)
}
