package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pionus/arry"
	"github.com/pionus/pionus-go/internal/content"
	"github.com/pionus/pionus-go/internal/model"
	"github.com/pionus/pionus-go/internal/store"
)

// YearGroup holds posts grouped by year for the archive page.
type YearGroup struct {
	Year  int
	Posts []*model.Post
}

func Index(deps Deps) arry.Handler {
	return func(ctx arry.Context) {
		now := time.Now()
		posts, _, err := deps.Posts.ListPosts(store.PostFilter{
			Type:                model.PostTypePost,
			Statuses:            []string{model.StatusPublish},
			PublishedBefore:     &now,
			Page:                1,
			Limit:               6,
			WithTaxonomy:        true,
			ExcludeHiddenTagged: true,
		})
		if err != nil {
			slog.Error("index page", "err", err)
		}

		categories, _ := deps.Taxonomy.ListCategories()
		allTags, _ := deps.Taxonomy.ListTags()
		tags := make([]*model.Tag, 0, len(allTags))
		for _, tag := range allTags {
			if !tag.Hidden {
				tags = append(tags, tag)
			}
		}
		sort.Slice(tags, func(i, j int) bool {
			if tags[i].PostCount != tags[j].PostCount {
				return tags[i].PostCount > tags[j].PostCount
			}
			return tags[i].Name < tags[j].Name
		})
		if len(tags) > 24 {
			tags = tags[:24]
		}

		data := baseData(ctx, deps, "")
		data["Posts"] = posts
		data["Categories"] = categories
		data["Tags"] = tags
		ctx.Render(http.StatusOK, "home.html", data)
	}
}

func ArticleList(deps Deps) arry.Handler {
	return func(ctx arry.Context) {
		data, err := listData(ctx, deps, store.PostFilter{
			Page:  parsePage(ctx),
			Limit: perPage(deps, 20),
		}, "文章归档", "/article", nil, true)
		if err != nil {
			slog.Error("article list page", "err", err)
			ctx.Reply(http.StatusInternalServerError)
			return
		}
		data["ArchiveTitle"] = "全部文章"
		data["ArchiveKind"] = "all"
		ctx.Render(http.StatusOK, "archive.html", data)
	}
}

func ArticleDetail(deps Deps, guard *PostGuard) arry.Handler {
	return func(ctx arry.Context) {
		renderArticle(ctx, deps, guard, ctx.Param("slug"), "", nil)
	}
}

func CategoryArchive(deps Deps) arry.Handler {
	return func(ctx arry.Context) {
		slug := ctx.Param("slug")
		c, err := deps.Taxonomy.GetCategory(slug)
		if err != nil {
			notFound(ctx, deps, "分类不存在")
			return
		}

		data, err := listData(ctx, deps, store.PostFilter{
			Page:         parsePage(ctx),
			Limit:        perPage(deps, 20),
			CategorySlug: slug,
		}, "分类："+c.Name, "/category/"+slug, nil, false)
		if err != nil {
			slog.Error("category page", "slug", slug, "err", err)
			ctx.Reply(http.StatusInternalServerError)
			return
		}
		data["ArchiveTitle"] = "分类：" + c.Name
		data["ArchiveKind"] = "category"
		data["ArchiveSlug"] = slug
		data["Category"] = c
		ctx.Render(http.StatusOK, "archive.html", data)
	}
}

func TagArchive(deps Deps, guard *PostGuard) arry.Handler {
	return func(ctx arry.Context) {
		slug := ctx.Param("slug")
		t, err := deps.Taxonomy.GetTag(slug)
		if err != nil {
			notFound(ctx, deps, "标签不存在")
			return
		}

		if t.Hidden && !ctx.IsAuthed() && !guard.AllowedTag(ctx, t) {
			renderTagPassword(ctx, deps, t, "", http.StatusOK)
			return
		}

		data, err := listData(ctx, deps, store.PostFilter{
			Page:    parsePage(ctx),
			Limit:   perPage(deps, 20),
			TagSlug: slug,
		}, "标签："+t.Name, "/tag/"+slug, nil, false, t.Hidden)
		if err != nil {
			slog.Error("tag page", "slug", slug, "err", err)
			ctx.Reply(http.StatusInternalServerError)
			return
		}
		data["ArchiveTitle"] = "标签：" + t.Name
		data["ArchiveKind"] = "tag"
		data["ArchiveSlug"] = slug
		data["Tag"] = t
		if t.Hidden {
			data["NoIndex"] = true
		}
		ctx.Render(http.StatusOK, "archive.html", data)
	}
}

func DateArchive(deps Deps) arry.Handler {
	return func(ctx arry.Context) {
		year, err := strconv.Atoi(ctx.Param("year"))
		if err != nil || year < 1970 || year > 9999 {
			notFound(ctx, deps, "日期不正确")
			return
		}

		month := 0
		if raw := ctx.Param("month"); raw != "" {
			month, err = strconv.Atoi(raw)
			if err != nil || month < 1 || month > 12 {
				notFound(ctx, deps, "日期不正确")
				return
			}
		}
		day := 0
		if raw := ctx.Param("day"); raw != "" {
			day, err = strconv.Atoi(raw)
			if err != nil || day < 1 || day > 31 {
				notFound(ctx, deps, "日期不正确")
				return
			}
		}

		from := time.Date(year, time.Month(max(month, 1)), max(day, 1), 0, 0, 0, 0, time.Local)
		var to time.Time
		var title, base string
		switch {
		case day > 0:
			to = from.AddDate(0, 0, 1)
			title = from.Format("2006 年 01 月 02 日")
			base = from.Format("/archives/2006/01/02")
		case month > 0:
			to = from.AddDate(0, 1, 0)
			title = from.Format("2006 年 01 月")
			base = from.Format("/archives/2006/01")
		default:
			to = from.AddDate(1, 0, 0)
			title = from.Format("2006 年")
			base = from.Format("/archives/2006")
		}

		data, err := listData(ctx, deps, store.PostFilter{
			Page:          parsePage(ctx),
			Limit:         perPage(deps, 20),
			PublishedFrom: &from,
			PublishedTo:   &to,
		}, title, base, nil, false)
		if err != nil {
			slog.Error("date archive", "year", year, "err", err)
			ctx.Reply(http.StatusInternalServerError)
			return
		}
		data["ArchiveTitle"] = title
		data["ArchiveKind"] = "date"
		ctx.Render(http.StatusOK, "archive.html", data)
	}
}

func Search(deps Deps) arry.Handler {
	return func(ctx arry.Context) {
		query := strings.TrimSpace(ctx.Query("q"))
		extra := url.Values{}
		if query != "" {
			extra.Set("q", query)
		}

		title := "搜索"
		if query != "" {
			title = "搜索：" + query
		}

		var data map[string]any
		var err error
		if query == "" {
			data = baseData(ctx, deps, title)
			data["Posts"] = []*model.Post{}
			data["Total"] = 0
			data["Page"] = 1
			data["TotalPages"] = 1
			data["ArchiveTitle"] = title
			data["ArchiveKind"] = "search"
			data["SearchQuery"] = ""
			data["NoIndex"] = true
		} else {
			data, err = listData(ctx, deps, store.PostFilter{
				Page:  parsePage(ctx),
				Limit: perPage(deps, 20),
				Query: query,
			}, title, "/search", extra, false)
			if err != nil {
				slog.Error("search", "q", query, "err", err)
				ctx.Reply(http.StatusInternalServerError)
				return
			}
			data["ArchiveTitle"] = title
			data["ArchiveKind"] = "search"
			data["NoIndex"] = true
		}
		ctx.Render(http.StatusOK, "archive.html", data)
	}
}

func CategoriesPage(deps Deps) arry.Handler {
	return func(ctx arry.Context) {
		categories, err := deps.Taxonomy.ListCategories()
		if err != nil {
			slog.Error("categories page", "err", err)
			ctx.Reply(http.StatusInternalServerError)
			return
		}
		data := baseData(ctx, deps, "分类")
		data["Categories"] = categories
		ctx.Render(http.StatusOK, "categories.html", data)
	}
}

func TagsPage(deps Deps) arry.Handler {
	return func(ctx arry.Context) {
		allTags, err := deps.Taxonomy.ListTags()
		if err != nil {
			slog.Error("tags page", "err", err)
			ctx.Reply(http.StatusInternalServerError)
			return
		}
		tags := make([]*model.Tag, 0, len(allTags))
		for _, tag := range allTags {
			if !tag.Hidden {
				tags = append(tags, tag)
			}
		}
		sort.Slice(tags, func(i, j int) bool {
			if tags[i].PostCount != tags[j].PostCount {
				return tags[i].PostCount > tags[j].PostCount
			}
			return tags[i].Name < tags[j].Name
		})
		data := baseData(ctx, deps, "标签")
		data["Tags"] = tags
		ctx.Render(http.StatusOK, "tags.html", data)
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// renderArticle loads a post, its comments and renders the detail page.
func renderArticle(ctx arry.Context, deps Deps, guard *PostGuard, slug, errMsg string, form *commentForm) {
	post, err := deps.Posts.GetPost(slug)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			notFound(ctx, deps, "文章不存在")
			return
		}
		slog.Error("article page", "slug", slug, "err", err)
		ctx.Reply(http.StatusInternalServerError)
		return
	}

	now := time.Now()
	preview := guard.ValidPreview(ctx, post)
	hidden := HiddenTags(post)
	if !post.PubliclyVisible(now) && !ctx.IsAuthed() && !preview {
		notFound(ctx, deps, "文章不存在")
		return
	}
	needsPostPassword := post.IsLocked() && !ctx.IsAuthed() && !preview && !guard.Allowed(ctx, post)
	needsTagPassword := len(hidden) > 0 && !ctx.IsAuthed() && !preview && !guard.AllowedAnyTag(ctx, hidden)
	if needsPostPassword || needsTagPassword {
		renderPassword(ctx, deps, post, "", http.StatusOK)
		return
	}

	rendered := content.RenderPostCached(post)
	plain := model.PlainText(post.Content)
	readingTime := (len([]rune(plain)) + 199) / 200
	if readingTime < 1 {
		readingTime = 1
	}

	prev, next, err := deps.Posts.AdjacentPosts(slug)
	if err != nil {
		slog.Warn("adjacent posts", "slug", slug, "err", err)
	}

	comments, _, err := deps.Comments.ListComments(store.CommentFilter{
		PostID: post.ID,
		Status: model.CommentApproved,
		Page:   1,
		Limit:  1000,
	})
	if err != nil {
		slog.Error("list comments", "post", post.ID, "err", err)
	}

	const commentPageSize = 20
	commentPage := 1
	if raw := ctx.Query("cpage"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			commentPage = n
		}
	}
	roots := buildCommentTree(comments)
	commentTotalPages := (len(roots) + commentPageSize - 1) / commentPageSize
	if commentTotalPages < 1 {
		commentTotalPages = 1
	}
	if commentPage > commentTotalPages {
		commentPage = commentTotalPages
	}
	start := (commentPage - 1) * commentPageSize
	end := start + commentPageSize
	if end > len(roots) {
		end = len(roots)
	}
	pageRoots := roots[start:end]

	_ = deps.Views.IncrementPostViews(slug)

	canComment := post.AllowComment && commentsEnabled(deps) && post.PubliclyVisible(now) && !preview
	if form == nil {
		form = &commentForm{
			Started: time.Now().Unix(),
		}
	}

	data := baseData(ctx, deps, post.Title)
	data["CanonicalURL"] = siteBaseURL(ctx, deps) + "/article/" + post.Slug
	data["Post"] = post
	data["Rendered"] = rendered
	data["TOC"] = rendered.TOC
	data["ReadingTime"] = readingTime
	data["Prev"] = prev
	data["Next"] = next
	data["Comments"] = pageRoots
	data["CommentCount"] = len(comments)
	data["CommentPage"] = commentPage
	data["CommentTotalPages"] = commentTotalPages
	data["CommentHasPrev"] = commentPage > 1
	data["CommentHasNext"] = commentPage < commentTotalPages
	data["CommentPrevURL"] = fmt.Sprintf("/article/%s?cpage=%d#comments", slug, commentPage-1)
	data["CommentNextURL"] = fmt.Sprintf("/article/%s?cpage=%d#comments", slug, commentPage+1)
	data["CanComment"] = canComment
	data["CSRF"] = csrfToken(ctx)
	data["CommentAction"] = "/article/" + slug + "/comment"
	data["CommentError"] = errMsg
	data["CommentMessage"] = commentMessage(ctx.Query("comment"))
	data["Form"] = form
	data["IsPage"] = post.Type == model.PostTypePage
	data["Preview"] = preview
	if preview || len(hidden) > 0 {
		data["NoIndex"] = true
	}

	templateName := "post.html"
	if post.Type == model.PostTypePage && post.Slug == "links" {
		if deps.Friends == nil {
			slog.Error("friend links page: store is not configured")
			ctx.Reply(http.StatusInternalServerError)
			return
		}
		friends, err := deps.Friends.ListFriendLinks(false)
		if err != nil {
			slog.Error("friend links page", "err", err)
			ctx.Reply(http.StatusInternalServerError)
			return
		}
		data["FriendLinks"] = friends
		templateName = "links.html"
	}

	ctx.Render(http.StatusOK, templateName, data)
}

func commentsEnabled(deps Deps) bool {
	v, err := deps.Options.GetOption("comments.enabled")
	if err != nil {
		return true
	}
	return v != "0"
}

func commentMessage(status string) string {
	switch status {
	case "pending":
		return "评论已提交，审核通过后会显示。"
	case "ok":
		return "评论已发布。"
	}
	return ""
}
