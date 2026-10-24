// Package imageaudit finds images referenced by posts and checks whether local
// uploads exist.
package imageaudit

import (
	"fmt"
	"html"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/pionus/pionus-go/internal/content"
	"github.com/pionus/pionus-go/internal/model"
	"github.com/pionus/pionus-go/internal/store"
)

var imgTagRe = regexp.MustCompile(`(?is)<img\b[^>]*\bsrc\s*=\s*["']([^"']+)`)

// Item is one image reference.
type Item struct {
	PostSlug  string `json:"post_slug"`
	PostTitle string `json:"post_title"`
	URL       string `json:"url"`
	Kind      string `json:"kind"`
	Status    string `json:"status"`
}

// Report is the audit result.
type Report struct {
	Posts       int            `json:"posts"`
	References  int            `json:"references"`
	Local       int            `json:"local"`
	Missing     int            `json:"missing"`
	External    int            `json:"external"`
	Unreachable int            `json:"unreachable,omitempty"`
	Hosts       map[string]int `json:"hosts"`
	Items       []Item         `json:"missing_or_unreachable,omitempty"`
}

// Audit scans published posts. When online is true, external URLs are checked
// with a HEAD request.
func Audit(db store.Store, uploadDir string, online bool) (*Report, error) {
	report := &Report{Hosts: make(map[string]int)}

	var posts []*model.Post
	page := 1
	for {
		batch, total, err := db.ListPosts(store.PostFilter{
			Page:        page,
			Limit:       200,
			WithContent: true,
		})
		if err != nil {
			return nil, err
		}
		posts = append(posts, batch...)
		if page*200 >= total || len(batch) == 0 {
			break
		}
		page++
	}
	report.Posts = len(posts)

	seen := make(map[string]bool)
	var externalRefs []Item
	for _, p := range posts {
		rendered := string(content.RenderPost(p).HTML)
		for _, raw := range extract(rendered) {
			raw = strings.TrimSpace(html.UnescapeString(raw))
			if raw == "" || strings.HasPrefix(raw, "data:") {
				continue
			}
			key := p.Slug + "\x00" + raw
			if seen[key] {
				continue
			}
			seen[key] = true
			report.References++

			item := Item{PostSlug: p.Slug, PostTitle: p.Title, URL: raw}
			switch {
			case strings.HasPrefix(raw, "/uploads/"):
				item.Kind = "local"
				report.Local++
				rel := strings.TrimPrefix(raw, "/uploads/")
				if !fileExists(filepath.Join(uploadDir, filepath.FromSlash(rel))) {
					item.Status = "missing"
					report.Missing++
					report.Items = append(report.Items, item)
				}
			case strings.HasPrefix(raw, "http://"), strings.HasPrefix(raw, "https://"), strings.HasPrefix(raw, "//"):
				item.Kind = "external"
				report.External++
				host := hostOf(raw)
				report.Hosts[host]++
				externalRefs = append(externalRefs, item)
			default:
				continue
			}
		}
	}

	if online && len(externalRefs) > 0 {
		checkExternal(externalRefs, report)
	}

	return report, nil
}

func checkExternal(refs []Item, report *Report) {
	unique := make(map[string]bool, len(refs))
	for _, ref := range refs {
		unique[ref.URL] = true
	}

	type result struct {
		url string
		ok  bool
	}

	jobs := make(chan string)
	results := make(chan result)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for u := range jobs {
				results <- result{url: u, ok: reachable(u)}
			}
		}()
	}
	go func() {
		for u := range unique {
			jobs <- u
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()

	bad := make(map[string]bool)
	for r := range results {
		if !r.ok {
			bad[r.url] = true
		}
	}

	reported := make(map[string]bool)
	for _, ref := range refs {
		if !bad[ref.URL] || reported[ref.URL] {
			continue
		}
		reported[ref.URL] = true
		ref.Status = "unreachable"
		report.Items = append(report.Items, ref)
		report.Unreachable++
	}
}

func extract(html string) []string {
	var out []string
	for _, m := range imgTagRe.FindAllStringSubmatch(html, -1) {
		if len(m) > 1 {
			out = append(out, m[1])
		}
	}
	return out
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func hostOf(raw string) string {
	if strings.HasPrefix(raw, "//") {
		raw = "https:" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "(invalid)"
	}
	return u.Host
}

var httpClient = &http.Client{
	Timeout: 6 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 {
			return fmt.Errorf("too many redirects")
		}
		return nil
	},
}

func reachable(raw string) bool {
	if strings.HasPrefix(raw, "//") {
		raw = "https:" + raw
	}
	req, err := http.NewRequest(http.MethodHead, raw, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", "pionus-image-audit/1.0")
	resp, err := httpClient.Do(req)
	if err != nil {
		// Retry with GET; some hosts reject HEAD.
		req, err = http.NewRequest(http.MethodGet, raw, nil)
		if err != nil {
			return false
		}
		req.Header.Set("User-Agent", "pionus-image-audit/1.0")
		resp, err = httpClient.Do(req)
		if err != nil {
			return false
		}
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 400
}
