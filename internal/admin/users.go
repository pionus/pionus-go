package admin

import (
	"errors"
	"net/http"
	"strings"

	"github.com/pionus/arry"
	"github.com/pionus/pionus-go/internal/auth"
	"github.com/pionus/pionus-go/internal/model"
	"github.com/pionus/pionus-go/internal/store"
)

func (a *Admin) userList(ctx arry.Context) {
	users, err := a.store.ListUsers()
	if err != nil {
		logErr("admin list users", "err", err)
		ctx.Reply(http.StatusInternalServerError)
		return
	}
	data := a.base(ctx, "用户")
	data["Users"] = users
	switch ctx.Query("error") {
	case "invalid":
		data["Error"] = "用户名、邮箱或密码格式不正确。"
	case "conflict":
		data["Error"] = "用户名或邮箱已存在。"
	case "last_owner":
		data["Error"] = "不能删除最后一个管理员。"
	case "self":
		data["Error"] = "不能删除当前登录账号。"
	}
	a.render(ctx, http.StatusOK, "users.html", data)
}

func (a *Admin) userCreate(ctx arry.Context) {
	form := ctx.Request()
	username := strings.TrimSpace(form.PostFormValue("username"))
	email := strings.TrimSpace(form.PostFormValue("email"))
	screenName := strings.TrimSpace(form.PostFormValue("screen_name"))
	password := form.PostFormValue("password")
	role := form.PostFormValue("role")

	current := auth.CurrentUser(ctx)
	if !auth.ValidUsername(username) || !validEmail(email) || len(password) < 8 || !validRole(role) {
		a.redirect(ctx, "/admin/users?error=invalid")
		return
	}
	if role == model.RoleOwner && (current == nil || current.Role != model.RoleOwner) {
		a.redirect(ctx, "/admin/users?error=invalid")
		return
	}
	if screenName == "" {
		screenName = username
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		logErr("hash user password", "err", err)
		a.redirect(ctx, "/admin/users?error=invalid")
		return
	}
	user := &model.User{
		Username: username, Email: email, PasswordHash: hash,
		ScreenName: screenName, Role: role,
	}
	if err := a.store.CreateUser(user); err != nil {
		if errors.Is(err, store.ErrConflict) {
			a.redirect(ctx, "/admin/users?error=conflict")
			return
		}
		logErr("admin create user", "err", err)
		a.redirect(ctx, "/admin/users?error=invalid")
		return
	}
	a.redirect(ctx, "/admin/users?saved=1")
}

func (a *Admin) userDelete(ctx arry.Context) {
	id := atoiDefault(ctx.Request().PostFormValue("id"), 0)
	if id <= 0 {
		ctx.Reply(http.StatusBadRequest)
		return
	}
	current := auth.CurrentUser(ctx)
	if current != nil && current.ID == int64(id) {
		a.redirect(ctx, "/admin/users?error=self")
		return
	}
	user, err := a.store.GetUser(int64(id))
	if err != nil {
		a.redirect(ctx, "/admin/users?deleted=1")
		return
	}
	if user.Role == model.RoleOwner {
		users, _ := a.store.ListUsers()
		owners := 0
		for _, u := range users {
			if u.Role == model.RoleOwner {
				owners++
			}
		}
		if owners <= 1 {
			a.redirect(ctx, "/admin/users?error=last_owner")
			return
		}
	}
	if err := a.store.DeleteUser(int64(id)); err != nil && !errors.Is(err, store.ErrNotFound) {
		logErr("admin delete user", "id", id, "err", err)
	}
	a.redirect(ctx, "/admin/users?deleted=1")
}

func validRole(role string) bool {
	switch role {
	case model.RoleOwner, model.RoleAdmin, model.RoleEditor, model.RoleContributor:
		return true
	}
	return false
}
