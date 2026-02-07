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

//go:embed all:frontend/build
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

	// Wire up speaker health changes to SSE broadcast
	speakerMgr.SetHealthCallback(srv.HandleSpeakerHealth)

	// Initial speaker discovery and connection
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// First, load any speakers from config
		if cfg != nil {
			for _, spkCfg := range cfg.GetSpeakers() {
				log.Printf("Loading configured speaker: %s (%s)", spkCfg.Name, spkCfg.IPAddress)
				speakerMgr.AddConfiguredSpeaker(spkCfg.IPAddress, spkCfg.Name, spkCfg.Model)
			}
		}

		// Then discover speakers on the network
		speakers, err := speakerMgr.Discover(ctx)
		if err != nil {
			log.Printf("Speaker discovery error: %v", err)
		} else {
			log.Printf("Discovered %d speaker(s)", len(speakers))
		}

		// Connect to default speaker if configured
		if cfg != nil && cfg.GetDefaultSpeaker() != "" {
			defaultIP := cfg.GetDefaultSpeaker()
			if err := speakerMgr.SetActiveSpeaker(context.Background(), defaultIP); err != nil {
				log.Printf("Could not connect to default speaker %s: %v", defaultIP, err)
			} else {
				log.Printf("Connected to default speaker: %s", defaultIP)
			}
		} else if len(speakerMgr.GetSpeakers()) > 0 {
			// Auto-connect to first available speaker
			speakers := speakerMgr.GetSpeakers()
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
