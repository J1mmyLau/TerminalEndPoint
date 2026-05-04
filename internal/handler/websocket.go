package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/J1mmyLau/TerminalEndPoint/internal/session"
	"github.com/J1mmyLau/TerminalEndPoint/pkg/protocol"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (h *Handler) WebSocket(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s, err := h.manager.Get(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("ws upgrade failed", "session", id, "error", err)
		return
	}
	defer conn.Close()

	subID := fmt.Sprintf("ws-%d", time.Now().UnixNano())
	eventCh := make(chan session.Event, 64)
	s.Subscribe(subID, eventCh)
	defer s.Unsubscribe(subID)

	sinceSeq := parseSinceSeq(r)
	if sinceSeq > 0 {
		entries := s.OutputSince(sinceSeq, 0)
		history := make([]protocol.WSOutput, 0, len(entries))
		for _, e := range entries {
			history = append(history, protocol.WSOutput{
				Stream: "stdout",
				Data:   e.Data,
				Seq:    e.Seq,
			})
		}
		conn.WriteJSON(protocol.WSMessage{
			Type: "history",
			Data: protocol.WSHistory{
				Entries: history,
				NextSeq: s.LatestSeq() + 1,
			},
		})
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			h.handleWSMessage(s, msg)
		}
	}()

	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

loop:
	for {
		select {
		case event, ok := <-eventCh:
			if !ok {
				break loop
			}
			if err := conn.WriteJSON(wsEventToMessage(event)); err != nil {
				slog.Warn("ws write error", "session", id, "error", err)
				break loop
			}
		case <-done:
			break loop
		case <-pingTicker.C:
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				break loop
			}
		}
	}
}

func (h *Handler) handleWSMessage(s *session.Session, msg []byte) {
	var envelope struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(msg, &envelope); err != nil {
		return
	}

	switch envelope.Type {
	case "write":
		var data struct {
			Data string `json:"data"`
		}
		if err := json.Unmarshal(msg, &data); err != nil {
			return
		}
		if data.Data != "" {
			s.Write([]byte(data.Data))
		}
	case "resize":
		var data struct {
			Cols uint16 `json:"cols"`
			Rows uint16 `json:"rows"`
		}
		if err := json.Unmarshal(msg, &data); err != nil {
			return
		}
		s.Resize(data.Rows, data.Cols)
	case "signal":
		var data struct {
			Signal string `json:"signal"`
		}
		if err := json.Unmarshal(msg, &data); err != nil {
			return
		}
		sig := parseSignal(data.Signal)
		if sig != nil {
			s.Signal(sig)
		}
	}
}

func wsEventToMessage(event session.Event) protocol.WSMessage {
	switch event.Type {
	case session.EventOutput:
		var data session.OutputData
		json.Unmarshal(event.Data, &data)
		return protocol.WSMessage{
			Type: "output",
			Data: protocol.WSOutput{
				Stream: data.Stream,
				Data:   data.Data,
				Seq:    event.Seq,
			},
		}
	case session.EventExit:
		var data session.ExitData
		json.Unmarshal(event.Data, &data)
		return protocol.WSMessage{
			Type: "exit",
			Data: protocol.WSExit{Code: data.Code},
		}
	case session.EventError:
		var data session.ErrorData
		json.Unmarshal(event.Data, &data)
		return protocol.WSMessage{
			Type: "error",
			Data: protocol.WSError{Message: data.Message},
		}
	default:
		return protocol.WSMessage{Type: "unknown"}
	}
}

func parseSinceSeq(r *http.Request) uint64 {
	s := r.URL.Query().Get("since_seq")
	if s == "" {
		return 0
	}
	seq, _ := strconv.ParseUint(s, 10, 64)
	return seq
}
