package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hilli/kefw2ui/config"
	"github.com/hilli/kefw2ui/server"
	"github.com/hilli/kefw2ui/speaker"
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

	// Create speaker manager
	speakerMgr := speaker.NewManager()

	// Create server
	srv := server.New(server.Options{
		Bind:           bind,
		Port:           port,
		FrontendFS:     frontendFS,
		Config:         cfg,
		SpeakerManager: speakerMgr,
	})

	// Wire up speaker events to SSE broadcast
	speakerMgr.SetEventCallback(srv.HandleSpeakerEvent)

	// Initial speaker discovery
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		speakers, err := speakerMgr.Discover(ctx)
		if err != nil {
			log.Printf("Speaker discovery error: %v", err)
			return
		}
		log.Printf("Discovered %d speaker(s)", len(speakers))

		// If we have a default speaker configured, connect to it
		if cfg != nil && cfg.Speakers.Default != "" {
			if err := speakerMgr.SetActiveSpeaker(context.Background(), cfg.Speakers.Default); err != nil {
				log.Printf("Could not connect to default speaker %s: %v", cfg.Speakers.Default, err)
			} else {
				log.Printf("Connected to default speaker: %s", cfg.Speakers.Default)
			}
		} else if len(speakers) > 0 {
			// Auto-connect to first discovered speaker
			if err := speakerMgr.SetActiveSpeaker(context.Background(), speakers[0].IPAddress); err != nil {
				log.Printf("Could not connect to speaker %s: %v", speakers[0].IPAddress, err)
			} else {
				log.Printf("Connected to speaker: %s (%s)", speakers[0].Name, speakers[0].IPAddress)
			}
		}
	}()

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("Shutting down...")
		speakerMgr.Close()
		os.Exit(0)
	}()

	addr := fmt.Sprintf("%s:%d", bind, port)
	log.Printf("Starting kefw2ui on http://%s", addr)

	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
