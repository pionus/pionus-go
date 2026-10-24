package model

import "time"

// Media is an uploaded file tracked in the media library.
type Media struct {
	ID        int64     `json:"id"`
	Path      string    `json:"path"`
	Filename  string    `json:"filename"`
	MIME      string    `json:"mime"`
	Size      int64     `json:"size"`
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	CreatedAt time.Time `json:"created_at"`
}

// Session is a server-side login session. TokenHash is the SHA-256 hash of
// the cookie value so a database leak does not expose usable sessions.
type Session struct {
	TokenHash string
	UserID    int64
	IP        string
	UserAgent string
	CreatedAt time.Time
	ExpiresAt time.Time
}
