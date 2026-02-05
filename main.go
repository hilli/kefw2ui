package main

import (
	"embed"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/hilli/kefw2ui/config"
	"github.com/hilli/kefw2ui/server"
)

//go:embed frontend/build/*
var frontendFS embed.FS

func main() {
	var (
		bind    string
		port    int
		version bool
	)

	flag.StringVar(&bind, "bind", "0.0.0.0", "Address to bind to")
	flag.IntVar(&port, "port", 8080, "Port to listen on")
	flag.BoolVar(&version, "version", false, "Print version and exit")
	flag.Parse()

	if version {
		fmt.Println("kefw2ui v0.1.0")
		os.Exit(0)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Printf("Warning: could not load config: %v", err)
	}

	srv := server.New(server.Options{
		Bind:       bind,
		Port:       port,
		FrontendFS: frontendFS,
		Config:     cfg,
	})

	addr := fmt.Sprintf("%s:%d", bind, port)
	log.Printf("Starting kefw2ui on http://%s", addr)

	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
