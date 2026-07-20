# slogger

> The slog blogger

A scrappy static site generator focused on quick writing, plaintext files, and minimal dependencies. This version is written in Go.

Inspired by WNOADIARWB's [slog of thoughts](https://wnoadiarwb.us) and Bradley Taunt's [barf](https://barf.btxx.org/).

## Usage

This project is under active development, so expect some bugs when using it.

Create a `config.py` in `lib/` that defines the following variables:

```python
SITE_NAME="example.com"
SITE_URL="https://example.com"
SITE_DESCRIPTION="This is my slogger site"
BUILD_DIR="build"
MD_DIR="posts"
ASSETS_DIR="public"
COPYRIGHT_NAME="example.com"
FEATURED_POSTS_COUNT=5
```

Create a file called `index.md`. Example:

```markdown
# My Site

This is my slogger site.
```
