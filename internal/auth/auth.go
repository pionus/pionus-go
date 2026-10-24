// Package auth implements password hashing and cookie-backed login sessions.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/pionus/arry"
	"github.com/pionus/pionus-go/internal/model"
	"github.com/pionus/pionus-go/internal/store"
	"golang.org/x/crypto/bcrypt"
)

var usernameRe = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,32}$`)

// ValidUsername reports whether s is an acceptable login name.
func ValidUsername(s string) bool {
	return usernameRe.MatchString(s)
}

// UserKey is the context key holding the signed-in user.
const UserKey = "user"

// AdminBase is the mount point of the admin area.
const AdminBase = "/admin"

// Manager owns password hashing and session cookies.
type Manager struct {
	Store      store.UserStore
	CookieName string
	TTL        time.Duration
	Secure     bool
}

// NewManager creates a session manager.
func NewManager(st store.UserStore, cookieName string, ttl time.Duration, secure bool) *Manager {
	if cookieName == "" {
		cookieName = "pionus_session"
	}
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	return &Manager{Store: st, CookieName: cookieName, TTL: ttl, Secure: secure}
}

// HashPassword returns a bcrypt hash.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

// dummyHash is used when a username does not exist so login timing does not
// reveal which accounts are valid.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("pionus-dummy-password"), bcrypt.DefaultCost)

// CheckPassword verifies a password against a stored hash. When hash is empty
// it still performs a bcrypt comparison before failing.
func CheckPassword(hash, password string) bool {
	if hash == "" {
		bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// Middleware loads a session from the request cookie, when present, and
// attaches the user to the context. It never denies requests.
func (m *Manager) Middleware() arry.Middleware {
	return func(next arry.Handler) arry.Handler {
		return func(ctx arry.Context) {
			if cookie, err := ctx.Request().Cookie(m.CookieName); err == nil && cookie.Value != "" {
				_, user, err := m.Store.GetSession(tokenHash(cookie.Value), time.Now())
				if err == nil {
					ctx.Set(UserKey, user)
					ctx.Set("auth", true)
				} else {
					clearCookie(ctx, m.CookieName, m.Secure)
				}
			}
			next(ctx)
		}
	}
}

// RequireUser redirects anonymous GET requests to the login page and rejects
// other anonymous requests.
func (m *Manager) RequireUser(next arry.Handler) arry.Handler {
	return func(ctx arry.Context) {
		if CurrentUser(ctx) != nil {
			next(ctx)
			return
		}

		if ctx.Request().Method == http.MethodGet || ctx.Request().Method == http.MethodHead {
			nextURL := ctx.Request().URL.RequestURI()
			if !strings.HasPrefix(nextURL, AdminBase+"/") && nextURL != AdminBase {
				nextURL = AdminBase
			}
			ctx.Redirect(http.StatusFound, AdminBase+"/login?next="+url.QueryEscape(nextURL))
			return
		}
		ctx.Reply(http.StatusForbidden)
	}
}

// Login creates a session for the user and sets the session cookie.
func (m *Manager) Login(ctx arry.Context, user *model.User) error {
	token := randomToken(32)
	sess := &model.Session{
		TokenHash: tokenHash(token),
		UserID:    user.ID,
		IP:        ctx.ClientIP(),
		UserAgent: ctx.Request().UserAgent(),
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(m.TTL),
	}
	if err := m.Store.CreateSession(sess); err != nil {
		return err
	}

	ctx.SetCookie(&http.Cookie{
		Name:     m.CookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(m.TTL / time.Second),
		HttpOnly: true,
		Secure:   m.Secure,
		SameSite: http.SameSiteLaxMode,
	})
	ctx.Set(UserKey, user)
	ctx.Set("auth", true)

	_ = m.Store.TouchUserLogin(user.ID, time.Now())
	return nil
}

// Logout deletes the current session and clears the cookie.
func (m *Manager) Logout(ctx arry.Context) error {
	if cookie, err := ctx.Request().Cookie(m.CookieName); err == nil && cookie.Value != "" {
		_ = m.Store.DeleteSession(tokenHash(cookie.Value))
	}
	clearCookie(ctx, m.CookieName, m.Secure)
	ctx.Set(UserKey, nil)
	ctx.Set("auth", false)
	return nil
}

// CurrentUser returns the signed-in user or nil.
func CurrentUser(ctx arry.Context) *model.User {
	user, _ := ctx.Get(UserKey).(*model.User)
	return user
}

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func randomToken(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		panic("pionus: cannot generate session token: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}

func clearCookie(ctx arry.Context, name string, secure bool) {
	ctx.SetCookie(&http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}
