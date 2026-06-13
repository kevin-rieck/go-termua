package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"termua/internal/app"
	"termua/internal/config"
	"termua/internal/opcua"
	"termua/internal/tui"
)

func main() {
	if os.Getenv("DEBUG") != "" {
		f, err := tea.LogToFile("debug.log", "debug")
		if err != nil {
			fmt.Fprintln(os.Stderr, "fatal:", err)
			os.Exit(1)
		}
		defer f.Close()
	} else {
		log.SetOutput(io.Discard)
	}

	opts, err := app.ParseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	paths, err := config.ResolvePaths(config.PathOptions{Portable: opts.Portable})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if opts.ClientCertificatePath == "" && opts.ClientPrivateKeyPath == "" {
		certificatePath, privateKeyPath, err := config.EnsureClientCertificate(paths)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		opts.ClientCertificatePath = certificatePath
		opts.ClientPrivateKeyPath = privateKeyPath
	}

	client := opcua.NewClient()
	defer client.Close(context.Background())

	model := tui.NewModel(tui.Dependencies{
		Client: client,
		Paths:  paths,
		Launch: opts,
	})

	program := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := program.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
