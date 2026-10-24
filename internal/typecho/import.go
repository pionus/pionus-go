package typecho

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pionus/pionus-go/internal/content"
	"github.com/pionus/pionus-go/internal/model"
	"github.com/pionus/pionus-go/internal/store"
)

// Source marks rows imported from Typecho.
const Source = "typecho"

// Options configures an import.
type Options struct {
	Store store.Store
	// SourceUploads is the old usr/uploads directory (optional).
	SourceUploads string
	// DestUploads is the new upload directory (optional).
	DestUploads string
	// DryRun only reports what would happen.
	DryRun bool
	// ImportDrafts also imports typecho post_draft rows (autosave revisions).
	ImportDrafts bool
	// PageOverrides replaces page bodies by slug (content that lived in old
	// PHP theme templates).
	PageOverrides map[string]string
	Logger        *slog.Logger
}

// Report summarizes an import.
type Report struct {
	Users              int      `json:"users"`
	Categories         int      `json:"categories"`
	Tags               int      `json:"tags"`
	Posts              int      `json:"posts"`
	Pages              int      `json:"pages"`
	Comments           int      `json:"comments"`
	Notes              int      `json:"notes"`
	Attachments        int      `json:"attachments"`
	Updated            int      `json:"updated"`
	SkippedDrafts      int      `json:"skipped_drafts"`
	SkippedAttachments int      `json:"skipped_attachments"`
	NeedsPasswordReset bool     `json:"needs_password_reset,omitempty"`
	PagesOverridden    int      `json:"pages_overridden,omitempty"`
	MissingPages       []string `json:"missing_pages,omitempty"`
	MissingUploads     []string `json:"missing_uploads,omitempty"`
	Warnings           []string `json:"warnings,omitempty"`
}

type importer struct {
	ex         Export
	opts       Options
	log        *slog.Logger
	report     *Report
	userIDs    map[int64]int64
	catIDs     map[int64]int64
	tagIDs     map[int64]int64
	postIDs    map[int64]int64
	commentIDs map[int64]int64

	postCats map[int64][]int64
	postTags map[int64][]int64

	slugByCID map[int64]string
	linker    *LinkFixer
}

// Import reads a Typecho export and writes it to the configured store.
func Import(reader io.Reader, opts Options) (*Report, error) {
	var ex Export
	if err := json.NewDecoder(reader).Decode(&ex); err != nil {
		return nil, fmt.Errorf("decode typecho export: %w", err)
	}
	if opts.Store == nil {
		return nil, fmt.Errorf("typecho: store is required")
	}

	log := opts.Logger
	if log == nil {
		log = slog.Default()
	}

	im := &importer{
		ex:         ex,
		opts:       opts,
		log:        log,
		report:     &Report{},
		userIDs:    make(map[int64]int64),
		catIDs:     make(map[int64]int64),
		tagIDs:     make(map[int64]int64),
		postIDs:    make(map[int64]int64),
		commentIDs: make(map[int64]int64),
		postCats:   make(map[int64][]int64),
		postTags:   make(map[int64][]int64),
		slugByCID:  make(map[int64]string),
		linker:     NewLinkFixer(ex.Site.URL),
	}

	if opts.DryRun {
		return im.dryRun()
	}

	if err := im.importUsers(); err != nil {
		return nil, err
	}
	if err := im.importMetas(); err != nil {
		return nil, err
	}
	im.buildRelations()
	if err := im.prepareSlugs(); err != nil {
		return nil, err
	}
	if err := im.importContents(); err != nil {
		return nil, err
	}
	if err := im.importComments(); err != nil {
		return nil, err
	}
	if err := im.importNotes(); err != nil {
		return nil, err
	}
	if len(im.opts.PageOverrides) > 0 {
		pages, err := ApplyPageOverrides(im.opts.Store, im.opts.PageOverrides, false)
		if err != nil {
			return nil, err
		}
		im.report.PagesOverridden = len(pages.Applied)
		im.report.MissingPages = pages.Missing
	}
	if err := im.applySiteOptions(); err != nil {
		return nil, err
	}

	if len(im.report.MissingUploads) > 0 {
		im.report.Warnings = append(im.report.Warnings,
			"some Typecho attachments are missing on disk; copy usr/uploads before running with -typecho-uploads")
	}
	if im.report.NeedsPasswordReset {
		im.report.Warnings = append(im.report.Warnings,
			"imported users have no password; reset with: pionus -create-admin <username> -admin-password <password>")
	}

	return im.report, nil
}

func (im *importer) dryRun() (*Report, error) {
	im.report.Users = len(im.ex.Users)
	im.report.NeedsPasswordReset = len(im.ex.Users) > 0
	for _, m := range im.ex.Metas {
		switch m.Type {
		case "category":
			im.report.Categories++
		case "tag":
			im.report.Tags++
		}
	}
	for _, c := range im.ex.Contents {
		switch c.Type {
		case "post":
			im.report.Posts++
		case "page":
			im.report.Pages++
		case "post_draft":
			im.report.SkippedDrafts++
		case "attachment":
			im.checkAttachment(c)
		}
	}
	im.report.Comments = len(im.ex.Comments)
	im.report.Notes = len(im.ex.Dreams)
	if len(im.opts.PageOverrides) > 0 {
		pages, err := ApplyPageOverrides(im.opts.Store, im.opts.PageOverrides, true)
		if err != nil {
			return nil, err
		}
		im.report.PagesOverridden = len(pages.Applied)
		im.report.MissingPages = pages.Missing
	}
	im.report.Warnings = append(im.report.Warnings, "dry run: nothing was written")
	return im.report, nil
}

func (im *importer) importUsers() error {
	for _, u := range im.ex.Users {
		username := strings.TrimSpace(u.Name)
		if username == "" {
			continue
		}

		role := roleForGroup(u.Group)
		email := strings.TrimSpace(u.Mail)
		existing, err := im.opts.Store.GetUserByUsername(username)
		if err == nil {
			existing.ScreenName = u.ScreenName
			existing.URL = u.URL
			existing.Role = role
			if email != "" {
				existing.Email = email
			}
			if err := im.opts.Store.UpdateUser(existing); err != nil {
				im.log.Warn("update typecho user", "username", username, "err", err)
			}
			im.userIDs[u.UID] = existing.ID
			im.report.Users++
			if existing.PasswordHash == "" {
				im.report.NeedsPasswordReset = true
			}
			continue
		}
		if !errors.Is(err, store.ErrNotFound) {
			return fmt.Errorf("lookup user %q: %w", username, err)
		}

		if email == "" {
			email = fmt.Sprintf("%s@typecho.local", username)
		}
		user := &model.User{
			Username:   username,
			Email:      email,
			ScreenName: u.ScreenName,
			URL:        u.URL,
			Role:       role,
		}
		if user.ScreenName == "" {
			user.ScreenName = username
		}
		if err := im.opts.Store.CreateUser(user); err != nil {
			// Retry once with a unique fallback address.
			user.Email = fmt.Sprintf("%s+%d@typecho.local", username, u.UID)
			if err2 := im.opts.Store.CreateUser(user); err2 != nil {
				return fmt.Errorf("create user %q: %w", username, err2)
			}
		}
		im.userIDs[u.UID] = user.ID
		im.report.Users++
		im.report.NeedsPasswordReset = true
	}
	return nil
}

func roleForGroup(group string) string {
	switch strings.ToLower(group) {
	case "administrator":
		return model.RoleOwner
	case "editor":
		return model.RoleEditor
	case "contributor":
		return model.RoleContributor
	default:
		return model.RoleContributor
	}
}

func (im *importer) importMetas() error {
	for _, m := range im.ex.Metas {
		name := strings.TrimSpace(m.Name)
		if name == "" {
			continue
		}
		slug := strings.TrimSpace(m.Slug)
		if slug == "" {
			slug = model.Slugify(name)
		}

		switch m.Type {
		case "category":
			c, err := im.opts.Store.GetCategory(slug)
			if err == nil {
				im.catIDs[m.MID] = c.ID
				continue
			}
			if !errors.Is(err, store.ErrNotFound) {
				return fmt.Errorf("lookup category %q: %w", slug, err)
			}
			c = &model.Category{
				Name:        name,
				Slug:        slug,
				Description: m.Description,
				SortOrder:   m.Order,
			}
			if err := im.opts.Store.CreateCategory(c); err != nil {
				return fmt.Errorf("create category %q: %w", slug, err)
			}
			im.catIDs[m.MID] = c.ID
			im.report.Categories++
		case "tag":
			t, err := im.opts.Store.UpsertTag(name, slug)
			if err != nil {
				return fmt.Errorf("upsert tag %q: %w", slug, err)
			}
			im.tagIDs[m.MID] = t.ID
			im.report.Tags++
		}
	}

	// Second pass for nested categories.
	for _, m := range im.ex.Metas {
		if m.Type != "category" || m.Parent == 0 {
			continue
		}
		parentID, ok := im.catIDs[m.Parent]
		if !ok {
			continue
		}
		slug := m.Slug
		if slug == "" {
			slug = model.Slugify(m.Name)
		}
		c, err := im.opts.Store.GetCategory(slug)
		if err != nil {
			continue
		}
		c.ParentID = &parentID
		if err := im.opts.Store.UpdateCategory(slug, c); err != nil {
			return fmt.Errorf("set category parent %q: %w", slug, err)
		}
	}
	return nil
}

func (im *importer) buildRelations() {
	for _, r := range im.ex.Relationships {
		if id, ok := im.catIDs[r.MID]; ok {
			im.postCats[r.CID] = appendUnique(im.postCats[r.CID], id)
		}
		if id, ok := im.tagIDs[r.MID]; ok {
			im.postTags[r.CID] = appendUnique(im.postTags[r.CID], id)
		}
	}
}

func (im *importer) importContents() error {
	contents := append([]Content(nil), im.ex.Contents...)
	sort.Slice(contents, func(i, j int) bool { return contents[i].CID < contents[j].CID })

	for _, c := range contents {
		switch c.Type {
		case "attachment":
			im.importAttachment(c)
			continue
		case "post", "page":
		case "post_draft":
			im.report.SkippedDrafts++
			if !im.opts.ImportDrafts {
				continue
			}
		default:
			continue
		}
		if err := im.importContent(c); err != nil {
			return err
		}
	}
	return nil
}

func (im *importer) importContent(c Content) error {
	format, text := parseContentText(c.Text)
	text = im.linker.Rewrite(text)
	status := mapStatus(c.Status)
	postType := model.PostTypePost
	if c.Type == model.PostTypePage {
		postType = model.PostTypePage
	}
	if c.Type == "post_draft" {
		status = model.StatusDraft
	}

	created := im.wallTime(c.Created)
	updated := created
	if c.Modified > 0 {
		updated = im.wallTime(c.Modified)
	}

	sourceID := strconv.FormatInt(c.CID, 10)
	existing, err := im.opts.Store.GetPostBySource(Source, sourceID)
	slug := im.slugByCID[c.CID]
	if slug == "" {
		slug = im.uniqueSlug(c)
	}
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return fmt.Errorf("lookup typecho post %d: %w", c.CID, err)
	}

	p := &model.Post{
		Slug:         slug,
		Title:        c.Title,
		Content:      text,
		Excerpt:      model.MakeExcerpt(text, 200),
		Type:         postType,
		Status:       status,
		Format:       format,
		Template:     c.Template,
		PostPassword: c.Password,
		AllowComment: allowComment(c.AllowComment),
		Views:        c.Views,
		Likes:        c.Likes,
		CreatedAt:    created,
		UpdatedAt:    updated,
		CategoryIDs:  im.postCats[c.CID],
		TagIDs:       im.postTags[c.CID],
		Source:       Source,
		SourceID:     sourceID,
	}
	if p.CategoryIDs == nil {
		p.CategoryIDs = []int64{}
	}
	if p.TagIDs == nil {
		p.TagIDs = []int64{}
	}
	if id, ok := im.userIDs[c.AuthorID]; ok {
		p.AuthorID = &id
	}
	if status == model.StatusPublish || status == model.StatusHidden {
		t := created
		p.PublishedAt = &t
	}

	if existing != nil {
		p.ID = existing.ID
		if err := im.opts.Store.UpdatePost(existing.Slug, p); err != nil {
			return fmt.Errorf("update typecho post %d: %w", c.CID, err)
		}
		im.report.Updated++
	} else {
		if err := im.opts.Store.CreatePost(p); err != nil {
			return fmt.Errorf("create typecho post %d: %w", c.CID, err)
		}
		if postType == model.PostTypePage {
			im.report.Pages++
		} else {
			im.report.Posts++
		}
	}
	im.postIDs[c.CID] = p.ID
	return nil
}

// wallTime converts a Typecho unix timestamp to the configured site timezone.
// main sets time.Local from config, so display matches the old blog.
func (im *importer) wallTime(ts int64) time.Time {
	if ts == 0 {
		return time.Now()
	}
	return time.Unix(ts, 0).In(time.Local)
}

func (im *importer) uniqueSlug(c Content) string {
	base := strings.TrimSpace(c.Slug)
	if base != "" && !content.ValidSlug(base) {
		base = model.Slugify(base)
	}
	if base == "" {
		if c.Type == "post" || c.Type == "post_draft" {
			base = fmt.Sprintf("archives-%d", c.CID)
		} else {
			base = fmt.Sprintf("page-%d", c.CID)
		}
	}

	candidate := base
	if _, err := im.opts.Store.GetPost(candidate); err == nil {
		candidate = fmt.Sprintf("%s-%d", base, c.CID)
	}
	for i := 2; ; i++ {
		if _, err := im.opts.Store.GetPost(candidate); err != nil {
			return candidate
		}
		candidate = fmt.Sprintf("%s-%d", base, i)
	}
}

func (im *importer) importComments() error {
	comments := append([]Comment(nil), im.ex.Comments...)
	sort.Slice(comments, func(i, j int) bool { return comments[i].Coid < comments[j].Coid })

	pending := comments
	for pass := 0; len(pending) > 0 && pass <= len(comments); pass++ {
		var deferred []Comment
		progress := false

		for _, c := range pending {
			if c.Parent != 0 {
				if _, ok := im.commentIDs[c.Parent]; !ok {
					deferred = append(deferred, c)
					continue
				}
			}
			if err := im.importComment(c); err != nil {
				return err
			}
			progress = true
		}
		if len(deferred) == 0 {
			break
		}
		if !progress {
			for _, c := range deferred {
				if err := im.importComment(c); err != nil {
					return err
				}
			}
			break
		}
		pending = deferred
	}
	return nil
}

func (im *importer) importComment(c Comment) error {
	sourceID := strconv.FormatInt(c.Coid, 10)
	if existing, err := im.opts.Store.GetCommentBySource(Source, sourceID); err == nil {
		im.commentIDs[c.Coid] = existing.ID
		return nil
	} else if !errors.Is(err, store.ErrNotFound) {
		return fmt.Errorf("lookup typecho comment %d: %w", c.Coid, err)
	}

	postID, ok := im.postIDs[c.CID]
	if !ok {
		im.report.Warnings = append(im.report.Warnings,
			fmt.Sprintf("skipped comment %d: post %d was not imported", c.Coid, c.CID))
		return nil
	}

	comment := &model.Comment{
		PostID:    postID,
		Author:    c.Author,
		Email:     c.Mail,
		URL:       c.URL,
		Content:   im.linker.Rewrite(c.Text),
		Status:    mapCommentStatus(c.Status),
		IP:        c.IP,
		UserAgent: c.Agent,
		CreatedAt: im.wallTime(c.Created),
		Source:    Source,
		SourceID:  sourceID,
	}
	if c.Parent != 0 {
		if parentID, ok := im.commentIDs[c.Parent]; ok {
			comment.ParentID = &parentID
		}
	}
	if err := im.opts.Store.CreateComment(comment); err != nil {
		return fmt.Errorf("create comment %d: %w", c.Coid, err)
	}
	im.commentIDs[c.Coid] = comment.ID
	im.report.Comments++
	return nil
}

func (im *importer) importNotes() error {
	for _, d := range im.ex.Dreams {
		sourceID := strconv.FormatInt(d.ID, 10)
		if _, err := im.opts.Store.GetNoteBySource("typecho-dreams", sourceID); err == nil {
			continue
		} else if !errors.Is(err, store.ErrNotFound) {
			return fmt.Errorf("lookup note %d: %w", d.ID, err)
		}

		created := time.Now()
		for _, layout := range []string{"2006-01-02 15:04:05", time.RFC3339} {
			if t, err := time.ParseInLocation(layout, strings.TrimSpace(d.Created), time.Local); err == nil {
				created = t
				break
			}
		}
		content := strings.TrimSpace(d.Content)
		if content == "" {
			continue
		}
		note := &model.Note{
			Content:   content,
			Status:    model.StatusPublish,
			CreatedAt: created,
			UpdatedAt: created,
			Source:    "typecho-dreams",
			SourceID:  sourceID,
		}
		if err := im.opts.Store.CreateNote(note); err != nil {
			return fmt.Errorf("create note %d: %w", d.ID, err)
		}
		im.report.Notes++
	}
	return nil
}

func (im *importer) applySiteOptions() error {
	site := im.ex.Site
	values := map[string]string{}
	if site.Title != "" {
		values["site.title"] = site.Title
	}
	if site.Description != "" {
		values["site.description"] = site.Description
	}
	if site.URL != "" {
		values["site.url"] = site.URL
	}
	if site.Keywords != "" {
		values["site.keywords"] = site.Keywords
	}
	if site.PageSize > 0 {
		values["posts.per_page"] = strconv.Itoa(site.PageSize)
	}
	if len(im.ex.Users) > 0 {
		values["site.author"] = im.ex.Users[0].ScreenName
		if values["site.author"] == "" {
			values["site.author"] = im.ex.Users[0].Name
		}
	}
	values["comments.enabled"] = "1"
	if site.CommentsRequireModerate != 0 {
		values["comments.moderation"] = "1"
	} else {
		values["comments.moderation"] = "0"
	}

	for k, v := range values {
		if err := im.opts.Store.SetOption(k, v); err != nil {
			return fmt.Errorf("set option %s: %w", k, err)
		}
	}
	return nil
}

var (
	attachPathRe = regexp.MustCompile(`s:4:"path";s:\d+:"([^"]+)"`)
	attachNameRe = regexp.MustCompile(`s:4:"name";s:\d+:"([^"]+)"`)
	attachMimeRe = regexp.MustCompile(`s:4:"mime";s:\d+:"([^"]+)"`)
	attachSizeRe = regexp.MustCompile(`s:4:"size";i:(\d+);`)
)

func (im *importer) checkAttachment(c Content) {
	meta := parseAttachment(c.Text)
	rel := strings.TrimPrefix(strings.TrimPrefix(meta.path, "/usr/uploads/"), "/")
	if rel == "" || strings.Contains(rel, "..") {
		im.report.SkippedAttachments++
		return
	}
	if im.opts.SourceUploads == "" {
		im.report.MissingUploads = append(im.report.MissingUploads, rel)
		im.report.SkippedAttachments++
		return
	}
	if _, err := os.Stat(filepath.Join(im.opts.SourceUploads, filepath.FromSlash(rel))); err != nil {
		im.report.MissingUploads = append(im.report.MissingUploads, rel)
		im.report.SkippedAttachments++
		return
	}
	im.report.Attachments++
}

func (im *importer) importAttachment(c Content) {
	meta := parseAttachment(c.Text)
	if meta.path == "" {
		im.report.SkippedAttachments++
		return
	}
	rel := strings.TrimPrefix(meta.path, "/usr/uploads/")
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" || strings.Contains(rel, "..") {
		im.report.SkippedAttachments++
		return
	}

	if im.opts.SourceUploads == "" || im.opts.DestUploads == "" {
		im.report.MissingUploads = append(im.report.MissingUploads, rel)
		im.report.SkippedAttachments++
		return
	}

	src := filepath.Join(im.opts.SourceUploads, filepath.FromSlash(rel))
	if _, err := os.Stat(src); err != nil {
		im.report.MissingUploads = append(im.report.MissingUploads, rel)
		im.report.SkippedAttachments++
		return
	}

	dst := filepath.Join(im.opts.DestUploads, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		im.log.Warn("create attachment dir", "path", dst, "err", err)
		im.report.SkippedAttachments++
		return
	}
	if err := copyFile(src, dst); err != nil {
		im.log.Warn("copy attachment", "path", rel, "err", err)
		im.report.SkippedAttachments++
		return
	}

	media := &model.Media{
		Path:     rel,
		Filename: meta.name,
		MIME:     meta.mime,
		Size:     meta.size,
	}
	if media.Filename == "" {
		media.Filename = filepath.Base(rel)
	}
	if err := im.opts.Store.CreateMedia(media); err != nil {
		if !errors.Is(err, store.ErrConflict) {
			im.log.Warn("create media", "path", rel, "err", err)
		}
		im.report.SkippedAttachments++
		return
	}
	im.report.Attachments++
}

type attachmentMeta struct {
	name string
	path string
	mime string
	size int64
}

func parseAttachment(text string) attachmentMeta {
	var meta attachmentMeta
	if m := attachPathRe.FindStringSubmatch(text); len(m) > 1 {
		meta.path = m[1]
	}
	if m := attachNameRe.FindStringSubmatch(text); len(m) > 1 {
		meta.name = m[1]
	}
	if m := attachMimeRe.FindStringSubmatch(text); len(m) > 1 {
		meta.mime = m[1]
	}
	if m := attachSizeRe.FindStringSubmatch(text); len(m) > 1 {
		meta.size, _ = strconv.ParseInt(m[1], 10, 64)
	}
	return meta
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

// prepareSlugs resolves every imported post/page slug before writing content so
// old internal links can be rewritten to the new canonical URLs.
func (im *importer) prepareSlugs() error {
	for _, c := range im.ex.Contents {
		if c.Type != "post" && c.Type != "page" && c.Type != "post_draft" {
			continue
		}
		sourceID := strconv.FormatInt(c.CID, 10)
		existing, err := im.opts.Store.GetPostBySource(Source, sourceID)
		if err == nil {
			im.slugByCID[c.CID] = existing.Slug
			if c.Type == "page" {
				im.linker.SetPageSlug(c.Slug, existing.Slug)
			}
			im.linker.SetPostSlug(c.CID, existing.Slug)
			continue
		}
		if !errors.Is(err, store.ErrNotFound) {
			return fmt.Errorf("lookup typecho post %d: %w", c.CID, err)
		}
		slug := im.uniqueSlug(c)
		im.slugByCID[c.CID] = slug
		im.linker.SetPostSlug(c.CID, slug)
		if c.Type == "page" && c.Slug != "" {
			im.linker.SetPageSlug(c.Slug, slug)
		}
	}
	return nil
}

func parseContentText(text string) (format, body string) {
	const marker = "<!--markdown-->"
	if idx := strings.Index(text, marker); idx >= 0 && strings.TrimSpace(text[:idx]) == "" {
		return model.FormatMarkdown, text[idx+len(marker):]
	}
	return model.FormatHTML, text
}

func mapStatus(status string) string {
	switch status {
	case "publish":
		return model.StatusPublish
	case "hidden":
		return model.StatusHidden
	case "waiting":
		return model.StatusWaiting
	case "draft", "post_draft":
		return model.StatusDraft
	default:
		return model.StatusDraft
	}
}

func mapCommentStatus(status string) string {
	switch status {
	case "approved":
		return model.CommentApproved
	case "spam":
		return model.CommentSpam
	case "waiting", "pending":
		return model.CommentPending
	default:
		return model.CommentPending
	}
}

func allowComment(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "":
		return true
	case "0", "false", "off", "no":
		return false
	}
	return true
}

func appendUnique(ids []int64, id int64) []int64 {
	for _, v := range ids {
		if v == id {
			return ids
		}
	}
	return append(ids, id)
}
