package admin

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/pionus/arry"
	"github.com/pionus/pionus-go/internal/model"
	"github.com/pionus/pionus-go/internal/store"
)

func (a *Admin) dashboard(ctx arry.Context) {
	published := a.countPosts(model.StatusPublish)
	drafts := a.countPosts(model.StatusDraft)
	hidden := a.countPosts(model.StatusHidden)
	waiting := a.countPosts(model.StatusWaiting)

	latest, _, err := a.store.ListPosts(store.PostFilter{Page: 1, Limit: 5})
	if err != nil {
		logErr("dashboard latest posts", "err", err)
	}

	pendingComments, pendingTotal, err := a.store.ListComments(store.CommentFilter{
		Status: model.CommentPending,
		Page:   1,
		Limit:  5,
	})
	if err != nil {
		logErr("dashboard pending comments", "err", err)
	}

	_, mediaTotal, err := a.store.ListMedia(1, 1)
	if err != nil {
		logErr("dashboard media", "err", err)
	}

	categories, _ := a.store.ListCategories()
	tags, _ := a.store.ListTags()

	data := a.base(ctx, "仪表盘")
	data["Published"] = published
	data["Drafts"] = drafts
	data["Hidden"] = hidden
	data["Waiting"] = waiting
	data["PendingComments"] = pendingTotal
	data["MediaTotal"] = mediaTotal
	data["CategoryTotal"] = len(categories)
	data["TagTotal"] = len(tags)
	data["LatestPosts"] = latest
	data["Pending"] = pendingComments
	a.render(ctx, http.StatusOK, "dashboard.html", data)
}

func (a *Admin) countPosts(status string) int {
	f := store.PostFilter{Page: 1, Limit: 1}
	if status != "" {
		f.Statuses = []string{status}
	}
	_, total, err := a.store.ListPosts(f)
	if err != nil {
		logErr("count posts", "status", status, "err", err)
		return 0
	}
	return total
}

func (a *Admin) commentList(ctx arry.Context) {
	page, limit := pageOf(ctx, 20)
	status := ctx.Query("status")
	if status == "" {
		status = model.CommentPending
	}
	f := store.CommentFilter{Page: page, Limit: limit}
	if status != "all" {
		if status != model.CommentPending && status != model.CommentApproved && status != model.CommentSpam {
			status = model.CommentPending
		}
		f.Status = status
	}

	comments, total, err := a.store.ListComments(f)
	if err != nil {
		logErr("admin list comments", "err", err)
		ctx.Reply(http.StatusInternalServerError)
		return
	}

	totalPages := (total + limit - 1) / limit
	if totalPages < 1 {
		totalPages = 1
	}

	data := a.base(ctx, "评论")
	data["Comments"] = comments
	data["Total"] = total
	data["Status"] = status
	data["Page"] = page
	data["TotalPages"] = totalPages
	data["HasPrev"] = page > 1
	data["HasNext"] = page < totalPages
	data["PrevPage"] = page - 1
	data["NextPage"] = page + 1
	a.render(ctx, http.StatusOK, "comments.html", data)
}

func (a *Admin) commentStatus(ctx arry.Context) {
	form := ctx.Request()
	id := atoiDefault(form.PostFormValue("id"), 0)
	status := form.PostFormValue("action")

	if id <= 0 {
		ctx.Reply(http.StatusBadRequest)
		return
	}
	if err := a.store.UpdateCommentStatus(int64(id), status); err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			logErr("admin update comment", "id", id, "err", err)
		}
	}
	a.redirect(ctx, commentsURL(form, form.PostFormValue("filter")))
}

func (a *Admin) commentDelete(ctx arry.Context) {
	form := ctx.Request()
	id := atoiDefault(form.PostFormValue("id"), 0)

	if id <= 0 {
		ctx.Reply(http.StatusBadRequest)
		return
	}
	if err := a.store.DeleteComment(int64(id)); err != nil && !errors.Is(err, store.ErrNotFound) {
		logErr("admin delete comment", "id", id, "err", err)
	}
	a.redirect(ctx, commentsURL(form, form.PostFormValue("filter")))
}

func commentsURL(form *http.Request, status string) string {
	page := atoiDefault(form.PostFormValue("page"), 1)
	if page < 1 {
		page = 1
	}
	switch status {
	case model.CommentApproved, model.CommentSpam, "all":
	default:
		status = model.CommentPending
	}
	return fmt.Sprintf("/admin/comments?status=%s&page=%d", status, page)
}
