// Package messaging provides the in-app real-time delivery layer.
//
// The Hub is a tiny pub-sub keyed by user UUID. When a message is persisted
// the conversation handler calls Hub.Publish to fan it out to every active
// websocket of the receiver (and, for echo, the sender). Clients that are
// offline or disconnected simply miss the live push and must re-fetch
// history via the REST endpoint -- which doubles as the polling fallback.
package messaging

import (
	"sync"

	"github.com/google/uuid"
)

// OutboundEvent is the JSON envelope pushed over the websocket. Keeping the
// envelope explicit (and not just the raw message) lets us extend with
// "typing", "read", etc. later without a breaking change.
type OutboundEvent struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

// Hub holds the set of subscriber channels for each connected user.
type Hub struct {
	mu          sync.RWMutex
	subscribers map[uuid.UUID]map[chan OutboundEvent]struct{}
}

var Default = NewHub()

func NewHub() *Hub {
	return &Hub{
		subscribers: make(map[uuid.UUID]map[chan OutboundEvent]struct{}),
	}
}

// Subscribe registers a new client channel for a user and returns the
// channel along with a cleanup function the caller must defer.
func (h *Hub) Subscribe(userID uuid.UUID) (chan OutboundEvent, func()) {
	ch := make(chan OutboundEvent, 32)

	h.mu.Lock()
	if _, ok := h.subscribers[userID]; !ok {
		h.subscribers[userID] = make(map[chan OutboundEvent]struct{})
	}
	h.subscribers[userID][ch] = struct{}{}
	h.mu.Unlock()

	cleanup := func() {
		h.mu.Lock()
		if subs, ok := h.subscribers[userID]; ok {
			delete(subs, ch)
			if len(subs) == 0 {
				delete(h.subscribers, userID)
			}
		}
		h.mu.Unlock()
		close(ch)
	}
	return ch, cleanup
}

// Publish delivers the event to every active connection for the given user.
// Slow consumers are skipped (channel full) rather than blocking the writer.
func (h *Hub) Publish(userID uuid.UUID, evt OutboundEvent) {
	h.mu.RLock()
	subs := h.subscribers[userID]
	for ch := range subs {
		select {
		case ch <- evt:
		default:
		}
	}
	h.mu.RUnlock()
}

// IsOnline reports whether at least one websocket is currently attached.
func (h *Hub) IsOnline(userID uuid.UUID) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	subs, ok := h.subscribers[userID]
	return ok && len(subs) > 0
}
