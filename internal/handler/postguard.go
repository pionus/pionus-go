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

// AllowedTag reports whether the request carries a valid access cookie for a
// hidden tag.
func (g *PostGuard) AllowedTag(ctx arry.Context, tag *model.Tag) bool {
	if tag == nil || !tag.Hidden || tag.PasswordHash == "" {
		return false
	}
	cookie, err := ctx.Request().Cookie(g.tagCookieName(tag.ID))
	if err != nil || cookie.Value == "" {
		return false
	}
	return hmac.Equal([]byte(cookie.Value), []byte(g.tagToken(tag)))
}

// GrantTag sets the access cookie for a hidden tag.
func (g *PostGuard) GrantTag(ctx arry.Context, tag *model.Tag) {
	ctx.SetCookie(&http.Cookie{
		Name:     g.tagCookieName(tag.ID),
		Value:    g.tagToken(tag),
		Path:     "/",
		MaxAge:   24 * 60 * 60,
		HttpOnly: true,
		Secure:   g.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// AllowedAnyTag reports whether any hidden tag on a post is unlocked.
func (g *PostGuard) AllowedAnyTag(ctx arry.Context, tags []*model.Tag) bool {
	for _, tag := range tags {
		if g.AllowedTag(ctx, tag) {
			return true
		}
	}
	return false
}

func (g *PostGuard) cookieName(id int64) string {
	return "pionus_post_" + strconv.FormatInt(id, 10)
}

func (g *PostGuard) tagCookieName(id int64) string {
	return "pionus_tag_" + strconv.FormatInt(id, 10)
}

func (g *PostGuard) tagToken(tag *model.Tag) string {
	return g.sign(fmt.Sprintf("tag:%d:%s", tag.ID, tag.PasswordHash))
}

// HiddenTags returns the hidden tags attached to a post.
func HiddenTags(post *model.Post) []*model.Tag {
	if post == nil {
		return nil
	}
	var tags []*model.Tag
	for _, tag := range post.Tags {
		if tag.Hidden {
			tags = append(tags, tag)
		}
	}
	return tags
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
