package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"

	"github.com/pionus/arry"
	"github.com/pionus/pionus-go/internal/model"
)

// PostGuard issues and verifies access cookies for password-protected posts.
type PostGuard struct {
	secret []byte
	secure bool
}

// NewPostGuard creates a guard. secret must be stable across restarts; if it
// changes, existing access cookies become invalid.
func NewPostGuard(secret string, secure bool) *PostGuard {
	return &PostGuard{secret: []byte(secret), secure: secure}
}

// Allowed reports whether the request carries a valid access cookie for post.
func (g *PostGuard) Allowed(ctx arry.Context, post *model.Post) bool {
	if post.PostPassword == "" {
		return true
	}
	cookie, err := ctx.Request().Cookie(g.cookieName(post.ID))
	if err != nil || cookie.Value == "" {
		return false
	}
	return hmac.Equal([]byte(cookie.Value), []byte(g.token(post)))
}

// Grant sets the access cookie for post.
func (g *PostGuard) Grant(ctx arry.Context, post *model.Post) {
	ctx.SetCookie(&http.Cookie{
		Name:     g.cookieName(post.ID),
		Value:    g.token(post),
		Path:     "/",
		MaxAge:   24 * 60 * 60,
		HttpOnly: true,
		Secure:   g.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (g *PostGuard) cookieName(id int64) string {
	return "pionus_post_" + strconv.FormatInt(id, 10)
}

func (g *PostGuard) token(post *model.Post) string {
	mac := hmac.New(sha256.New, g.secret)
	mac.Write([]byte(fmt.Sprintf("post:%d:%s", post.ID, post.PostPassword)))
	return hex.EncodeToString(mac.Sum(nil))
}
