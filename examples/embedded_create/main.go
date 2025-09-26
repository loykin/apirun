package main

import (
	"flag"
	"fmt"
	"log"
	"path/filepath"

	"github.com/loykin/apirun"
)

func main() {
	// Flags
	defaultDir := filepath.Join("examples", "embedded_create", "migration")
	dir := flag.String("dir", defaultDir, "directory to place the new migration file")
	flag.Parse()
	name := "task"
	if flag.NArg() > 0 {
		name = flag.Arg(0)
	}

	p, err := apirun.CreateMigration(apirun.CreateOptions{Name: name, Dir: *dir})
	if err != nil {
		log.Fatalf("create migration: %v", err)
	}
	fmt.Println(p)
}
