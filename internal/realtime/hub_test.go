package realtime

import (
	"context"
	"net"
	"net/http"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/kachofugetsu09/nemeton/internal/runner"
	"github.com/kachofugetsu09/nemeton/internal/store"
)

type programmaticEventSource struct {
	mu     sync.Mutex
	events []store.StreamEvent
}

func (s *programmaticEventSource) EventsAfter(_ context.Context, _ string, after int64) ([]store.StreamEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var result []store.StreamEvent
	for _, item := range s.events {
		if item.Sequence > after {
			result = append(result, item)
		}
	}
	return result, nil
}

func (s *programmaticEventSource) append(item store.StreamEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, item)
}

func TestWebSocketBacklogLiveDeltaAndResumeOverUnixSocket(t *testing.T) {
	meetingID := "00000000-0000-4000-8000-000000000001"
	source := &programmaticEventSource{events: []store.StreamEvent{
		{Sequence: 1, EventID: "event-1", EventType: "one"},
		{Sequence: 2, EventID: "event-2", EventType: "two"},
	}}
	hub := NewHub()
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(response http.ResponseWriter, request *http.Request) {
		_ = hub.Serve(source, response, request, meetingID)
	})
	socket := filepath.Join(t.TempDir(), "hub.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatalf("listen on Unix Socket: %v", err)
	}
	server := &http.Server{Handler: mux}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() {
		_ = server.Shutdown(context.Background())
		_ = listener.Close()
	})

	connection := dialTestWebSocket(t, socket, "ws://nemetond/ws?after_sequence=1")
	message := readTestMessage(t, connection)
	if message.Sequence != 2 || message.Type != "domain_event" {
		t.Fatalf("backlog message = %#v", message)
	}
	secondConnection := dialTestWebSocket(t, socket, "ws://nemetond/ws?after_sequence=2")
	third := store.StreamEvent{Sequence: 3, EventID: "event-3", EventType: "three"}
	source.append(third)
	hub.PublishEvent(meetingID, third)
	message = readTestMessage(t, connection)
	if message.Sequence != 3 || message.Event.EventID != "event-3" {
		t.Fatalf("live message = %#v", message)
	}
	secondMessage := readTestMessage(t, secondConnection)
	if secondMessage.Sequence != 3 || secondMessage.Event.EventID != "event-3" {
		t.Fatalf("second live message = %#v", secondMessage)
	}
	_ = secondConnection.Close()
	hub.PublishDelta(meetingID, "participant", runner.Delta{Provider: "codex", Type: "text", Content: "live"})
	message = readTestMessage(t, connection)
	if message.Type != "runner_delta" || message.Sequence != 0 || message.Delta.Content != "live" {
		t.Fatalf("delta message = %#v", message)
	}
	_ = connection.Close()

	reconnected := dialTestWebSocket(t, socket, "ws://nemetond/ws?after_sequence=2")
	defer reconnected.Close()
	message = readTestMessage(t, reconnected)
	if message.Sequence != 3 {
		t.Fatalf("resumed message = %#v", message)
	}
}

func dialTestWebSocket(t *testing.T, socket, target string) *websocket.Conn {
	t.Helper()
	dialer := websocket.Dialer{NetDialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socket)
	}}
	connection, _, err := dialer.Dial(target, nil)
	if err != nil {
		t.Fatalf("dial WebSocket: %v", err)
	}
	return connection
}

func readTestMessage(t *testing.T, connection *websocket.Conn) Message {
	t.Helper()
	connection.SetReadDeadline(time.Now().Add(5 * time.Second))
	var message Message
	if err := connection.ReadJSON(&message); err != nil {
		t.Fatalf("read WebSocket message: %v", err)
	}
	return message
}
