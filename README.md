# slogger

> The slog blogger

A scrappy static site generator focused on quick writing, plaintext files, and minimal dependencies. This version is written in Go.

Inspired by WNOADIARWB's [slog of thoughts](https://wnoadiarwb.us) and Bradley Taunt's [barf](https://barf.btxx.org/).

## Usage

This project is under active development, so expect some bugs when using it.

Copy `config.example.json` to `config.json` and edit as needed:

```json
{
  "site_name": "example.com",
  "site_url": "https://example.com",
  "site_description": "This is my slogger site",
  "build_dir": "build",
  "md_dir": "posts",
  "assets_dir": "public",
  "copyright_name": "example.com",
  "featured_posts_count": 5,
  "rss_posts_count": 20,
  "index_posts_count": 5
}
```

`index_posts_count` is how many of the newest posts `index.html` shows in full; older posts are only listed as links. Leave it out (or set `0`) to show every post.

### Keeping pages light

- Remote images linked in posts are mirrored into `assets_dir` at their original size. Mirrored JPEGs and PNGs have their metadata (EXIF incl. GPS location, XMP, IPTC, comments, PNG text chunks) stripped without re-encoding; a JPEG's rotation is kept so it still displays upright. Files you put in `assets_dir` yourself are not touched.
- `<img>` tags get `loading="lazy" decoding="async"`; `<video>` and `<audio>` get `preload="none"`. Any of these you set yourself in a post are kept.

Create a file called `index.md`. Example:

```markdown
# My Site

This is my slogger site.
```

Build the site:

```bash
go build -o slogger .
./slogger
```

On subsequent builds, unchanged posts are served from `.slogger-cache/` — markdown parsing and individual page HTML generation are skipped for those posts. The log will show cache hits and page copies:

```
cache: 5 hit, 0 miss
pages: 5 copied, 0 written
```

Use `-force` to clear the cache and rebuild everything. Run this after changing `header.html`, `footer.html`, or site config values (`site_name`, `site_url`), since those are not tracked by the cache:

```bash
./slogger -force
```
