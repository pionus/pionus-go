package model

import "time"

// Note is a short post shown in the 微语 timeline.
type Note struct {
	ID        int64     `json:"id"`
	Content   string    `json:"content"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	Source   string `json:"-"`
	SourceID string `json:"-"`
}
