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
	log.Printf("Ensuring posts directory %q exists", postsDir)
	if err := os.MkdirAll(postsDir, 0o755); err != nil {
		log.Fatal(err)
	}

	log.Print("Generating post filename")
	epoch := time.Now().UTC().Unix()
	date := time.Unix(epoch, 0).UTC().Format("20060102")
	name := date + "_" + strconv.FormatInt(epoch, 10)
	if *title != "" {
		name += " " + *title
		log.Printf("Using title %q", *title)
	} else {
		log.Print("No title provided")
	}
	fname := internal.Slugify(name) + ".md"
	path := filepath.Join(postsDir, fname)
	log.Printf("Post path: %s", path)

	log.Printf("Opening editor for %s", path)
	cmd := exec.Command("vim", path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatal(err)
	}
}
