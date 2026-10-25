package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

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
	return g.sign(fmt.Sprintf("post:%d:%s", post.ID, post.PostPassword))
}

func (g *PostGuard) sign(payload string) string {
	mac := hmac.New(sha256.New, g.secret)
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// PreviewToken returns a time-limited token that lets an anonymous visitor
// read a draft. Editing the post (updated_at changes) invalidates it.
func (g *PostGuard) PreviewToken(post *model.Post, ttl time.Duration) string {
	expires := time.Now().Add(ttl).Unix()
	payload := g.previewPayload(post, expires)
	return strconv.FormatInt(expires, 10) + "." + g.sign(payload)
}

// ValidPreview reports whether the request carries a valid preview token.
func (g *PostGuard) ValidPreview(ctx arry.Context, post *model.Post) bool {
	token := strings.TrimSpace(ctx.Query("preview"))
	if token == "" {
		return false
	}
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return false
	}
	expires, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || time.Now().Unix() > expires {
		return false
	}
	return hmac.Equal([]byte(parts[1]), []byte(g.sign(g.previewPayload(post, expires))))
}

func (g *PostGuard) previewPayload(post *model.Post, expires int64) string {
	return fmt.Sprintf("preview:%d:%d:%d", post.ID, post.UpdatedAt.UnixNano(), expires)
}
