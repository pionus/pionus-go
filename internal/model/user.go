package model

import "time"

// User roles, ordered from most to least privileged.
const (
	RoleOwner       = "owner"
	RoleAdmin       = "admin"
	RoleEditor      = "editor"
	RoleContributor = "contributor"
)

// User is an account that can sign in to the admin area.
type User struct {
	ID           int64      `json:"id"`
	Username     string     `json:"username"`
	Email        string     `json:"email"`
	PasswordHash string     `json:"-"`
	ScreenName   string     `json:"screen_name"`
	URL          string     `json:"url"`
	Role         string     `json:"role"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	LastLoginAt  *time.Time `json:"last_login_at,omitempty"`
}

// DisplayName returns the name shown next to posts.
func (u *User) DisplayName() string {
	if u.ScreenName != "" {
		return u.ScreenName
	}
	return u.Username
}

// CanEdit reports whether the user may create or change content.
func (u *User) CanEdit() bool {
	switch u.Role {
	case RoleOwner, RoleAdmin, RoleEditor, RoleContributor:
		return true
	}
	return false
}

// CanManage reports whether the user may manage site settings and users.
func (u *User) CanManage() bool {
	return u.Role == RoleOwner || u.Role == RoleAdmin
}
