package handler

import (
	"github.com/pionus/arry"
	"github.com/pionus/pionus-go/internal/store"
)

// NotFound renders the themed 404 page for unmatched routes.
func NotFound(db store.Store) arry.Handler {
	return func(ctx arry.Context) {
		notFound(ctx, db, "页面不存在或已被移动。")
	}
}
