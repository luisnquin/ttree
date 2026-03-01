package main

import (
	"context"
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/luisnquin/ttree/internal/db"
	"github.com/luisnquin/ttree/internal/ui"
)

func main() {
	dbInst, err := db.Open()
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer dbInst.Close()

	if err := dbInst.InitSchema(context.Background()); err != nil {
		log.Fatalf("failed to init schema: %v", err)
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
