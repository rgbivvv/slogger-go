package main

import (
	"bytes"
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

	cfg, err := internal.LoadConfig("config.json")
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	mdDir, err := internal.EnsureDir(cfg.MDDir)
	if err != nil {
		log.Fatal(err)
	}
	buildDir, err := internal.EnsureDir(cfg.BuildDir)
	if err != nil {
		log.Fatal(err)
	}
	buildTemp, err := internal.EnsureDir("build.temp")
	if err != nil {
		log.Fatal(err)
	}
	assetsDir, err := internal.EnsureDir(cfg.AssetsDir)
	if err != nil {
		log.Fatal(err)
	}

	policy := internal.SanitizePolicy()
	r, err := internal.NewRenderer(cfg)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Parsing Markdown files from %q", mdDir)
	postList, err := internal.ParsePosts(mdDir, assetsDir, cfg, policy)
	if err != nil {
		log.Fatal(err)
	}

	if err := copyTree(assetsDir, filepath.Join(buildTemp, assetsDir)); err != nil {
		log.Fatal(err)
	}

	n, err := internal.WritePostPages(postList, buildTemp, cfg, r)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Wrote a total of %d posts", n)

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
		internal.SanitizeHTML(indexBuf.String(), policy),
		postListHTML,
		feedContent.String(),
	}, filepath.Join(buildTemp, "index.html")); err != nil {
		log.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(buildTemp, "feed.xml"), []byte(rssFeed(postList, cfg)), 0o644); err != nil {
		log.Fatal(err)
	}

	if err := internal.WipeDirFilesOnly(buildDir); err != nil {
		log.Fatal(err)
	}
	if err := copyTree(buildTemp, buildDir); err != nil {
		log.Fatal(err)
	}
	if err := os.RemoveAll(buildTemp); err != nil {
		log.Fatal(err)
	}
	log.Print("Done.")
}

func rssFeed(postList []internal.Post, cfg *internal.Config) string {
	log.Print("Generating RSS feed")
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
