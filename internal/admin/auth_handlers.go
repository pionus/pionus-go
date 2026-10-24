package admin

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/pionus/arry"
	"github.com/pionus/pionus-go/internal/auth"
	"github.com/pionus/pionus-go/internal/model"
	"github.com/pionus/pionus-go/internal/store"
)

func (a *Admin) loginPage(ctx arry.Context) {
	if auth.CurrentUser(ctx) != nil {
		a.redirect(ctx, auth.AdminBase)
		return
	}
	data := a.base(ctx, "登录")
	data["Next"] = safeNext(ctx.Query("next"))
	a.render(ctx, http.StatusOK, "login.html", data)
}

func (a *Admin) loginSubmit(ctx arry.Context) {
	form := ctx.Request()
	username := strings.TrimSpace(form.PostFormValue("username"))
	password := form.PostFormValue("password")
	next := safeNext(form.PostFormValue("next"))

	if !a.loginLimits.Allow(username) {
		data := a.base(ctx, "登录")
		data["Error"] = "尝试次数过多，请 15 分钟后再试。"
		data["Username"] = username
		data["Next"] = next
		a.render(ctx, http.StatusTooManyRequests, "login.html", data)
		return
	}

	user, err := a.store.GetUserByUsername(username)
	ok := false
	if err == nil {
		ok = auth.CheckPassword(user.PasswordHash, password)
	} else {
		// Keep timing similar for unknown accounts.
		auth.CheckPassword("", password)
	}
	if !ok {
		a.loginLimits.Fail(username)
		data := a.base(ctx, "登录")
		data["Error"] = "用户名或密码不正确。"
		data["Username"] = username
		data["Next"] = next
		a.render(ctx, http.StatusUnauthorized, "login.html", data)
		return
	}

	a.loginLimits.Reset(username)
	if err := a.auth.Login(ctx, user); err != nil {
		logErr("admin login", "err", err)
		data := a.base(ctx, "登录")
		data["Error"] = "登录失败，请稍后再试。"
		a.render(ctx, http.StatusInternalServerError, "login.html", data)
		return
	}
	a.redirect(ctx, next)
}

func (a *Admin) logout(ctx arry.Context) {
	if err := a.auth.Logout(ctx); err != nil {
		logErr("admin logout", "err", err)
	}
	a.redirect(ctx, auth.AdminBase+"/login")
}

func (a *Admin) setupPage(ctx arry.Context) {
	if a.userCount() > 0 {
		a.redirect(ctx, auth.AdminBase+"/login")
		return
	}

	data := a.base(ctx, "初始化")
	data["FormSiteTitle"] = "我的博客"
	data["SiteAuthor"] = ""
	a.render(ctx, http.StatusOK, "setup.html", data)
}

func (a *Admin) setupSubmit(ctx arry.Context) {
	if a.userCount() > 0 {
		a.redirect(ctx, auth.AdminBase+"/login")
		return
	}

	form := ctx.Request()
	username := strings.TrimSpace(form.PostFormValue("username"))
	email := strings.TrimSpace(form.PostFormValue("email"))
	password := form.PostFormValue("password")
	confirm := form.PostFormValue("confirm")
	siteTitle := strings.TrimSpace(form.PostFormValue("site_title"))

	fail := func(msg string) {
		data := a.base(ctx, "初始化")
		data["Error"] = msg
		data["Username"] = username
		data["Email"] = email
		data["FormSiteTitle"] = siteTitle
		a.render(ctx, http.StatusBadRequest, "setup.html", data)
	}

	if !auth.ValidUsername(username) {
		fail("用户名需要 3–32 位字母、数字、下划线或连字符。")
		return
	}
	if !validEmail(email) {
		fail("请输入有效的邮箱地址。")
		return
	}
	if len(password) < 8 {
		fail("密码至少需要 8 个字符。")
		return
	}
	if password != confirm {
		fail("两次输入的密码不一致。")
		return
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		logErr("hash password", "err", err)
		fail("创建账号失败，请稍后再试。")
		return
	}

	user := &model.User{
		Username:     username,
		Email:        email,
		PasswordHash: hash,
		ScreenName:   username,
		Role:         model.RoleOwner,
	}
	if err := a.store.CreateUser(user); err != nil {
		if err == store.ErrConflict {
			fail("用户名或邮箱已被使用。")
			return
		}
		logErr("create owner", "err", err)
		fail("创建账号失败，请稍后再试。")
		return
	}

	if siteTitle != "" {
		_ = a.store.SetOption("site.title", siteTitle)
	}
	_ = a.store.SetOption("site.author", username)

	if err := a.auth.Login(ctx, user); err != nil {
		logErr("setup login", "err", err)
		a.redirect(ctx, auth.AdminBase+"/login")
		return
	}
	a.redirect(ctx, auth.AdminBase)
}

func (a *Admin) userCount() int {
	users, err := a.store.ListUsers()
	if err != nil {
		logErr("count users", "err", err)
		return 0
	}
	return len(users)
}

func validEmail(email string) bool {
	if len(email) < 3 || len(email) > 254 || strings.ContainsAny(email, " \t\r\n") {
		return false
	}
	at := strings.LastIndex(email, "@")
	return at > 0 && at < len(email)-1 && strings.Contains(email[at+1:], ".")
}

// safeNext only allows redirects inside the admin area.
func safeNext(next string) string {
	if next == "" {
		return auth.AdminBase
	}
	u, err := url.Parse(next)
	if err != nil || u.Host != "" || u.Scheme != "" {
		return auth.AdminBase
	}
	if u.Path != auth.AdminBase && !strings.HasPrefix(u.Path, auth.AdminBase+"/") {
		return auth.AdminBase
	}
	if u.RawQuery != "" {
		return u.Path + "?" + u.RawQuery
	}
	return u.Path
}
