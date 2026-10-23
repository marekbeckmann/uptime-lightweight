package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"uptime-mon/internal/prober"
)

// SSEBroker broadcasts server-sent events to connected dashboard clients
type SSEBroker struct {
	clients    map[chan []byte]bool
	addClient  chan chan []byte
	remClient  chan chan []byte
	broadcast  chan []byte
	mu         sync.Mutex
}

func NewSSEBroker() *SSEBroker {
	b := &SSEBroker{
		clients:   make(map[chan []byte]bool),
		addClient: make(chan chan []byte),
		remClient: make(chan chan []byte),
		broadcast: make(chan []byte, 100),
	}
	go b.run()
	return b
}

func (b *SSEBroker) run() {
	for {
		select {
		case ch := <-b.addClient:
			b.mu.Lock()
			b.clients[ch] = true
			b.mu.Unlock()
		case ch := <-b.remClient:
			b.mu.Lock()
			if _, ok := b.clients[ch]; ok {
				delete(b.clients, ch)
				close(ch)
			}
			b.mu.Unlock()
		case msg := <-b.broadcast:
			b.mu.Lock()
			for ch := range b.clients {
				select {
				case ch <- msg:
				default:
					// Slow client dropped
				}
			}
			b.mu.Unlock()
		}
	}
}

// BroadcastEvent publishes an event with an SSE event name and data payload
func (b *SSEBroker) BroadcastEvent(event string, data interface{}) {
	raw, err := json.Marshal(data)
	if err != nil {
		log.Printf("[sse] json marshal error: %v", err)
		return
	}
	msg := []byte(fmt.Sprintf("event: %s\ndata: %s\n\n", event, string(raw)))
	b.broadcast <- msg
}

// HandleEvents streams SSE events to the browser
func (b *SSEBroker) HandleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	clientChan := make(chan []byte, 20)
	b.addClient <- clientChan
	defer func() {
		b.remClient <- clientChan
	}()

	// Send initial ping event
	w.Write([]byte("event: connected\ndata: {}\n\n"))
	flusher.Flush()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-clientChan:
			if !ok {
				return
			}
			w.Write(msg)
			flusher.Flush()
		}
	}
}

// AttachProberEvents hooks scheduler events into SSE broadcast
func (b *SSEBroker) AttachProberEvents(s *prober.Scheduler) {
	s.OnEvent(func(evt prober.TransitionEvent) {
		if evt.Type != "HEARTBEAT" {
			return
		}
		probeStatus := evt.ProbeStatus
		if probeStatus == "" {
			probeStatus = evt.CurrentStatus
		}
		payload := map[string]interface{}{
			"endpoint_id":  evt.EndpointID,
			"status":       evt.CurrentStatus,
			"probe_status": probeStatus,
			"latency_ms":   evt.LatencyMS,
			"timestamp":    evt.Timestamp,
			"error":        evt.LastError,
		}
		b.BroadcastEvent("heartbeat", payload)
	})
}
