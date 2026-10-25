package handler

import (
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/pionus/arry"
	"github.com/pionus/pionus-go/internal/content"
	"github.com/pionus/pionus-go/internal/model"
	"github.com/pionus/pionus-go/internal/store"
)

type noteView struct {
	*model.Note
	HTML template.HTML
}

// NotesPage renders the 微语 timeline.
// NoteDetail renders one 微语 entry at /notes/:id.
func NoteDetail(deps Deps) arry.Handler {
	return func(ctx arry.Context) {
		id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			notFound(ctx, deps, "微语不存在")
			return
		}
		note, err := deps.Notes.GetNote(id)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				notFound(ctx, deps, "微语不存在")
				return
			}
			ctx.Reply(http.StatusInternalServerError)
			return
		}
		if note.Status != model.StatusPublish {
			notFound(ctx, deps, "微语不存在")
			return
		}
		data := baseData(ctx, deps, "微语")
		data["Note"] = note
		data["HTML"] = content.RenderNote(note.Content)
		ctx.Render(http.StatusOK, "note.html", data)
	}
}

func NotesPage(deps Deps) arry.Handler {
	return func(ctx arry.Context) {
		const limit = 30
		page := parsePage(ctx)

		notes, total, err := deps.Notes.ListNotes(store.NoteFilter{
			Statuses: []string{model.StatusPublish},
			Page:     page,
			Limit:    limit,
		})
		if err != nil {
			slog.Error("notes page", "err", err)
			ctx.Reply(http.StatusInternalServerError)
			return
		}

		totalPages := (total + limit - 1) / limit
		if totalPages < 1 {
			totalPages = 1
		}

		views := make([]*noteView, 0, len(notes))
		for _, n := range notes {
			views = append(views, &noteView{Note: n, HTML: content.RenderNote(n.Content)})
		}

		data := baseData(ctx, deps, "微语")
		data["Notes"] = views
		data["Total"] = total
		data["Page"] = page
		data["TotalPages"] = totalPages
		data["HasPrev"] = page > 1
		data["HasNext"] = page < totalPages
		data["PrevURL"] = pageURL("/notes", page-1, nil)
		data["NextURL"] = pageURL("/notes", page+1, nil)
		ctx.Render(http.StatusOK, "notes.html", data)
	}
}
