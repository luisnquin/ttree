package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/luisnquin/ttree/internal/db"
	"github.com/luisnquin/ttree/internal/ui"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "ttree: A simple task tree CLI\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Usage:\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  ttree [flags]\n")
		fmt.Fprintf(flag.CommandLine.Output(), "	ls\tShow the task tree in a pass-like format\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Flags:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	dbInst, err := db.Open()
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer dbInst.Close()

	if err := dbInst.InitSchema(context.Background()); err != nil {
		log.Fatalf("failed to init schema: %v", err)
	}

	args := flag.Args()
	if len(args) > 0 && args[0] == "ls" {
		if err := ui.PrintTree(dbInst); err != nil {
			log.Fatalf("failed to print tree: %v", err)
		}
		return
	}

	app, err := ui.New(dbInst)
	if err != nil {
		log.Fatalf("failed to init ui: %v", err)
	}

	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
