package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"complex/internal/app"
	"complex/internal/claude"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// Set up signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		cancel()
	}()

	// Create session manager
	sessionManager := claude.NewSessionManager()

	// Create application
	tuiApp, err := app.NewApplication(ctx, sessionManager)
	if err != nil {
		fmt.Printf("Error creating application: %v\n", err)
		os.Exit(1)
	}

	// Create bubbletea program
	program := tea.NewProgram(
		tuiApp,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	// Set the program in the application for shutdown handling
	tuiApp.SetProgram(program)

	// Start the program
	if _, err := program.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}