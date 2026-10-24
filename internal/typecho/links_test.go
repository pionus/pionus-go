package typecho

import (
	"strings"
	"testing"
)

func TestLinkFixerRewrite(t *testing.T) {
	f := NewLinkFixer("https://old.example")
	f.SetPostSlug(11, "md-post")
	f.SetPageSlug("about", "about")

	cases := []struct{ in, want string }{
		{"http://old.example/index.php/archives/11/", "/article/md-post"},
		{"https://old.example/archives/11/", "/article/md-post"},
		{"//old.example/index.php/archives/11/", "/article/md-post"},
		{"href=\"/index.php/archives/11/\"", "href=\"/article/md-post\""},
		{"/index.php/about.html", "/article/about"},
		{"/about.html", "/article/about"},
		{"/index.php/category/tech/2/", "/category/tech?page=2"},
		{"/index.php/tag/go/", "/tag/go"},
		{"/index.php/search/注入/", "/search?q=注入"},
		{"/index.php/feed/", "/feed"},
		{"/usr/uploads/2015/03/a.jpg", "/uploads/2015/03/a.jpg"},
		{"http://old.example/usr/uploads/2015/03/a.jpg", "/uploads/2015/03/a.jpg"},
	}
	for _, tc := range cases {
		if got := f.Rewrite(tc.in); got != tc.want {
			t.Errorf("Rewrite(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}

	// External hosts and unknown ids are left alone.
	for _, in := range []string{
		"http://other.example/archives/1242.html",
		"http://other.example/index.php/archives/11/",
		"/index.php/archives/999/",
	} {
		if got := f.Rewrite(in); got != in {
			t.Errorf("Rewrite(%q) changed external/unknown link to %q", in, got)
		}
	}
}

func TestLinkFixerFindOldLinks(t *testing.T) {
	f := NewLinkFixer("https://old.example")
	f.SetPageSlug("about", "about")

	remaining := f.FindOldLinks("see /index.php/archives/999/ and /usr/uploads/a.jpg and /about.html")
	joined := strings.Join(remaining, " ")
	for _, want := range []string{"/index.php/archives/999/", "/usr/uploads/a.jpg", "/about.html"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in %v", want, remaining)
		}
	}
	if got := f.FindOldLinks("http://other.example/archives/1242.html"); len(got) != 0 {
		t.Fatalf("external link flagged: %v", got)
	}
}
