package main

import (
	"bytes"
	"encoding/xml"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"slogger-go/internal"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)

	force := flag.Bool("force", false, "ignore cache and rebuild all posts")
	flag.Parse()
	if *force {
		log.Print("clearing cache (.slogger-cache)")
		if err := os.RemoveAll(".slogger-cache"); err != nil {
			log.Fatalf("clear cache: %v", err)
		}
		log.Print("cleared cache")
	}

	cfg, err := internal.LoadConfig("config.json")
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	for _, dir := range []string{cfg.MDDir, cfg.BuildDir, cfg.AssetsDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Fatal(err)
		}
	}
	mdDir, buildDir, assetsDir := cfg.MDDir, cfg.BuildDir, cfg.AssetsDir

	r, err := internal.NewRenderer(cfg)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("parsing markdown files from %q", mdDir)
	postList, err := internal.ParsePosts(mdDir, assetsDir, cfg)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("found %d posts", len(postList))

	assetsOut := filepath.Join(buildDir, assetsDir)
	log.Printf("syncing assets %q -> %q", assetsDir, assetsOut)
	if err := internal.SyncTree(assetsDir, assetsOut); err != nil {
		log.Fatal(err)
	}

	log.Printf("writing post pages to %q", buildDir)
	written, copied, err := internal.WritePostPages(postList, buildDir, ".slogger-cache", cfg, r)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("pages: %d copied, %d written", copied, written)

	log.Print("building index.html")
	// newest first for index
	for i, j := 0, len(postList)-1; i < j; i, j = i+1, j-1 {
		postList[i], postList[j] = postList[j], postList[i]
	}

	featured := "<ul>"
	limit := cfg.FeaturedPostsCount
	if limit > len(postList) {
		limit = len(postList)
	}
	for _, p := range postList[:limit] {
		featured += fmt.Sprintf(
			`<li><span class="post-list-link"><a href="%s">%s</a></span></li>`,
			p.Permalink, p.Title,
		)
	}
	featured += "</ul>"
	postListHTML := featured
	if len(postList) > cfg.FeaturedPostsCount {
		other := `<details><summary><small><i>more posts...</i></small></summary><ul>`
		for _, p := range postList[cfg.FeaturedPostsCount:] {
			other += fmt.Sprintf(
				`<li><span class="post-list-link"><a href="%s">%s</a></span></li>`,
				p.Permalink, p.Title,
			)
		}
		other += "</ul></details>"
		postListHTML += other
	}

	var feedContent strings.Builder
	for _, p := range postList {
		feedContent.WriteString("\n<br><br><br>\n")
		feedContent.WriteString(fmt.Sprintf(
			`<span class="post-link"><small><b><a href="%s">%s</a></b><small> ( at %d )</small></small></span>`,
			p.Permalink, p.Title, p.Epoch,
		))
		feedContent.WriteString("\n<br>")
		feedContent.WriteString(p.HTMLContent)
	}

	indexMD, err := os.ReadFile("index.md")
	if err != nil {
		log.Fatal(err)
	}
	var indexBuf bytes.Buffer
	if err := internal.NewMarkdown().Convert(indexMD, &indexBuf); err != nil {
		log.Fatal(err)
	}
	if err := r.RenderWrite("", []string{
		indexBuf.String(),
		postListHTML,
		feedContent.String(),
	}, filepath.Join(buildDir, "index.html")); err != nil {
		log.Fatal(err)
	}

	log.Print("generating rss feed")
	if err := os.WriteFile(filepath.Join(buildDir, "feed.xml"), []byte(rssFeed(postList, cfg)), 0o644); err != nil {
		log.Fatal(err)
	}

	keep := map[string]struct{}{"index.html": {}}
	for _, p := range postList {
		keep[p.Fname] = struct{}{}
	}
	if err := internal.PruneStaleHTML(buildDir, keep); err != nil {
		log.Fatal(err)
	}
	log.Print("done.")
}

func rssFeed(postList []internal.Post, cfg *internal.Config) string {
	n := cfg.RSSPostsCount
	if n <= 0 || n > len(postList) {
		n = len(postList)
	}
	postList = postList[:n]
	siteURL := strings.TrimRight(cfg.SiteURL, "/")
	var items strings.Builder
	for _, p := range postList {
		permalink := xmlEscape(siteURL + "/" + p.Slug + ".html")
		pub := time.Unix(p.Epoch, 0).UTC().Format("Mon, 02 Jan 2006 15:04:05 GMT")
		fmt.Fprintf(&items, `
        <item>
            <title>%s</title>
            <link>%s</link>
            <description><![CDATA[%s]]></description>
            <pubDate>%s</pubDate>
            <guid isPermaLink="true">%s</guid>
        </item>`, xmlEscape(p.Title), permalink, cdata(absoluteURLs(p.HTMLContent, siteURL)), pub, permalink)
	}
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:atom="http://www.w3.org/2005/Atom">
    <channel>
        <title>%s</title>
        <link>%s/</link>
        <description>%s</description>
        <atom:link href="%s/feed.xml" rel="self" type="application/rss+xml"/>%s
    </channel>
</rss>
`, xmlEscape(cfg.SiteName), xmlEscape(siteURL), xmlEscape(cfg.SiteDescription), xmlEscape(siteURL), items.String())
}

func xmlEscape(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

// cdata splits any "]]>" so post HTML can't end the CDATA section early.
func cdata(s string) string {
	return strings.ReplaceAll(s, "]]>", "]]]]><![CDATA[>")
}

var urlAttr = regexp.MustCompile(`(?i)(\b(?:src|href)\s*=\s*)(?:"([^"]*)"|'([^']*)')`)

// absoluteURLs resolves relative src/href values against siteURL, since
// feed readers have no page URL to resolve them against.
func absoluteURLs(htmlContent, siteURL string) string {
	base, err := url.Parse(siteURL + "/")
	if err != nil {
		return htmlContent
	}
	return urlAttr.ReplaceAllStringFunc(htmlContent, func(m string) string {
		sub := urlAttr.FindStringSubmatch(m)
		val, quote := sub[2], `"`
		if strings.HasPrefix(m[len(sub[1]):], "'") {
			val, quote = sub[3], "'"
		}
		u, err := url.Parse(val)
		if err != nil || val == "" || strings.HasPrefix(val, "#") || u.Scheme != "" || u.Host != "" {
			return m
		}
		return sub[1] + quote + base.ResolveReference(u).String() + quote
	})
}
