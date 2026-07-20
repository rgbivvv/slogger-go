package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yuin/goldmark"

	"slogger-go/internal"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)

	force := flag.Bool("force", false, "ignore cache and rebuild all posts")
	flag.Parse()
	if *force {
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

	log.Printf("Parsing Markdown files from %q", mdDir)
	postList, err := internal.ParsePosts(mdDir, assetsDir, cfg)
	if err != nil {
		log.Fatal(err)
	}

	assetsOut := filepath.Join(buildDir, assetsDir)
	if err := os.MkdirAll(assetsOut, 0o755); err != nil {
		log.Fatal(err)
	}
	if err := internal.WipeDirFilesOnly(assetsOut); err != nil {
		log.Fatal(err)
	}
	if err := copyTree(assetsDir, assetsOut); err != nil {
		log.Fatal(err)
	}

	written, copied, err := internal.WritePostPages(postList, buildDir, ".slogger-cache", cfg, r)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("pages: %d copied, %d written", copied, written)

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
	if err := goldmark.Convert(indexMD, &indexBuf); err != nil {
		log.Fatal(err)
	}
	if err := r.RenderWrite([]string{
		indexBuf.String(),
		postListHTML,
		feedContent.String(),
	}, filepath.Join(buildDir, "index.html")); err != nil {
		log.Fatal(err)
	}

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
	log.Print("Done.")
}

func rssFeed(postList []internal.Post, cfg *internal.Config) string {
	log.Print("Generating RSS feed")
	n := cfg.RSSPostsCount
	if n <= 0 || n > len(postList) {
		n = len(postList)
	}
	postList = postList[:n]
	var items strings.Builder
	for _, p := range postList {
		permalink := cfg.SiteURL + "/" + p.Slug + ".html"
		pub := time.Unix(p.Epoch, 0).UTC().Format("Mon, 02 Jan 2006 15:04:05 GMT")
		items.WriteString(fmt.Sprintf(`
        <item>
            <title>%s</title>
            <link>%s</link>
            <description><![CDATA[%s]]></description>
            <pubDate>%s</pubDate>


            <guid isPermaLink="true">%s</guid>
        </item>
        `, p.Title, permalink, p.HTMLContent, pub, permalink))
	}
	return fmt.Sprintf(`
<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
    <channel>
        <title>%s</title>
        <link>%s/</link>
        <description>%s</description>

        %s
        
    </channel>
</rss>
    `, cfg.SiteName, cfg.SiteURL, cfg.SiteDescription, items.String())
}

func copyTree(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, in)
		return err
	})
}
