package admin

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/pionus/arry"
	"github.com/pionus/pionus-go/internal/model"
	"github.com/pionus/pionus-go/internal/store"
)

func (a *Admin) friendList(ctx arry.Context) {
	friends, err := a.store.ListFriendLinks(true)
	if err != nil {
		logErr("admin list friend links", "err", err)
		ctx.Reply(http.StatusInternalServerError)
		return
	}
	data := a.base(ctx, "友链")
	data["Friends"] = friends
	switch ctx.Query("error") {
	case "invalid":
		data["Error"] = "名称和链接不能为空，链接必须是 http/https 地址。"
	case "conflict":
		data["Error"] = "这个链接已经存在。"
	case "save":
		data["Error"] = "保存失败，请稍后再试。"
	}
	a.render(ctx, http.StatusOK, "friends.html", data)
}

func (a *Admin) friendCreate(ctx arry.Context) {
	f, err := friendFromForm(ctx, nil)
	if err != nil {
		a.redirect(ctx, "/admin/friends?error=invalid")
		return
	}
	if err := a.store.CreateFriendLink(f); err != nil {
		if errors.Is(err, store.ErrConflict) {
			a.redirect(ctx, "/admin/friends?error=conflict")
			return
		}
		logErr("admin create friend link", "err", err)
		a.redirect(ctx, "/admin/friends?error=save")
		return
	}
	a.redirect(ctx, "/admin/friends?saved=1")
}

func (a *Admin) friendUpdate(ctx arry.Context) {
	id, err := strconv.ParseInt(strings.TrimSpace(ctx.Request().PostFormValue("id")), 10, 64)
	if err != nil || id <= 0 {
		ctx.Reply(http.StatusBadRequest)
		return
	}
	existing, err := a.store.GetFriendLink(id)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			logErr("admin load friend link", "id", id, "err", err)
		}
		a.redirect(ctx, "/admin/friends?error=save")
		return
	}
	f, err := friendFromForm(ctx, existing)
	if err != nil {
		a.redirect(ctx, "/admin/friends?error=invalid")
		return
	}
	if err := a.store.UpdateFriendLink(f); err != nil {
		if errors.Is(err, store.ErrConflict) {
			a.redirect(ctx, "/admin/friends?error=conflict")
			return
		}
		logErr("admin update friend link", "id", id, "err", err)
		a.redirect(ctx, "/admin/friends?error=save")
		return
	}
	a.redirect(ctx, "/admin/friends?saved=1")
}

func (a *Admin) friendDelete(ctx arry.Context) {
	id, err := strconv.ParseInt(strings.TrimSpace(ctx.Request().PostFormValue("id")), 10, 64)
	if err != nil || id <= 0 {
		ctx.Reply(http.StatusBadRequest)
		return
	}
	if err := a.store.DeleteFriendLink(id); err != nil && !errors.Is(err, store.ErrNotFound) {
		logErr("admin delete friend link", "id", id, "err", err)
	}
	a.redirect(ctx, "/admin/friends?deleted=1")
}

func friendFromForm(ctx arry.Context, existing *model.FriendLink) (*model.FriendLink, error) {
	form := ctx.Request()
	name := strings.TrimSpace(form.PostFormValue("name"))
	rawURL := strings.TrimSpace(form.PostFormValue("url"))
	avatar := strings.TrimSpace(form.PostFormValue("avatar"))
	description := strings.TrimSpace(form.PostFormValue("description"))
	if name == "" || rawURL == "" || !validHTTPURL(rawURL) {
		return nil, errors.New("invalid friend link")
	}
	if avatar != "" && !validHTTPURL(avatar) {
		return nil, errors.New("invalid avatar URL")
	}

	f := &model.FriendLink{
		Name:        name,
		URL:         rawURL,
		Avatar:      avatar,
		Description: description,
		SortOrder:   atoiDefault(form.PostFormValue("sort_order"), 0),
		Visible:     form.PostFormValue("visible") != "",
	}
	if existing != nil {
		f.ID = existing.ID
		f.CreatedAt = existing.CreatedAt
	}
	return f, nil
}

func validHTTPURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}
