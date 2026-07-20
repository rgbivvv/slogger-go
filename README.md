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
  "featured_posts_count": 5
}
```

Create a file called `index.md`. Example:

```markdown
# My Site

This is my slogger site.
```
