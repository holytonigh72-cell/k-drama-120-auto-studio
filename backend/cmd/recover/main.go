package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"kdrama120/internal/pipeline"
)

func main() {
	config := flag.String("config", "", "Codex config.toml/json path; defaults to the local Codex profile")
	workspace := flag.String("workspace", "./data/recovered", "asset workspace")
	flag.Parse()
	root, err := filepath.Abs(*workspace)
	check(err)
	svc := pipeline.NewService(root)
	assets, err := svc.RecoverAbocadoAssets(*config, "abocado")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, a := range assets {
		fmt.Printf("%s %s %s\n", a.Kind, a.Status, a.LocalPath)
		if a.Error != "" {
			fmt.Fprintf(os.Stderr, "%s: %s\n", a.JobKey, a.Error)
		}
	}
	fmt.Printf("Recovery complete: %d existing jobs; no generation submitted.\n", len(assets))
}
func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

