package main

import (
	"context"
	"embed"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/hilli/kefw2ui/config"
	"github.com/hilli/kefw2ui/server"
	"github.com/hilli/kefw2ui/speaker"
	"tailscale.com/tsnet"
)

//go:embed all:frontend/build
var frontendFS embed.FS

// Set via ldflags at build time.
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

// envInt returns the environment variable as an int, or the fallback if unset/invalid.
func envInt(key string, fallback int) int {
	v, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

// parseDurationWithDays parses a duration string that may use a "d" suffix for days.
// Examples: "0" (zero/disabled), "1h", "7d", "30d", "168h".
// Falls back to time.ParseDuration for standard Go duration strings.
func parseDurationWithDays(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "0" {
		return 0, nil
	}
	if strings.HasSuffix(s, "d") {
		numStr := strings.TrimSuffix(s, "d")
		days, err := strconv.Atoi(numStr)
		if err != nil {
			return 0, fmt.Errorf("invalid day duration %q: %w", s, err)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

//nolint:gocyclo // main orchestrates startup/shutdown; splitting would obscure the flow.
func main() {
	var (
		bind            string
		port            int
		showVersion     bool
		speakerIPs      string
		noDiscovery     bool
		tsEnabled       bool
		tsHostname      string
		tsAuthKey       string
		tsStateDir      string
		imageCacheTTL   string
		imageCacheMemMB int
	)

	flag.StringVar(&bind, "bind", envOrDefault("KEFW2UI_BIND", "0.0.0.0"), "Address to bind to")
	flag.IntVar(&port, "port", envInt("KEFW2UI_PORT", 8080), "Port to listen on")
	flag.BoolVar(&showVersion, "version", false, "Print version and exit")

	// Speaker flags (env vars provide defaults)
	flag.StringVar(&speakerIPs, "speaker-ips", envOrDefault("KEFW2UI_SPEAKER_IPS", ""), "Comma-separated list of speaker IP addresses")
	flag.BoolVar(&noDiscovery, "no-discovery", envBool("KEFW2UI_NO_DISCOVERY"), "Skip mDNS speaker discovery")

	// Image cache flags (env vars provide defaults)
	flag.StringVar(&imageCacheTTL, "image-cache-ttl", envOrDefault("KEFW2UI_IMAGE_CACHE_TTL", "7d"), "Image cache TTL (0 = never expire). Examples: 1h, 7d, 30d")
	flag.IntVar(&imageCacheMemMB, "image-cache-mem-mb", envInt("KEFW2UI_IMAGE_CACHE_MEM_MB", 50), "Max memory for image cache in MB")

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

	// Parse image cache TTL
	imgTTL, err := parseDurationWithDays(imageCacheTTL)
	if err != nil {
		log.Fatalf("Invalid --image-cache-ttl %q: %v", imageCacheTTL, err)
	}

	// Create speaker manager
	speakerMgr := speaker.NewManager()

	// Create server
	srv := server.New(server.Options{
		Bind:            bind,
		Port:            port,
		FrontendFS:      frontendFS,
		Config:          cfg,
		SpeakerManager:  speakerMgr,
		ImageCacheTTL:   imgTTL,
		ImageCacheMemMB: imageCacheMemMB,
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

		// Add speakers from --speaker-ips flag
		if speakerIPs != "" {
			for _, raw := range strings.Split(speakerIPs, ",") {
				ip := strings.TrimSpace(raw)
				if ip == "" {
					continue
				}
				log.Printf("Adding speaker from --speaker-ips: %s", ip)
				if _, err := speakerMgr.AddSpeaker(ctx, ip); err != nil {
					log.Printf("Warning: could not add speaker %s: %v", ip, err)
				}
			}
		}

		// Then discover speakers on the network (unless --no-discovery is set)
		if !noDiscovery {
			speakers, err := speakerMgr.Discover(ctx)
			if err != nil {
				log.Printf("Speaker discovery error: %v", err)
			} else {
				log.Printf("Discovered %d speaker(s)", len(speakers))
			}
		} else {
			log.Printf("Speaker discovery disabled (--no-discovery)")
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
			if err := http.Serve(ln, srv.Handler()); err != nil { //nolint:gosec // local Tailscale listener, timeouts not needed
				log.Fatalf("Tailscale serve error: %v", err)
			}
		}()
	}

	// Graceful shutdown: wait for signal, then drain connections.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	addr := fmt.Sprintf("%s:%d", bind, port)
	log.Printf("Starting kefw2ui %s on http://%s", version, addr)

	// Run the HTTP server in a goroutine so the main goroutine can wait for signals.
	srvErr := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			srvErr <- err
		}
		close(srvErr)
	}()

	// Block until we get a signal or the server fails to start.
	select {
	case err := <-srvErr:
		if err != nil {
			log.Fatalf("Server error: %v", err)
		}
	case sig := <-sigCh:
		log.Printf("Received %v, shutting down...", sig)
	}

	// Give in-flight requests up to 5 seconds to finish.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	if tsServer != nil {
		_ = tsServer.Close()
	}

	speakerMgr.Close()
	log.Println("Shutdown complete")
}
