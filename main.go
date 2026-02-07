package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hilli/kefw2ui/config"
	"github.com/hilli/kefw2ui/server"
	"github.com/hilli/kefw2ui/speaker"
	"tailscale.com/tsnet"
)

//go:embed all:frontend/build
var frontendFS embed.FS

// Set via ldflags at build time
var (
	version = "dev"
	commit  = "none"
)

// envOrDefault returns the environment variable value if set, otherwise the default.
func envOrDefault(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

// envBool returns true if the environment variable is set to a truthy value.
func envBool(key string) bool {
	v := os.Getenv(key)
	return v == "1" || v == "true" || v == "yes"
}

func main() {
	var (
		bind        string
		port        int
		showVersion bool
		tsEnabled   bool
		tsHostname  string
		tsAuthKey   string
		tsStateDir  string
	)

	flag.StringVar(&bind, "bind", envOrDefault("KEFW2UI_BIND", "0.0.0.0"), "Address to bind to")
	flag.IntVar(&port, "port", 8080, "Port to listen on")
	flag.BoolVar(&showVersion, "version", false, "Print version and exit")

	// Tailscale flags (env vars provide defaults)
	flag.BoolVar(&tsEnabled, "tailscale", envBool("TS_ENABLED"), "Enable Tailscale listener")
	flag.StringVar(&tsHostname, "tailscale-hostname", envOrDefault("TS_HOSTNAME", "kefw2ui"), "Hostname on the tailnet")
	flag.StringVar(&tsAuthKey, "tailscale-authkey", envOrDefault("TS_AUTHKEY", ""), "Tailscale auth key for headless login")
	flag.StringVar(&tsStateDir, "tailscale-dir", envOrDefault("TS_STATE_DIR", ""), "Directory for Tailscale state persistence")

	flag.Parse()

	if showVersion {
		fmt.Printf("kefw2ui %s (%s)\n", version, commit)
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

	// Tailscale listener (optional)
	var tsServer *tsnet.Server
	if tsEnabled {
		tsServer = &tsnet.Server{
			Hostname: tsHostname,
		}
		if tsAuthKey != "" {
			tsServer.AuthKey = tsAuthKey
		}
		if tsStateDir != "" {
			tsServer.Dir = tsStateDir
		}

		if err := tsServer.Start(); err != nil {
			log.Fatalf("Tailscale error: %v", err)
		}

		ln, err := tsServer.ListenTLS("tcp", ":443")
		if err != nil {
			log.Fatalf("Tailscale ListenTLS error: %v", err)
		}

		go func() {
			log.Printf("Tailscale HTTPS listener active on %s:443", tsHostname)
			if err := http.Serve(ln, srv.Handler()); err != nil {
				log.Fatalf("Tailscale serve error: %v", err)
			}
		}()
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("Shutting down...")
		speakerMgr.Close()
		if tsServer != nil {
			tsServer.Close()
		}
		os.Exit(0)
	}()

	addr := fmt.Sprintf("%s:%d", bind, port)
	log.Printf("Starting kefw2ui %s on http://%s", version, addr)

	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
