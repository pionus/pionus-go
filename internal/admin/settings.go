package admin

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/pionus/arry"
)

var settingKeys = []string{
	"site.title",
	"site.description",
	"site.author",
	"site.url",
	"site.keywords",
	"site.locale",
	"posts.per_page",
	"comments.enabled",
	"comments.moderation",
}

func (a *Admin) settingsPage(ctx arry.Context) {
	options, err := a.store.AllOptions()
	if err != nil {
		logErr("admin load options", "err", err)
		ctx.Reply(http.StatusInternalServerError)
		return
	}

	data := a.base(ctx, "设置")
	data["Options"] = options
	a.render(ctx, http.StatusOK, "settings.html", data)
}

func (a *Admin) settingsSave(ctx arry.Context) {
	form := ctx.Request()

	values := map[string]string{
		"site.title":       strings.TrimSpace(form.PostFormValue("site.title")),
		"site.description": strings.TrimSpace(form.PostFormValue("site.description")),
		"site.author":      strings.TrimSpace(form.PostFormValue("site.author")),
		"site.url":         strings.TrimRight(strings.TrimSpace(form.PostFormValue("site.url")), "/"),
		"site.keywords":    strings.TrimSpace(form.PostFormValue("site.keywords")),
		"site.locale":      strings.TrimSpace(form.PostFormValue("site.locale")),
	}

	perPage := atoiDefault(form.PostFormValue("posts.per_page"), 20)
	if perPage < 1 || perPage > 200 {
		perPage = 20
	}
	values["posts.per_page"] = strconv.Itoa(perPage)

	values["comments.enabled"] = checkboxValue(form.PostFormValue("comments.enabled"))
	values["comments.moderation"] = checkboxValue(form.PostFormValue("comments.moderation"))

	for _, key := range settingKeys {
		if err := a.store.SetOption(key, values[key]); err != nil {
			logErr("admin save option", "key", key, "err", err)
			ctx.Reply(http.StatusInternalServerError)
			return
		}
	}

	a.redirect(ctx, "/admin/settings?saved=1")
}

func checkboxValue(v string) string {
	if v == "" || v == "0" || v == "false" {
		return "0"
	}
	return "1"
}
