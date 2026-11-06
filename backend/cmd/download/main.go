package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"kdrama120/internal/abocado"
)

func main() {
	manifest := flag.String("manifest", "", "asset manifest JSON")
	out := flag.String("out", "./data/assets/recovered", "download destination")
	flag.Parse()
	if *manifest == "" {
		fmt.Fprintln(os.Stderr, "-manifest is required")
		os.Exit(2)
	}
	b, err := os.ReadFile(*manifest)
	check(err)
	var assets []abocado.Asset
	check(json.Unmarshal(b, &assets))
	downloader := abocado.NewDownloader()
	for _, a := range assets {
		saved, e := downloader.Download(context.Background(), a, *out)
		if e != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", a.JobKey, e)
			continue
		}
		fmt.Printf("%s -> %s\n", a.JobKey, saved.LocalPath)
	}
	abs, _ := filepath.Abs(*out)
	fmt.Printf("download folder: %s\n", abs)
}
func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

