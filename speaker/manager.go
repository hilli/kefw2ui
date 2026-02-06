package speaker

import (
	"context"
	"sync"
	"time"

	"github.com/hilli/go-kef-w2/kefw2"
)

// Manager handles speaker discovery and active speaker management
type Manager struct {
	mu            sync.RWMutex
	speakers      map[string]*kefw2.KEFSpeaker
	activeSpeaker *kefw2.KEFSpeaker
	eventClient   *kefw2.EventClient
	eventCancel   context.CancelFunc

	// Event callbacks
	onEvent func(event kefw2.Event)
}

// NewManager creates a new speaker manager
func NewManager() *Manager {
	return &Manager{
		speakers: make(map[string]*kefw2.KEFSpeaker),
	}
}

// SetEventCallback sets the callback for speaker events
func (m *Manager) SetEventCallback(cb func(event kefw2.Event)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onEvent = cb
}

// Discover finds speakers on the network using mDNS
func (m *Manager) Discover(ctx context.Context) ([]*kefw2.KEFSpeaker, error) {
	// Use 5 second discovery timeout
	speakers, err := kefw2.DiscoverSpeakers(ctx, 5*time.Second)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, s := range speakers {
		m.speakers[s.IPAddress] = s
	}

	return speakers, nil
}

// AddSpeaker manually adds a speaker by IP address
func (m *Manager) AddSpeaker(ctx context.Context, ip string) (*kefw2.KEFSpeaker, error) {
	// Use a longer timeout for manual add - speakers in standby can be slow to respond
	speaker, err := kefw2.NewSpeaker(ip, kefw2.WithTimeout(10*time.Second))
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	m.speakers[ip] = speaker
	m.mu.Unlock()

	return speaker, nil
}

// AddConfiguredSpeaker adds a speaker from config without connecting
// This is used at startup to preload known speakers before discovery
func (m *Manager) AddConfiguredSpeaker(ip, name, model string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Only add if not already present (discovery might have found it)
	if _, exists := m.speakers[ip]; !exists {
		// Create a placeholder speaker - will be fully initialized on connect
		m.speakers[ip] = &kefw2.KEFSpeaker{
			IPAddress: ip,
			Name:      name,
			Model:     model,
		}
	}
}

// GetSpeakers returns all known speakers
func (m *Manager) GetSpeakers() []*kefw2.KEFSpeaker {
	m.mu.RLock()
	defer m.mu.RUnlock()

	speakers := make([]*kefw2.KEFSpeaker, 0, len(m.speakers))
	for _, s := range m.speakers {
		speakers = append(speakers, s)
	}
	return speakers
}

// GetActiveSpeaker returns the currently active speaker
func (m *Manager) GetActiveSpeaker() *kefw2.KEFSpeaker {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.activeSpeaker
}

// SetActiveSpeaker sets the active speaker by IP address
func (m *Manager) SetActiveSpeaker(ctx context.Context, ip string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Stop existing event client
	if m.eventCancel != nil {
		m.eventCancel()
		m.eventCancel = nil
	}
	if m.eventClient != nil {
		m.eventClient.Close()
		m.eventClient = nil
	}

	speaker, ok := m.speakers[ip]
	if !ok {
		// Try to add it - use longer timeout for speakers in standby
		var err error
		speaker, err = kefw2.NewSpeaker(ip, kefw2.WithTimeout(10*time.Second))
		if err != nil {
			return err
		}
		m.speakers[ip] = speaker
	}

	m.activeSpeaker = speaker

	// Start event client for this speaker
	eventClient, err := speaker.NewEventClient(
		kefw2.WithSubscriptions(kefw2.DefaultEventSubscriptions),
	)
	if err != nil {
		return err
	}

	m.eventClient = eventClient

	// Start listening for events in background
	eventCtx, cancel := context.WithCancel(context.Background())
	m.eventCancel = cancel
	go m.listenForEvents(eventCtx)

	return nil
}

// listenForEvents forwards speaker events to the callback
func (m *Manager) listenForEvents(ctx context.Context) {
	m.mu.RLock()
	client := m.eventClient
	m.mu.RUnlock()

	if client == nil {
		return
	}

	// Start the event client in a goroutine
	go func() {
		_ = client.Start(ctx)
	}()

	// Forward events
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-client.Events():
			if !ok {
				return
			}
			m.mu.RLock()
			cb := m.onEvent
			m.mu.RUnlock()

			if cb != nil {
				cb(event)
			}
		}
	}
}

// Close stops the manager and releases resources
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.eventCancel != nil {
		m.eventCancel()
		m.eventCancel = nil
	}
	if m.eventClient != nil {
		m.eventClient.Close()
		m.eventClient = nil
	}
}
