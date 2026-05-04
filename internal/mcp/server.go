package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"sync"
)

const jsonrpcVersion = "2.0"

type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data,omitempty"`
}

type Notification struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type Server struct {
	reader   *bufio.Reader
	writer   io.Writer
	handlers map[string]HandlerFunc
	mu       sync.Mutex
}

type HandlerFunc func(params json.RawMessage) (interface{}, error)

func NewServer(input io.Reader, output io.Writer) *Server {
	return &Server{
		reader:   bufio.NewReader(input),
		writer:   output,
		handlers: make(map[string]HandlerFunc),
	}
}

func (s *Server) Register(method string, handler HandlerFunc) {
	s.handlers[method] = handler
}

func (s *Server) Serve() error {
	for {
		line, err := s.reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("read error: %w", err)
		}

		if len(line) == 0 {
			continue
		}

		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			slog.Warn("mcp: invalid request", "error", err)
			continue
		}

		if req.ID == nil {
			s.handleNotification(&req)
			continue
		}

		resp := s.handleRequest(&req)
		s.sendResponse(resp)
	}
}

func (s *Server) handleRequest(req *Request) *Response {
	handler, ok := s.handlers[req.Method]
	if !ok {
		return &Response{
			JSONRPC: jsonrpcVersion,
			ID:      req.ID,
			Error: &RPCError{
				Code:    -32601,
				Message: fmt.Sprintf("method not found: %s", req.Method),
			},
		}
	}

	result, err := handler(req.Params)
	if err != nil {
		return &Response{
			JSONRPC: jsonrpcVersion,
			ID:      req.ID,
			Error: &RPCError{
				Code:    -32000,
				Message: err.Error(),
			},
		}
	}

	return &Response{
		JSONRPC: jsonrpcVersion,
		ID:      req.ID,
		Result:  result,
	}
}

func (s *Server) handleNotification(req *Request) {
	if handler, ok := s.handlers[req.Method]; ok {
		handler(req.Params)
	}
}

func (s *Server) sendResponse(resp *Response) {
	data, err := json.Marshal(resp)
	if err != nil {
		slog.Error("mcp: marshal response", "error", err)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.writer.Write(data)
	s.writer.Write([]byte{'\n'})
}

func (s *Server) Notify(method string, params interface{}) {
	notif := Notification{
		JSONRPC: jsonrpcVersion,
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(notif)
	if err != nil {
		slog.Error("mcp: marshal notification", "error", err)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.writer.Write(data)
	s.writer.Write([]byte{'\n'})
}
