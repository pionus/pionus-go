package admin

import (
	"github.com/pionus/arry"
	"github.com/pionus/pionus-go/internal/auth"
	"net/http"
)

func (a *Admin) profilePage(ctx arry.Context) {
	data := a.base(ctx, "个人资料")
	data["ProfileUser"] = auth.CurrentUser(ctx)
	a.render(ctx, http.StatusOK, "profile.html", data)
}

func (a *Admin) profileSave(ctx arry.Context) {
	user := auth.CurrentUser(ctx)
	if user == nil {
		ctx.Reply(http.StatusForbidden)
		return
	}

	form := ctx.Request()
	current := form.PostFormValue("current_password")
	password := form.PostFormValue("password")
	confirm := form.PostFormValue("confirm")

	fail := func(msg string) {
		data := a.base(ctx, "个人资料")
		data["ProfileUser"] = user
		data["Error"] = msg
		a.render(ctx, http.StatusBadRequest, "profile.html", data)
	}

	if !auth.CheckPassword(user.PasswordHash, current) {
		fail("当前密码不正确。")
		return
	}
	if len(password) < 8 {
		fail("新密码至少需要 8 个字符。")
		return
	}
	if password != confirm {
		fail("两次输入的新密码不一致。")
		return
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		logErr("hash new password", "err", err)
		fail("保存失败，请稍后再试。")
		return
	}
	user.PasswordHash = hash
	if err := a.store.UpdateUser(user); err != nil {
		logErr("update password", "err", err)
		fail("保存失败，请稍后再试。")
		return
	}
	_ = a.store.DeleteUserSessions(user.ID)
	if err := a.auth.Login(ctx, user); err != nil {
		logErr("relogin after password change", "err", err)
		a.redirect(ctx, "/admin/login")
		return
	}
	a.redirect(ctx, "/admin/profile?saved=1")
}

// requireManager guards site-wide settings for admins and owners.
func (a *Admin) requireManager(next arry.Handler) arry.Handler {
	return func(ctx arry.Context) {
		user := auth.CurrentUser(ctx)
		if user == nil || !user.CanManage() {
			ctx.Reply(http.StatusForbidden)
			return
		}
		next(ctx)
	}
}
