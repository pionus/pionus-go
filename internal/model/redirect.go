package model

import "time"

// Redirect maps a legacy URL path to its new location, used to keep old
// Typecho permalinks working after migration.
type Redirect struct {
	From      string    `json:"from"`
	To        string    `json:"to"`
	Status    int       `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}
