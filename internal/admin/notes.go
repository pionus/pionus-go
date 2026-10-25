package admin

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/pionus/arry"
	"github.com/pionus/pionus-go/internal/events"
	"github.com/pionus/pionus-go/internal/model"
	"github.com/pionus/pionus-go/internal/store"
)

func (a *Admin) noteList(ctx arry.Context) {
	page, limit := pageOf(ctx, 30)
	notes, total, err := a.store.ListNotes(store.NoteFilter{Page: page, Limit: limit})
	if err != nil {
		logErr("admin list notes", "err", err)
		ctx.Reply(http.StatusInternalServerError)
		return
	}

	totalPages := (total + limit - 1) / limit
	if totalPages < 1 {
		totalPages = 1
	}

	data := a.base(ctx, "微语")
	data["Notes"] = notes
	data["Total"] = total
	data["Page"] = page
	data["TotalPages"] = totalPages
	data["HasPrev"] = page > 1
	data["HasNext"] = page < totalPages
	data["PrevPage"] = page - 1
	data["NextPage"] = page + 1

	switch ctx.Query("error") {
	case "empty":
		data["Error"] = "微语内容不能为空。"
	case "too_long":
		data["Error"] = "微语最多 2000 个字符。"
	}
	a.render(ctx, http.StatusOK, "notes.html", data)
}

func (a *Admin) noteEditPage(ctx arry.Context) {
	id := atoiDefault(ctx.Param("id"), 0)
	if id <= 0 {
		ctx.Reply(http.StatusNotFound)
		return
	}
	note, err := a.store.GetNote(int64(id))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			ctx.Reply(http.StatusNotFound)
			return
		}
		logErr("admin load note", "id", id, "err", err)
		ctx.Reply(http.StatusInternalServerError)
		return
	}
	data := a.base(ctx, "编辑微语")
	data["Note"] = note
	switch ctx.Query("error") {
	case "empty":
		data["Error"] = "内容不能为空。"
	case "too_long":
		data["Error"] = "最多 2000 个字符。"
	case "save":
		data["Error"] = "保存失败，请稍后再试。"
	}
	a.render(ctx, http.StatusOK, "note-edit.html", data)
}

func (a *Admin) noteUpdate(ctx arry.Context) {
	id := atoiDefault(ctx.Param("id"), 0)
	if id <= 0 {
		ctx.Reply(http.StatusNotFound)
		return
	}
	form := ctx.Request()
	content := strings.TrimSpace(form.PostFormValue("content"))
	if content == "" {
		a.redirect(ctx, "/admin/notes/"+strconv.Itoa(id)+"/edit?error=empty")
		return
	}
	if utf8.RuneCountInString(content) > 2000 {
		a.redirect(ctx, "/admin/notes/"+strconv.Itoa(id)+"/edit?error=too_long")
		return
	}
	status := form.PostFormValue("status")
	if status != model.StatusHidden {
		status = model.StatusPublish
	}
	if err := a.store.UpdateNote(int64(id), content, status); err != nil {
		logErr("admin update note", "id", id, "err", err)
		a.redirect(ctx, "/admin/notes/"+strconv.Itoa(id)+"/edit?error=save")
		return
	}
	if note, err := a.store.GetNote(int64(id)); err != nil {
		logErr("admin load updated note", "id", id, "err", err)
	} else {
		a.emit(ctx, events.New(events.NoteUpdated, events.SourceAdmin, events.NoteData{Note: note}))
	}
	a.redirect(ctx, "/admin/notes?saved=1")
}

func (a *Admin) noteCreate(ctx arry.Context) {
	content := strings.TrimSpace(ctx.Request().PostFormValue("content"))
	if content == "" {
		a.redirect(ctx, "/admin/notes?error=empty")
		return
	}
	if utf8.RuneCountInString(content) > 2000 {
		a.redirect(ctx, "/admin/notes?error=too_long")
		return
	}

	note := &model.Note{Content: content, Status: model.StatusPublish}
	if err := a.store.CreateNote(note); err != nil {
		logErr("admin create note", "err", err)
		a.redirect(ctx, "/admin/notes?error=empty")
		return
	}
	a.emit(ctx, events.New(events.NoteCreated, events.SourceAdmin, events.NoteData{Note: note}))
	a.redirect(ctx, "/admin/notes?saved=1")
}

func (a *Admin) noteDelete(ctx arry.Context) {
	id := atoiDefault(ctx.Request().PostFormValue("id"), 0)
	if id <= 0 {
		ctx.Reply(http.StatusBadRequest)
		return
	}
	if err := a.store.DeleteNote(int64(id)); err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			logErr("admin delete note", "id", id, "err", err)
		}
	} else {
		a.emit(ctx, events.New(events.NoteDeleted, events.SourceAdmin, events.DeletedData{ID: int64(id)}))
	}
	a.redirect(ctx, "/admin/notes?deleted=1")
}
