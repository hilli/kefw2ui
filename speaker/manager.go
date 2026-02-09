package speaker

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/hilli/go-kef-w2/kefw2"
)

// Manager handles speaker discovery and active speaker management.
type Manager struct {
	mu            sync.RWMutex
	speakers      map[string]*kefw2.KEFSpeaker
	activeSpeaker *kefw2.KEFSpeaker
	eventClient   *kefw2.EventClient
	eventCancel   context.CancelFunc

	// Event callbacks
	onEvent  func(event kefw2.Event)
	onHealth func(connected bool)

	// Speaker connectivity state
	speakerConnected bool

	// Standby awareness: when true, the reconnection loop pauses
	// to avoid waking the speaker with HTTP requests.
	speakerInStandby bool
	standbyWake      chan struct{} // closed when speaker should wake up
}

// NewManager creates a new speaker manager.
func NewManager() *Manager {
	return &Manager{
		speakers: make(map[string]*kefw2.KEFSpeaker),
	}
}

// SetEventCallback sets the callback for speaker events.
func (m *Manager) SetEventCallback(cb func(event kefw2.Event)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onEvent = cb
}

// SetHealthCallback sets the callback for speaker connectivity changes.
func (m *Manager) SetHealthCallback(cb func(connected bool)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onHealth = cb
}

// IsSpeakerConnected returns whether the active speaker is reachable.
func (m *Manager) IsSpeakerConnected() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.speakerConnected
}

// setSpeakerConnected updates connectivity state and fires the health callback.
func (m *Manager) setSpeakerConnected(connected bool) {
	m.mu.Lock()
	changed := m.speakerConnected != connected
	m.speakerConnected = connected
	cb := m.onHealth
	m.mu.Unlock()

	if changed && cb != nil {
		cb(connected)
	}
}

// NotifyStandby tells the manager the speaker has entered standby.
// The event reconnection loop will pause to avoid waking the speaker.
func (m *Manager) NotifyStandby() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.speakerInStandby {
		return // already in standby
	}
	m.speakerInStandby = true
	m.standbyWake = make(chan struct{})
	log.Printf("Speaker entered standby — pausing event reconnection")
}

// NotifyWake tells the manager the speaker is waking from standby.
// The event reconnection loop will resume.
func (m *Manager) NotifyWake() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.speakerInStandby {
		return // not in standby
	}
	m.speakerInStandby = false
	close(m.standbyWake) // unblock the reconnection loop
	log.Printf("Speaker waking from standby — resuming event reconnection")
}

// isInStandby returns the standby state and the wake channel.
func (m *Manager) isInStandby() (bool, chan struct{}) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.speakerInStandby, m.standbyWake
}

// IsInStandby returns whether the speaker is known to be in standby.
// Used by the server to avoid querying the speaker (which would wake it).
func (m *Manager) IsInStandby() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.speakerInStandby
}

// Discover finds speakers on the network using mDNS.
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

// AddSpeaker manually adds a speaker by IP address.
func (m *Manager) AddSpeaker(_ context.Context, ip string) (*kefw2.KEFSpeaker, error) {
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

// AddConfiguredSpeaker adds a speaker from config without connecting.
// This is used at startup to preload known speakers before discovery.
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

// GetSpeakers returns all known speakers.
func (m *Manager) GetSpeakers() []*kefw2.KEFSpeaker {
	m.mu.RLock()
	defer m.mu.RUnlock()

	speakers := make([]*kefw2.KEFSpeaker, 0, len(m.speakers))
	for _, s := range m.speakers {
		speakers = append(speakers, s)
	}
	return speakers
}

// GetActiveSpeaker returns the currently active speaker.
func (m *Manager) GetActiveSpeaker() *kefw2.KEFSpeaker {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.activeSpeaker
}

// SetActiveSpeaker sets the active speaker by IP address.
func (m *Manager) SetActiveSpeaker(_ context.Context, ip string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Stop existing event client
	if m.eventCancel != nil {
		m.eventCancel()
		m.eventCancel = nil
	}
	if m.eventClient != nil {
		_ = m.eventClient.Close()
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

	// Start listening for events in background (with reconnection support)
	eventCtx, cancel := context.WithCancel(context.Background())
	m.eventCancel = cancel
	go m.listenForEvents(eventCtx)

	// Mark speaker as connected
	m.speakerConnected = true
	if m.onHealth != nil {
		go m.onHealth(true)
	}

	return nil
}

// listenForEvents forwards speaker events to the callback, with automatic reconnection.
// When the event client disconnects (speaker offline, network error, etc.), it will:.
// 1. Notify via setSpeakerConnected(false)
// 2. Attempt to reconnect with exponential backoff (2s, 4s, 8s, 16s, max 30s)
// 3. On successful reconnect, notify via setSpeakerConnected(true) and resume event forwarding.
func (m *Manager) listenForEvents(ctx context.Context) {
	m.mu.RLock()
	client := m.eventClient
	speaker := m.activeSpeaker
	m.mu.RUnlock()

	if client == nil || speaker == nil {
		return
	}

	for {
		// Start the event client (blocks in a goroutine, closes Events() channel when done)
		startDone := make(chan error, 1)
		go func() {
			startDone <- client.Start(ctx)
		}()

		// Forward events until the channel closes
		eventsCh := client.Events()
	eventLoop:
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-eventsCh:
				if !ok {
					// Channel closed — event client died
					break eventLoop
				}
				m.mu.RLock()
				cb := m.onEvent
				m.mu.RUnlock()

				if cb != nil {
					cb(event)
				}
			}
		}

		// Wait for Start() to finish (it should have returned already)
		select {
		case <-ctx.Done():
			return
		case err := <-startDone:
			if err != nil {
				log.Printf("Event client stopped: %v", err)
			}
		}

		// Close the old client
		_ = client.Close()

		// Mark speaker as disconnected
		m.setSpeakerConnected(false)

		// Reconnection loop with exponential backoff
		backoff := 2 * time.Second
		const maxBackoff = 30 * time.Second

		for {
			// Check if speaker is in standby — if so, wait for wake signal
			// instead of hammering the speaker with reconnection attempts.
			if inStandby, wakeCh := m.isInStandby(); inStandby {
				log.Printf("Speaker is in standby — waiting for wake signal before reconnecting")
				select {
				case <-ctx.Done():
					return
				case <-wakeCh:
					log.Printf("Wake signal received — will attempt reconnection")
					// Reset backoff for fresh reconnection after wake
					backoff = 2 * time.Second
				}
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}

			log.Printf("Attempting to reconnect event client to %s...", speaker.IPAddress)

			newClient, err := speaker.NewEventClient(
				kefw2.WithSubscriptions(kefw2.DefaultEventSubscriptions),
			)
			if err != nil {
				log.Printf("Reconnect failed: %v (retrying in %v)", err, backoff)
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
				continue
			}

			// Reconnect succeeded — update state and resume
			log.Printf("Reconnected event client to %s", speaker.IPAddress)

			m.mu.Lock()
			m.eventClient = newClient
			m.mu.Unlock()

			client = newClient
			m.setSpeakerConnected(true)
			break // Break out of reconnection loop, continue outer loop to forward events
		}
	}
}

// Close stops the manager and releases resources.
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.eventCancel != nil {
		m.eventCancel()
		m.eventCancel = nil
	}
	if m.eventClient != nil {
		_ = m.eventClient.Close()
		m.eventClient = nil
	}
}
