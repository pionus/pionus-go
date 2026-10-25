package handler

import (
	"github.com/pionus/arry"
)

// NotFound renders the themed 404 page for unmatched routes.
func NotFound(deps Deps) arry.Handler {
	return func(ctx arry.Context) {
		notFound(ctx, deps, "页面不存在或已被移动。")
	}
}
