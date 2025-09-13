package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"path/filepath"

	"github.com/loykin/apimigrate"
	"github.com/loykin/apimigrate/pkg/status"
)

func main() {
	// Flags
	history := flag.Bool("history", false, "print migration run history")
	defaultDir := filepath.Join("examples", "status_embedded", "migration")
	dir := flag.String("dir", defaultDir, "migration directory to read status from (must contain apimigrate.db)")
	flag.Parse()

	// 1) Run migrations first so there is actual history to show
	ctx := context.Background()
	m := apimigrate.Migrator{Dir: *dir}
	if _, err := m.MigrateUp(ctx, 0); err != nil {
		log.Printf("warning: migrate up encountered an error: %v", err)
	}

	// 2) Read and print status from the same directory
	info, err := status.FromOptions(*dir, nil)
	if err != nil {
		log.Fatalf("failed to get status: %v", err)
	}

	fmt.Print(info.FormatHuman(*history))
}
