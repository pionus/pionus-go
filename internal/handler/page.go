package handler

import (
	"errors"
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

func Index(db store.Store) arry.Handler {
	return func(ctx arry.Context) {
		now := time.Now()
		posts, _, err := db.ListPosts(store.PostFilter{
			Type:            model.PostTypePost,
			Statuses:        []string{model.StatusPublish},
			PublishedBefore: &now,
			Page:            1,
			Limit:           6,
			WithTaxonomy:    true,
		})
		if err != nil {
			slog.Error("index page", "err", err)
		}

		categories, _ := db.ListCategories()
		tags, _ := db.ListTags()
		sort.Slice(tags, func(i, j int) bool {
			if tags[i].PostCount != tags[j].PostCount {
				return tags[i].PostCount > tags[j].PostCount
			}
			return tags[i].Name < tags[j].Name
		})
		if len(tags) > 24 {
			tags = tags[:24]
		}

		data := baseData(ctx, db, "")
		data["Posts"] = posts
		data["Categories"] = categories
		data["Tags"] = tags
		ctx.Render(http.StatusOK, "home.html", data)
	}
}

func ArticleList(db store.Store) arry.Handler {
	return func(ctx arry.Context) {
		data, err := listData(ctx, db, store.PostFilter{
			Page:  parsePage(ctx),
			Limit: perPage(db, 20),
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

func ArticleDetail(db store.Store) arry.Handler {
	return func(ctx arry.Context) {
		renderArticle(ctx, db, ctx.Param("slug"), "", nil)
	}
}

func CategoryArchive(db store.Store) arry.Handler {
	return func(ctx arry.Context) {
		slug := ctx.Param("slug")
		c, err := db.GetCategory(slug)
		if err != nil {
			notFound(ctx, db, "分类不存在")
			return
		}

		data, err := listData(ctx, db, store.PostFilter{
			Page:         parsePage(ctx),
			Limit:        perPage(db, 20),
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

func TagArchive(db store.Store) arry.Handler {
	return func(ctx arry.Context) {
		slug := ctx.Param("slug")
		t, err := db.GetTag(slug)
		if err != nil {
			notFound(ctx, db, "标签不存在")
			return
		}

		data, err := listData(ctx, db, store.PostFilter{
			Page:    parsePage(ctx),
			Limit:   perPage(db, 20),
			TagSlug: slug,
		}, "标签："+t.Name, "/tag/"+slug, nil, false)
		if err != nil {
			slog.Error("tag page", "slug", slug, "err", err)
			ctx.Reply(http.StatusInternalServerError)
			return
		}
		data["ArchiveTitle"] = "标签：" + t.Name
		data["ArchiveKind"] = "tag"
		data["ArchiveSlug"] = slug
		data["Tag"] = t
		ctx.Render(http.StatusOK, "archive.html", data)
	}
}

func DateArchive(db store.Store) arry.Handler {
	return func(ctx arry.Context) {
		year, err := strconv.Atoi(ctx.Param("year"))
		if err != nil || year < 1970 || year > 9999 {
			notFound(ctx, db, "日期不正确")
			return
		}

		month := 0
		if raw := ctx.Param("month"); raw != "" {
			month, err = strconv.Atoi(raw)
			if err != nil || month < 1 || month > 12 {
				notFound(ctx, db, "日期不正确")
				return
			}
		}
		day := 0
		if raw := ctx.Param("day"); raw != "" {
			day, err = strconv.Atoi(raw)
			if err != nil || day < 1 || day > 31 {
				notFound(ctx, db, "日期不正确")
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

		data, err := listData(ctx, db, store.PostFilter{
			Page:          parsePage(ctx),
			Limit:         perPage(db, 20),
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

func Search(db store.Store) arry.Handler {
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
			data = baseData(ctx, db, title)
			data["Posts"] = []*model.Post{}
			data["Total"] = 0
			data["Page"] = 1
			data["TotalPages"] = 1
			data["ArchiveTitle"] = title
			data["ArchiveKind"] = "search"
			data["SearchQuery"] = ""
			data["NoIndex"] = true
		} else {
			data, err = listData(ctx, db, store.PostFilter{
				Page:  parsePage(ctx),
				Limit: perPage(db, 20),
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

func CategoriesPage(db store.Store) arry.Handler {
	return func(ctx arry.Context) {
		categories, err := db.ListCategories()
		if err != nil {
			slog.Error("categories page", "err", err)
			ctx.Reply(http.StatusInternalServerError)
			return
		}
		data := baseData(ctx, db, "分类")
		data["Categories"] = categories
		ctx.Render(http.StatusOK, "categories.html", data)
	}
}

func TagsPage(db store.Store) arry.Handler {
	return func(ctx arry.Context) {
		tags, err := db.ListTags()
		if err != nil {
			slog.Error("tags page", "err", err)
			ctx.Reply(http.StatusInternalServerError)
			return
		}
		sort.Slice(tags, func(i, j int) bool {
			if tags[i].PostCount != tags[j].PostCount {
				return tags[i].PostCount > tags[j].PostCount
			}
			return tags[i].Name < tags[j].Name
		})
		data := baseData(ctx, db, "标签")
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
func renderArticle(ctx arry.Context, db store.Store, slug, errMsg string, form *commentForm) {
	post, err := db.GetPost(slug)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			notFound(ctx, db, "文章不存在")
			return
		}
		slog.Error("article page", "slug", slug, "err", err)
		ctx.Reply(http.StatusInternalServerError)
		return
	}

	if !post.PubliclyVisible(time.Now()) && !ctx.IsAuthed() {
		notFound(ctx, db, "文章不存在")
		return
	}

	rendered := content.RenderPost(post)
	plain := model.PlainText(post.Content)
	readingTime := (len([]rune(plain)) + 199) / 200
	if readingTime < 1 {
		readingTime = 1
	}

	prev, next, err := db.AdjacentPosts(slug)
	if err != nil {
		slog.Warn("adjacent posts", "slug", slug, "err", err)
	}

	comments, _, err := db.ListComments(store.CommentFilter{
		PostID: post.ID,
		Status: model.CommentApproved,
		Page:   1,
		Limit:  500,
	})
	if err != nil {
		slog.Error("list comments", "post", post.ID, "err", err)
	}

	_ = db.IncrementPostViews(slug)

	canComment := post.AllowComment && commentsEnabled(db)
	if form == nil {
		form = &commentForm{
			Started: time.Now().Unix(),
		}
	}

	data := baseData(ctx, db, post.Title)
	data["CanonicalURL"] = siteBaseURL(ctx, db) + "/article/" + post.Slug
	data["Post"] = post
	data["Rendered"] = rendered
	data["TOC"] = rendered.TOC
	data["ReadingTime"] = readingTime
	data["Prev"] = prev
	data["Next"] = next
	data["Comments"] = buildCommentTree(comments)
	data["CommentCount"] = len(comments)
	data["CanComment"] = canComment
	data["CSRF"] = csrfToken(ctx)
	data["CommentAction"] = "/article/" + slug + "/comment"
	data["CommentError"] = errMsg
	data["CommentMessage"] = commentMessage(ctx.Query("comment"))
	data["Form"] = form
	data["IsPage"] = post.Type == model.PostTypePage

	ctx.Render(http.StatusOK, "post.html", data)
}

func commentsEnabled(db store.Store) bool {
	v, err := db.GetOption("comments.enabled")
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
