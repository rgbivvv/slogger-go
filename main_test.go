package main

import (
	"encoding/xml"
	"strings"
	"testing"

	"slogger-go/internal"
)

func TestRSSFeedIsValidXML(t *testing.T) {
	cfg := &internal.Config{
		SiteName:        "Tom & Jerry's <blog>",
		SiteURL:         "https://example.com/",
		SiteDescription: "Notes & thoughts",
	}
	posts := []internal.Post{
		{Slug: "q_a", Title: "Q&A <draft>", Epoch: 1767657600, HTMLContent: "<p>a & b</p>"},
		{Slug: "raw", Title: "raw", Epoch: 1767571200, HTMLContent: "<div>raw ]]> html</div>"},
	}
	out := rssFeed(posts, cfg)

	if !strings.HasPrefix(out, "<?xml") {
		t.Errorf("feed must start with the XML declaration, got %q", out[:10])
	}
	var feed struct {
		Channel struct {
			Title string `xml:"title"`
			Items []struct {
				Title       string `xml:"title"`
				Link        string `xml:"link"`
				Description string `xml:"description"`
			} `xml:"item"`
		} `xml:"channel"`
	}
	if err := xml.Unmarshal([]byte(out), &feed); err != nil {
		t.Fatalf("feed is not valid XML: %v\n%s", err, out)
	}
	if got := feed.Channel.Title; got != cfg.SiteName {
		t.Errorf("channel title=%q want %q", got, cfg.SiteName)
	}
	items := feed.Channel.Items
	if len(items) != 2 {
		t.Fatalf("got %d items want 2", len(items))
	}
	if items[0].Title != "Q&A <draft>" {
		t.Errorf("item title=%q", items[0].Title)
	}
	if items[0].Link != "https://example.com/q_a.html" {
		t.Errorf("item link=%q", items[0].Link)
	}
	if items[1].Description != "<div>raw ]]> html</div>" {
		t.Errorf("item description=%q", items[1].Description)
	}
}

func TestAbsoluteURLs(t *testing.T) {
	cases := []struct{ in, want string }{
		{`<img src="/public/img/x.png">`, `<img src="https://example.com/public/img/x.png">`},
		{`<img src="public/img/x.png">`, `<img src="https://example.com/public/img/x.png">`},
		{`<a href='./other.html'>`, `<a href='https://example.com/other.html'>`},
		{`<a href="https://other.org/x">`, `<a href="https://other.org/x">`},
		{`<img src="//cdn.org/x.png">`, `<img src="//cdn.org/x.png">`},
		{`<a href="#fn1">`, `<a href="#fn1">`},
		{`<a href="mailto:a@b.c">`, `<a href="mailto:a@b.c">`},
	}
	for _, c := range cases {
		if got := absoluteURLs(c.in, "https://example.com"); got != c.want {
			t.Errorf("absoluteURLs(%q)=%q want %q", c.in, got, c.want)
		}
	}
}
