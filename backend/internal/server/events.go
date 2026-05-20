package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type Event struct {
	Type    string    `json:"type"`
	Time    time.Time `json:"time"`
	Payload any       `json:"payload,omitempty"`
}

type eventHub struct {
	mu      sync.Mutex
	clients map[chan Event]struct{}
	in      chan Event
}

func newEventHub() *eventHub {
	return &eventHub{
		clients: map[chan Event]struct{}{},
		in:      make(chan Event, 64),
	}
}

func (h *eventHub) publish(e Event) {
	select {
	case h.in <- e:
	default:
	}
}

func (h *eventHub) subscribe() chan Event {
	ch := make(chan Event, 16)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *eventHub) unsubscribe(ch chan Event) {
	h.mu.Lock()
	delete(h.clients, ch)
	h.mu.Unlock()
	close(ch)
}

func (h *eventHub) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case e := <-h.in:
			h.mu.Lock()
			for ch := range h.clients {
				select {
				case ch <- e:
				default:
				}
			}
			h.mu.Unlock()
		}
	}
}

// tickStatus emits a status heartbeat so SSE clients get a regular pulse and
// can show a live "last update" timestamp without polling.
func (s *Server) tickStatus(ctx context.Context) {
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			s.mu.RLock()
			cfg := s.Cfg
			eng := s.engine
			s.mu.RUnlock()
			st := Status{
				Running:    true,
				Bucket:     cfg.Bucket,
				Region:     cfg.Region,
				MachineID:  cfg.MachineID,
				LastSyncAt: now.UTC().Format(time.RFC3339),
			}
			if eng != nil {
				es := eng.Stats()
				st.Locked = eng.Locked()
				st.Paused = es.Paused
				st.QueueUploads = es.QueueUp
				st.QueueDownloads = es.QueueDown
				st.BytesUp = es.BytesUp
				st.BytesDown = es.BytesDown
				st.Conflicts = es.Conflicts
				if !es.LastRun.IsZero() {
					st.LastSyncAt = es.LastRun.Format(time.RFC3339)
				}
			}
			s.events.publish(Event{Type: "status", Time: now.UTC(), Payload: st})
		}
	}
}

// handleEvents implements SSE on /api/v1/events.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSONError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ch := s.events.subscribe()
	defer s.events.unsubscribe(ch)

	fmt.Fprintf(w, ": connected\n\n")
	flusher.Flush()

	ctx := r.Context()
	keepalive := time.NewTicker(20 * time.Second)
	defer keepalive.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-keepalive.C:
			fmt.Fprintf(w, ": ping\n\n")
			flusher.Flush()
		case e, ok := <-ch:
			if !ok {
				return
			}
			data, _ := json.Marshal(e)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", e.Type, data)
			flusher.Flush()
		}
	}
}
