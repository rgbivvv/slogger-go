package main

import (
	"flag"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"slogger-go/internal"
)

func main() {
	title := flag.String("title", "", "The title of the new post. Put this in quotes")
	flag.StringVar(title, "t", "", "The title of the new post. Put this in quotes")
	flag.Parse()

	postsDir := "posts"
	if _, err := internal.EnsureDir(postsDir); err != nil {
		log.Fatal(err)
	}

	epoch := time.Now().UTC().Unix()
	date := time.Unix(epoch, 0).UTC().Format("20060102")
	name := date + "_" + strconv.FormatInt(epoch, 10)
	if *title != "" {
		name += " " + *title
	}
	fname := internal.Slugify(name) + ".md"
	log.Printf("fname: %s", fname)

	path := filepath.Join(postsDir, fname)
	cmd := exec.Command("vim", path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatal(err)
	}
}
