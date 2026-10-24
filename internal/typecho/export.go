// Package typecho imports data exported from a Typecho blog.
package typecho

// Export is the JSON document produced by tools/typecho-export.php.
type Export struct {
	GeneratedAt   string         `json:"generated_at"`
	Site          Site           `json:"site"`
	Contents      []Content      `json:"contents"`
	Metas         []Meta         `json:"metas"`
	Relationships []Relationship `json:"relationships"`
	Comments      []Comment      `json:"comments"`
	Users         []User         `json:"users"`
	Fields        []Field        `json:"fields"`
	Dreams        []Dream        `json:"dreams"`
	Options       []Option       `json:"options"`
}

// Site contains the Typecho settings needed for migration.
type Site struct {
	URL                     string `json:"url"`
	Title                   string `json:"title"`
	Description             string `json:"description"`
	Keywords                string `json:"keywords"`
	Rewrite                 int    `json:"rewrite"`
	Theme                   string `json:"theme"`
	Timezone                int    `json:"timezone"`
	Lang                    string `json:"lang"`
	PageSize                int    `json:"page_size"`
	PostsListSize           int    `json:"posts_list_size"`
	CommentsListSize        int    `json:"comments_list_size"`
	CommentsRequireModerate int    `json:"comments_require_moderation"`
	CommentsThreaded        int    `json:"comments_threaded"`
	CommentsMaxNesting      int    `json:"comments_max_nesting"`
	CommentsOrder           string `json:"comments_order"`
	CommentsAntiSpam        int    `json:"comments_anti_spam"`
	DefaultAllowComment     int    `json:"default_allow_comment"`
	Generator               string `json:"generator"`
}

// Content is a row from typecho_contents.
type Content struct {
	CID          int64  `json:"cid"`
	Title        string `json:"title"`
	Slug         string `json:"slug"`
	Created      int64  `json:"created"`
	Modified     int64  `json:"modified"`
	Text         string `json:"text"`
	Order        int    `json:"order"`
	AuthorID     int64  `json:"authorId"`
	Template     string `json:"template"`
	Type         string `json:"type"`
	Status       string `json:"status"`
	Password     string `json:"password"`
	CommentsNum  int    `json:"commentsNum"`
	AllowComment string `json:"allowComment"`
	AllowPing    string `json:"allowPing"`
	AllowFeed    string `json:"allowFeed"`
	Parent       int64  `json:"parent"`
	Views        int64  `json:"views"`
	Likes        int64  `json:"likes"`
}

// Meta is a row from typecho_metas (category or tag).
type Meta struct {
	MID         int64  `json:"mid"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Count       int    `json:"count"`
	Order       int    `json:"order"`
	Parent      int64  `json:"parent"`
}

// Relationship links content and metas.
type Relationship struct {
	CID int64 `json:"cid"`
	MID int64 `json:"mid"`
}

// Comment is a row from typecho_comments.
type Comment struct {
	Coid     int64  `json:"coid"`
	CID      int64  `json:"cid"`
	Created  int64  `json:"created"`
	Author   string `json:"author"`
	AuthorID int64  `json:"authorId"`
	OwnerID  int64  `json:"ownerId"`
	Mail     string `json:"mail"`
	URL      string `json:"url"`
	IP       string `json:"ip"`
	Agent    string `json:"agent"`
	Text     string `json:"text"`
	Type     string `json:"type"`
	Status   string `json:"status"`
	Parent   int64  `json:"parent"`
}

// User is a row from typecho_users. Password hashes are not exported.
type User struct {
	UID        int64  `json:"uid"`
	Name       string `json:"name"`
	ScreenName string `json:"screenName"`
	Mail       string `json:"mail"`
	URL        string `json:"url"`
	Group      string `json:"group"`
	Created    int64  `json:"created"`
	Activated  int64  `json:"activated"`
}

// Field is a row from typecho_fields (may be empty).
type Field struct {
	CID   int64  `json:"cid"`
	Name  string `json:"name"`
	Type  string `json:"type"`
	Str   string `json:"str_value"`
	Int   int64  `json:"int_value"`
	Float string `json:"float_value"`
}

// Dream is a short post from the old custom `dreams` table, used here as the
// 微语 timeline source.
type Dream struct {
	ID      int64  `json:"id"`
	Author  string `json:"author"`
	Content string `json:"content"`
	Created string `json:"created"`
}

// Option is a row from typecho_options.
type Option struct {
	Name  string  `json:"name"`
	User  int64   `json:"user"`
	Value *string `json:"value"`
}
