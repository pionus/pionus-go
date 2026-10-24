package handler

import (
	"net/http"
	"strings"

	"github.com/pionus/arry"
	"github.com/pionus/pionus-go/internal/store"
)

// Redirects answers unmatched GET requests by looking up legacy Typecho URLs.
// Register it as the last catch-all route.
func Redirects(db store.RedirectStore) arry.Handler {
	return func(ctx arry.Context) {
		requestPath := ctx.Request().URL.Path

		for _, candidate := range redirectCandidates(requestPath) {
			r, err := db.FindRedirect(candidate)
			if err != nil {
				continue
			}
			if r.To == requestPath || r.To == candidate {
				break
			}

			target := r.To
			if q := ctx.Request().URL.RawQuery; q != "" {
				if strings.Contains(target, "?") {
					target += "&" + q
				} else {
					target += "?" + q
				}
			}

			status := r.Status
			switch status {
			case http.StatusMovedPermanently, http.StatusFound, http.StatusPermanentRedirect:
			default:
				status = http.StatusMovedPermanently
			}
			ctx.Redirect(status, target)
			return
		}

		ctx.Reply(http.StatusNotFound)
	}
}

func redirectCandidates(requestPath string) []string {
	seen := make(map[string]bool, 8)
	var out []string

	add := func(p string) {
		if p == "" || seen[p] {
			return
		}
		seen[p] = true
		out = append(out, p)
	}

	add(requestPath)
	if requestPath != "/" {
		if strings.HasSuffix(requestPath, "/") {
			add(strings.TrimSuffix(requestPath, "/"))
		} else {
			add(requestPath + "/")
		}
	}

	if strings.HasPrefix(requestPath, "/index.php") {
		rest := strings.TrimPrefix(requestPath, "/index.php")
		if rest == "" {
			add("/")
		} else {
			add(rest)
			if rest != "/" {
				if strings.HasSuffix(rest, "/") {
					add(strings.TrimSuffix(rest, "/"))
				} else {
					add(rest + "/")
				}
			}
		}
	}

	return out
}
