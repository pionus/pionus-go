package typecho

import (
	"fmt"
	"strconv"

	"github.com/pionus/pionus-go/internal/model"
	"github.com/pionus/pionus-go/internal/store"
)

// FixReport summarizes a link repair pass.
type FixReport struct {
	PostsScanned      int      `json:"posts_scanned"`
	PostsUpdated      int      `json:"posts_updated"`
	CommentsScanned   int      `json:"comments_scanned"`
	CommentsUpdated   int      `json:"comments_updated"`
	RemainingOldLinks []string `json:"remaining_old_links,omitempty"`
	DryRun            bool     `json:"dry_run,omitempty"`
}

// FixLinks rewrites old Typecho links in every post and comment. It is
// idempotent and can be run after an import or on its own.
func FixLinks(db store.Store, dryRun bool) (*FixReport, error) {
	report := &FixReport{DryRun: dryRun}

	siteURL, _ := db.GetOption("site.url")
	linker := NewLinkFixer(siteURL)

	// Build the cid -> new slug map from imported posts.
	page := 1
	for {
		posts, total, err := db.ListPosts(store.PostFilter{Page: page, Limit: 200})
		if err != nil {
			return nil, fmt.Errorf("list posts for link map: %w", err)
		}
		for _, p := range posts {
			if p.Source != Source || p.SourceID == "" {
				continue
			}
			cid, err := strconv.ParseInt(p.SourceID, 10, 64)
			if err != nil {
				continue
			}
			linker.SetPostSlug(cid, p.Slug)
			if p.Type == model.PostTypePage {
				linker.SetPageSlug(p.Slug, p.Slug)
			}
		}
		if page*200 >= total || len(posts) == 0 {
			break
		}
		page++
	}

	// Rewrite post bodies.
	page = 1
	for {
		posts, total, err := db.ListPosts(store.PostFilter{Page: page, Limit: 200, WithContent: true})
		if err != nil {
			return nil, fmt.Errorf("list posts for link repair: %w", err)
		}
		for _, p := range posts {
			report.PostsScanned++
			fixed := linker.Rewrite(p.Content)
			if fixed != p.Content {
				report.PostsUpdated++
				if !dryRun {
					if err := db.UpdatePostContent(p.ID, fixed, model.MakeExcerpt(fixed, 200)); err != nil {
						return nil, err
					}
				}
			}
			for _, old := range linker.FindOldLinks(fixed) {
				report.RemainingOldLinks = append(report.RemainingOldLinks,
					fmt.Sprintf("post:%s:%s", p.Slug, old))
			}
		}
		if page*200 >= total || len(posts) == 0 {
			break
		}
		page++
	}

	// Rewrite comments.
	page = 1
	for {
		comments, total, err := db.ListComments(store.CommentFilter{Page: page, Limit: 200})
		if err != nil {
			return nil, fmt.Errorf("list comments for link repair: %w", err)
		}
		for _, c := range comments {
			report.CommentsScanned++
			fixed := linker.Rewrite(c.Content)
			if fixed != c.Content {
				report.CommentsUpdated++
				if !dryRun {
					if err := db.UpdateCommentContent(c.ID, fixed); err != nil {
						return nil, err
					}
				}
			}
			for _, old := range linker.FindOldLinks(fixed) {
				report.RemainingOldLinks = append(report.RemainingOldLinks,
					fmt.Sprintf("comment:%d:%s", c.ID, old))
			}
		}
		if page*200 >= total || len(comments) == 0 {
			break
		}
		page++
	}

	return report, nil
}
