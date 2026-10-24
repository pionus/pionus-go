package auth

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/pionus/arry"
	"github.com/pionus/pionus-go/internal/model"
	"github.com/pionus/pionus-go/internal/store"
)

func TestPasswordAndSession(t *testing.T) {
	db, err := store.NewSQLite(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	hash, err := HashPassword("password123")
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPassword(hash, "password123") || CheckPassword(hash, "nope") {
		t.Fatal("password check failed")
	}

	user := &model.User{Username: "u", Email: "u@example.com", PasswordHash: hash, Role: model.RoleOwner}
	if err := db.CreateUser(user); err != nil {
		t.Fatal(err)
	}

	m := NewManager(db, "sid", time.Hour, false)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	ctx := arry.NewContext(req, rec)
	if err := m.Login(ctx, user); err != nil {
		t.Fatal(err)
	}

	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("login did not set a cookie")
	}

	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.AddCookie(cookies[0])
	ctx2 := arry.NewContext(req2, httptest.NewRecorder())
	called := false
	m.Middleware()(func(c arry.Context) {
		called = true
		if CurrentUser(c) == nil || CurrentUser(c).ID != user.ID {
			t.Fatal("session user missing")
		}
	})(ctx2)
	if !called {
		t.Fatal("middleware did not call next")
	}
}
