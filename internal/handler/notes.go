package handler

import (
	"html/template"
	"log/slog"
	"net/http"

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
func NotesPage(db store.Store) arry.Handler {
	return func(ctx arry.Context) {
		const limit = 30
		page := parsePage(ctx)

		notes, total, err := db.ListNotes(store.NoteFilter{
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

		data := baseData(ctx, db, "微语")
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
