package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"kdrama120/internal/pipeline"
)

func main() {
	workspace := flag.String("workspace", "./data", "state and outputs folder")
	title := flag.String("title", "계약결혼한 남자의 비밀", "project title")
	premise := flag.String("premise", "계약결혼으로 만난 두 성인이 서로의 비밀을 알아간다", "story premise")
	resumeID := flag.String("resume", "", "resume a saved project ID")
	importRoots := flag.String("import", "", "semicolon-separated local folders to scan")
	flag.Parse()
	root, err := filepath.Abs(*workspace)
	check(err)
	store, err := pipeline.NewStore(filepath.Join(root, "projects"))
	check(err)
	if *importRoots != "" {
		roots := filepath.SplitList(*importRoots)
		assets, er := pipeline.ImportAssets(roots, []string{filepath.Join(root, "imported")})
		check(er)
		b, _ := json.MarshalIndent(assets, "", "  ")
		check(os.MkdirAll(filepath.Join(root, "imported"), 0755))
		check(os.WriteFile(filepath.Join(root, "imported", "inventory.json"), b, 0600))
		fmt.Printf("Imported asset inventory: %d files\n", len(assets))
	}
	var p *pipeline.Project
	if *resumeID != "" {
		p, err = store.Get(*resumeID)
		check(err)
	} else {
		p = pipeline.NewProject(root, *title, *premise)
		check(store.Save(p))
	}
	if p.Provider != "mock" {
		fmt.Println("Paid provider is disconnected. No credits will be used.")
	}
	err = p.Resume(context.Background(), pipeline.MockRunner{}, store)
	check(err)
	b, err := json.MarshalIndent(p, "", "  ")
	check(err)
	fmt.Println(string(b))
	fmt.Printf("Mock run complete. No media was generated and no credits were used.\n")
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

