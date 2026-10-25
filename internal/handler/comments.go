package handler

import (
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/microcosm-cc/bluemonday"
	"github.com/pionus/arry"
	"github.com/pionus/arry/middlewares"
	"github.com/pionus/pionus-go/internal/events"
	"github.com/pionus/pionus-go/internal/model"
	"github.com/pionus/pionus-go/internal/store"
)

// commentForm holds the values needed to re-render a comment form.
type commentForm struct {
	Author  string
	Email   string
	URL     string
	Content string
	Parent  int64
	Started int64
}

// commentView is a comment plus its sanitized body and children.
type commentView struct {
	*model.Comment
	HTML     template.HTML
	Initial  string
	Color    string
	Depth    int
	Children []*commentView
}

var commentPolicy = func() *bluemonday.Policy {
	p := bluemonday.UGCPolicy()
	p.RequireNoFollowOnLinks(true)
	p.AddTargetBlankToFullyQualifiedLinks(true)
	return p
}()

var (
	commentLimits       = newRateLimiter(5, 10*time.Minute)
	commentGlobalLimits = newRateLimiter(60, 10*time.Minute)
)

var smileRe = regexp.MustCompile(`\{smile:(\d+)\}`)

var smileEmojis = []string{
	"😊", "😄", "😉", "😍", "😘", "😜", "😝", "😳", "😔", "😢",
	"😭", "😡", "😱", "😅", "😎", "😴", "👍", "👎", "🙏", "💪",
}

func renderComment(content string) template.HTML {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\n", "<br>\n")
	withSmiles := smileRe.ReplaceAllStringFunc(content, func(raw string) string {
		m := smileRe.FindStringSubmatch(raw)
		if len(m) < 2 {
			return "😊"
		}
		n, err := strconv.Atoi(m[1])
		if err != nil || n <= 0 {
			return "😊"
		}
		return smileEmojis[(n-1)%len(smileEmojis)]
	})
	return template.HTML(commentPolicy.Sanitize(withSmiles))
}

func buildCommentTree(comments []*model.Comment) []*commentView {
	views := make(map[int64]*commentView, len(comments))
	for _, c := range comments {
		views[c.ID] = &commentView{
			Comment: c,
			HTML:    renderComment(c.Content),
			Initial: initialOf(c.Author),
			Color:   colorFor(c.Author),
		}
	}

	ordered := append([]*model.Comment(nil), comments...)
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].CreatedAt.Before(ordered[j].CreatedAt)
	})

	var roots []*commentView
	for _, c := range ordered {
		v := views[c.ID]
		if c.ParentID != nil {
			if parent := views[*c.ParentID]; parent != nil && parent.Depth < 5 {
				v.Depth = parent.Depth + 1
				parent.Children = append(parent.Children, v)
				continue
			}
		}
		roots = append(roots, v)
	}
	return roots
}

var avatarPalette = []string{"#8c5a3c", "#2f6f4f", "#3d5a80", "#7a4f8a", "#9a6a1e", "#3f6f7a"}

func colorFor(name string) string {
	var sum int
	for _, r := range name {
		sum = (sum*31 + int(r)) % 100000
	}
	return avatarPalette[sum%len(avatarPalette)]
}

func initialOf(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "?"
	}
	r, _ := utf8.DecodeRuneInString(name)
	return strings.ToUpper(string(r))
}

func csrfToken(ctx arry.Context) string {
	return middlewares.CSRFToken(ctx)
}

func PostComment(deps Deps, guard *PostGuard) arry.Handler {
	return func(ctx arry.Context) {
		slug := ctx.Param("slug")

		post, err := deps.Posts.GetPost(slug)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				notFound(ctx, deps, "文章不存在")
				return
			}
			slog.Error("comment: load post", "slug", slug, "err", err)
			ctx.Reply(http.StatusInternalServerError)
			return
		}
		if !post.PubliclyVisible(time.Now()) {
			notFound(ctx, deps, "文章不存在")
			return
		}
		hidden := HiddenTags(post)
		if post.IsLocked() && !ctx.IsAuthed() && !guard.Allowed(ctx, post) {
			ctx.Reply(http.StatusForbidden)
			return
		}
		if len(hidden) > 0 && !ctx.IsAuthed() && !guard.AllowedAnyTag(ctx, hidden) {
			ctx.Reply(http.StatusForbidden)
			return
		}
		if !post.AllowComment || !commentsEnabled(deps) {
			ctx.Reply(http.StatusForbidden)
			return
		}

		r := ctx.Request()
		if err := r.ParseForm(); err != nil {
			slog.Warn("comment parse form", "err", err)
			renderArticle(ctx, deps, guard, slug, "表单内容无法解析。", nil)
			return
		}

		form := &commentForm{
			Author:  strings.TrimSpace(r.PostFormValue("author")),
			Email:   strings.TrimSpace(r.PostFormValue("email")),
			URL:     strings.TrimSpace(r.PostFormValue("url")),
			Content: strings.TrimSpace(r.PostFormValue("content")),
			Parent:  parseInt64(r.PostFormValue("parent_id")),
			Started: parseInt64(r.PostFormValue("started")),
		}

		// Honeypot: silently accept and drop obvious bots.
		if r.PostFormValue("_hp") != "" {
			ctx.Redirect(http.StatusSeeOther, "/article/"+slug+"?comment=ok#comments")
			return
		}

		now := time.Now().Unix()
		if form.Started == 0 || now-form.Started < 2 || now-form.Started > 2*60*60 {
			renderArticle(ctx, deps, guard, slug, "提交过快或页面已过期，请重新提交。", form)
			return
		}
		if form.Author == "" || utf8.RuneCountInString(form.Author) > 50 {
			renderArticle(ctx, deps, guard, slug, "昵称不能为空且不能超过 50 个字符。", form)
			return
		}
		if form.Content == "" || utf8.RuneCountInString(form.Content) > 5000 {
			renderArticle(ctx, deps, guard, slug, "评论内容不能为空且不能超过 5000 个字符。", form)
			return
		}
		if form.Email != "" && !validCommentEmail(form.Email) {
			renderArticle(ctx, deps, guard, slug, "邮箱格式不正确。", form)
			return
		}
		if form.URL != "" && !strings.HasPrefix(form.URL, "http://") && !strings.HasPrefix(form.URL, "https://") {
			form.URL = "https://" + form.URL
		}

		var parentID *int64
		if form.Parent > 0 {
			parent, err := deps.Comments.GetComment(form.Parent)
			if err == nil && parent.PostID == post.ID && parent.Status == model.CommentApproved {
				id := parent.ID
				parentID = &id
			}
		}

		status := model.CommentApproved
		if moderationRequired(deps) {
			status = model.CommentPending
		}

		ip := ctx.ClientIP()
		if !commentLimits.Allow(ip) || !commentGlobalLimits.Allow("global") {
			renderArticle(ctx, deps, guard, slug, "评论太频繁，请稍后再试。", form)
			return
		}
		commentLimits.Record(ip)
		commentGlobalLimits.Record("global")

		comment := &model.Comment{
			PostID:    post.ID,
			ParentID:  parentID,
			Author:    form.Author,
			Email:     form.Email,
			URL:       form.URL,
			Content:   form.Content,
			Status:    status,
			IP:        ctx.ClientIP(),
			UserAgent: r.UserAgent(),
			CreatedAt: time.Now(),
		}
		if err := deps.CommentWrites.CreateComment(comment); err != nil {
			slog.Error("create comment", "post", post.ID, "err", err)
			renderArticle(ctx, deps, guard, slug, "评论保存失败，请稍后再试。", form)
			return
		}

		message := "ok"
		if status == model.CommentPending {
			message = "pending"
		}
		ctx.Redirect(http.StatusSeeOther, "/article/"+slug+"?comment="+message+"#comments")
		deps.Emit(ctx, events.New(events.CommentCreated, events.SourceWeb, events.CommentData{Comment: comment}))
	}
}

func moderationRequired(deps Deps) bool {
	v, err := deps.Options.GetOption("comments.moderation")
	if err != nil {
		return true
	}
	return v == "1"
}

func validCommentEmail(email string) bool {
	if strings.ContainsAny(email, " \t\r\n") {
		return false
	}
	at := strings.LastIndex(email, "@")
	return at > 0 && at < len(email)-1 && strings.Contains(email[at+1:], ".")
}

func parseInt64(raw string) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	return n
}
