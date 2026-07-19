package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/kachofugetsu09/nemeton/internal/runner"
	"github.com/kachofugetsu09/nemeton/internal/store"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = 45 * time.Second
	queueSize  = 256
)

type EventSource interface {
	EventsAfter(context.Context, string, int64) ([]store.StreamEvent, error)
}

type Message struct {
	Type          string             `json:"type"`
	MeetingID     string             `json:"meeting_id"`
	Sequence      int64              `json:"sequence"`
	Event         *store.StreamEvent `json:"event,omitempty"`
	ParticipantID string             `json:"participant_id,omitempty"`
	Delta         *runner.Delta      `json:"delta,omitempty"`
	Error         string             `json:"error,omitempty"`
}

type client struct {
	connection *websocket.Conn
	send       chan []byte
	meetingID  string
}

type Hub struct {
	mu       sync.Mutex
	rooms    map[string]map[*client]struct{}
	upgrader websocket.Upgrader
}

func NewHub() *Hub {
	return &Hub{rooms: make(map[string]map[*client]struct{}), upgrader: websocket.Upgrader{
		CheckOrigin: func(request *http.Request) bool { return request.Header.Get("Origin") == "" },
	}}
}

func (h *Hub) PublishEvent(meetingID string, item store.StreamEvent) {
	h.publish(meetingID, Message{Type: "domain_event", MeetingID: meetingID,
		Sequence: item.Sequence, Event: &item})
}

func (h *Hub) PublishDelta(meetingID, participantID string, delta runner.Delta) {
	h.publish(meetingID, Message{Type: "runner_delta", MeetingID: meetingID,
		ParticipantID: participantID, Delta: &delta})
}

func (h *Hub) publish(meetingID string, message Message) {
	data, err := json.Marshal(message)
	if err != nil {
		panic(fmt.Sprintf("encode internal WebSocket message: %v", err))
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for receiver := range h.rooms[meetingID] {
		select {
		case receiver.send <- data:
		default:
			h.dropLocked(receiver)
		}
	}
}

func (h *Hub) Serve(source EventSource, response http.ResponseWriter, request *http.Request, meetingID string) error {
	after, err := parseAfterSequence(request.URL.Query().Get("after_sequence"))
	if err != nil {
		return err
	}
	connection, err := h.upgrader.Upgrade(response, request, nil)
	if err != nil {
		return fmt.Errorf("upgrade WebSocket: %w", err)
	}
	// Register only after enqueuing the durable backlog while holding the Hub
	// lock, so a commit can appear either in the backlog or live queue, never in
	// the gap between them.
	h.mu.Lock()
	backlog, err := source.EventsAfter(request.Context(), meetingID, after)
	if err != nil {
		h.mu.Unlock()
		connection.Close()
		return err
	}
	receiver := &client{connection: connection,
		send: make(chan []byte, len(backlog)+queueSize), meetingID: meetingID}
	for _, item := range backlog {
		message := Message{Type: "domain_event", MeetingID: meetingID,
			Sequence: item.Sequence, Event: &item}
		data, err := json.Marshal(message)
		if err != nil {
			h.mu.Unlock()
			connection.Close()
			return fmt.Errorf("encode WebSocket backlog: %w", err)
		}
		receiver.send <- data
	}
	if h.rooms[meetingID] == nil {
		h.rooms[meetingID] = make(map[*client]struct{})
	}
	h.rooms[meetingID][receiver] = struct{}{}
	h.mu.Unlock()

	go h.writePump(receiver)
	h.readPump(receiver)
	return nil
}

func (h *Hub) writePump(receiver *client) {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()
	for {
		select {
		case data, ok := <-receiver.send:
			receiver.connection.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = receiver.connection.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := receiver.connection.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		case <-ticker.C:
			receiver.connection.SetWriteDeadline(time.Now().Add(writeWait))
			if err := receiver.connection.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (h *Hub) readPump(receiver *client) {
	defer func() {
		h.mu.Lock()
		h.dropLocked(receiver)
		h.mu.Unlock()
	}()
	receiver.connection.SetReadLimit(1024)
	receiver.connection.SetReadDeadline(time.Now().Add(pongWait))
	receiver.connection.SetPongHandler(func(string) error {
		return receiver.connection.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		if _, _, err := receiver.connection.ReadMessage(); err != nil {
			return
		}
	}
}

func (h *Hub) dropLocked(receiver *client) {
	room := h.rooms[receiver.meetingID]
	if _, exists := room[receiver]; !exists {
		return
	}
	delete(room, receiver)
	close(receiver.send)
	_ = receiver.connection.Close()
	if len(room) == 0 {
		delete(h.rooms, receiver.meetingID)
	}
}

func parseAfterSequence(value string) (int64, error) {
	if value == "" {
		return 0, nil
	}
	after, err := strconv.ParseInt(value, 10, 64)
	if err != nil || after < 0 {
		return 0, fmt.Errorf("after_sequence must be a non-negative integer")
	}
	return after, nil
}
