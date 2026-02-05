package speaker

import (
	"context"
	"sync"

	"github.com/hilli/go-kef-w2/kefw2"
)

// Manager handles speaker discovery and active speaker management
type Manager struct {
	mu            sync.RWMutex
	speakers      map[string]*kefw2.KEFSpeaker
	activeSpeaker *kefw2.KEFSpeaker
	eventClient   *kefw2.EventClient

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
	speakers, err := kefw2.DiscoverSpeakers(ctx)
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
	speaker, err := kefw2.NewSpeaker(ip)
	if err != nil {
		return nil, err
	}

	// Verify speaker is reachable by getting info
	if err := speaker.Update(ctx); err != nil {
		return nil, err
	}

	m.mu.Lock()
	m.speakers[ip] = speaker
	m.mu.Unlock()

	return speaker, nil
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
	if m.eventClient != nil {
		m.eventClient.Stop()
		m.eventClient = nil
	}

	speaker, ok := m.speakers[ip]
	if !ok {
		// Try to add it
		var err error
		speaker, err = kefw2.NewSpeaker(ip)
		if err != nil {
			return err
		}
		if err := speaker.Update(ctx); err != nil {
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
	go m.listenForEvents(ctx)

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

	// Start the event client
	go client.Start(ctx)

	// Forward events
	for event := range client.Events() {
		m.mu.RLock()
		cb := m.onEvent
		m.mu.RUnlock()

		if cb != nil {
			cb(event)
		}
	}
}

// Close stops the manager and releases resources
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.eventClient != nil {
		m.eventClient.Stop()
		m.eventClient = nil
	}
}
