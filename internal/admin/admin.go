package admin

import (
	"embed"
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/pionus/arry"
	"github.com/pionus/arry/middlewares"
	"github.com/pionus/pionus-go/internal/auth"
	"github.com/pionus/pionus-go/internal/content"
	"github.com/pionus/pionus-go/internal/handler"
	"github.com/pionus/pionus-go/internal/model"
	"github.com/pionus/pionus-go/internal/store"
)

//go:embed templates/*.html
var templateFS embed.FS

// Options configures the admin area.
type Options struct {
	Store       store.Store
	Auth        *auth.Manager
	Preview     *handler.PostGuard
	UploadDir   string
	MaxUploadMB int
}

// Admin holds the admin UI dependencies and templates.
type Admin struct {
	store        store.Store
	auth         *auth.Manager
	previewGuard *handler.PostGuard
	engine       arry.Engine
	uploadDir    string
	maxUpload    int64
	loginLimits  *loginLimiter
}

// New creates the admin area.
func New(opts Options) *Admin {
	maxMB := opts.MaxUploadMB
	if maxMB <= 0 {
		maxMB = 10
	}

	a := &Admin{
		store:        opts.Store,
		auth:         opts.Auth,
		previewGuard: opts.Preview,
		uploadDir:    opts.UploadDir,
		maxUpload:    int64(maxMB) << 20,
		loginLimits:  newLoginLimiter(5, 15*time.Minute),
	}
	a.engine = arry.NewEngineWithConfig(arry.EngineConfig{
		Type:      arry.EngineHTML,
		Dir:       "templates",
		FS:        templateFS,
		Extension: "html",
		Cache:     true,
		FuncMap:   templateFuncs(),
	})
	return a
}

// Register mounts the admin area on the app at /admin.
func (a *Admin) Register(app *arry.Arry) {
	bodyLimit := middlewares.BodyLimit(a.maxUpload + 1<<20)
	csrf := middlewares.CSRFWithConfig(middlewares.CSRFConfig{
		CookieName: "pionus_csrf",
		Secure:     a.auth.Secure,
		MaxAge:     12 * 60 * 60,
	})

	// Login and first-run setup are reachable without a session.
	open := arry.NewRouter(bodyLimit, csrf)
	open.Get("/login", a.loginPage)
	open.Post("/login", a.loginSubmit)
	open.Get("/setup", a.setupPage)
	open.Post("/setup", a.setupSubmit)
	app.Router().Graft(auth.AdminBase, open)

	// Everything else requires a signed-in user.
	priv := arry.NewRouter(bodyLimit, csrf, a.auth.RequireUser)
	priv.Get("/", a.dashboard)
	priv.Post("/logout", a.logout)
	priv.Get("/profile", a.profilePage)
	priv.Post("/profile", a.profileSave)
	priv.Get("/users", a.requireManager(a.userList))
	priv.Post("/users/create", a.requireManager(a.userCreate))
	priv.Post("/users/delete", a.requireManager(a.userDelete))

	priv.Get("/posts", a.postList)
	priv.Get("/posts/new", a.postNew)
	priv.Post("/posts", a.postCreate)
	priv.Get("/posts/:id/edit", a.postEdit)
	priv.Post("/posts/:id", a.postUpdate)
	priv.Post("/posts/:id/delete", a.postDelete)
	priv.Post("/preview", a.preview)

	priv.Get("/categories", a.categoryList)
	priv.Post("/categories/create", a.categoryCreate)
	priv.Post("/categories/update", a.categoryUpdate)
	priv.Post("/categories/delete", a.categoryDelete)

	priv.Get("/tags", a.tagList)
	priv.Post("/tags/create", a.tagCreate)
	priv.Post("/tags/update", a.tagUpdate)
	priv.Post("/tags/delete", a.tagDelete)

	priv.Get("/notes", a.noteList)
	priv.Get("/notes/:id/edit", a.noteEditPage)
	priv.Post("/notes/:id", a.noteUpdate)
	priv.Post("/notes/create", a.noteCreate)
	priv.Post("/notes/delete", a.noteDelete)

	priv.Get("/comments", a.commentList)
	priv.Post("/comments/status", a.commentStatus)
	priv.Post("/comments/delete", a.commentDelete)

	priv.Get("/media", a.mediaList)
	priv.Post("/media/upload", a.mediaUpload)
	priv.Post("/media/delete", a.mediaDelete)

	priv.Get("/settings", a.requireManager(a.settingsPage))
	priv.Post("/settings", a.requireManager(a.settingsSave))

	app.Router().Graft(auth.AdminBase, priv)
}

func (a *Admin) render(ctx arry.Context, code int, name string, data map[string]any) {
	ctx.SetEngine(a.engine)
	ctx.Render(code, name, data)
}

// base builds the data shared by every admin page.
func (a *Admin) base(ctx arry.Context, title string) map[string]any {
	site := a.option("site.title", "Pionus")
	flash := ""
	q := ctx.Request().URL.Query()
	switch {
	case q.Get("saved") != "":
		flash = "已保存"
	case q.Get("deleted") != "":
		flash = "已删除"
	case q.Get("uploaded") != "":
		flash = "上传成功"
	case q.Get("ok") != "":
		flash = "操作成功"
	}

	return map[string]any{
		"Title":       title,
		"SiteTitle":   site,
		"User":        auth.CurrentUser(ctx),
		"CSRF":        middlewares.CSRFToken(ctx),
		"Path":        ctx.Request().URL.Path,
		"Flash":       flash,
		"CurrentYear": time.Now().Year(),
	}
}

func (a *Admin) option(name, def string) string {
	v, err := a.store.GetOption(name)
	if err != nil || v == "" {
		return def
	}
	return v
}

func (a *Admin) intOption(name string, def int) int {
	v, err := a.store.GetOption(name)
	if err != nil {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return def
	}
	return n
}

func (a *Admin) redirect(ctx arry.Context, to string) {
	ctx.Redirect(http.StatusFound, to)
}

func parseID(ctx arry.Context) (int64, bool) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		ctx.Reply(http.StatusNotFound)
		return 0, false
	}
	return id, true
}

func pageOf(ctx arry.Context, defaultLimit int) (page, limit int) {
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

// templateFuncs are helpers available to admin templates.
func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"date": func(t time.Time) string {
			if t.IsZero() {
				return ""
			}
			return t.Format("2006-01-02")
		},
		"datetime": func(t time.Time) string {
			if t.IsZero() {
				return ""
			}
			return t.Format("2006-01-02 15:04")
		},
		"datePtr": func(t *time.Time) string {
			if t == nil || t.IsZero() {
				return ""
			}
			return t.Format("2006-01-02")
		},
		"datetimeLocal": func(t *time.Time) string {
			if t == nil || t.IsZero() {
				return ""
			}
			return t.Format("2006-01-02T15:04")
		},
		"statusText": func(s string) string {
			switch s {
			case model.StatusPublish:
				return "已发布"
			case model.StatusDraft:
				return "草稿"
			case model.StatusHidden:
				return "私密"
			case model.StatusWaiting:
				return "待审核"
			}
			return s
		},
		"typeText": func(t string) string {
			if t == model.PostTypePage {
				return "页面"
			}
			return "文章"
		},
		"commentStatusText": func(s string) string {
			switch s {
			case model.CommentApproved:
				return "已通过"
			case model.CommentPending:
				return "待审核"
			case model.CommentSpam:
				return "垃圾"
			}
			return s
		},
		"truncate": func(s string, n int) string {
			r := []rune(s)
			if len(r) <= n {
				return s
			}
			return string(r[:n]) + "…"
		},
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		"hasCategory": func(ids []int64, id int64) bool {
			for _, v := range ids {
				if v == id {
					return true
				}
			}
			return false
		},
		"idPtr": func(p *int64) int64 {
			if p == nil {
				return 0
			}
			return *p
		},
		"contains": func(s, sub string) bool { return strings.Contains(s, sub) },
	}
}

// friendlyError keeps validation errors readable in the UI.
func friendlyError(err error) string {
	if errors.Is(err, content.ErrTitleRequired) {
		return "标题不能为空"
	}
	return err.Error()
}

// logger is a small indirection so handlers can log consistently.
func logErr(msg string, args ...any) {
	slog.Error(msg, args...)
}
