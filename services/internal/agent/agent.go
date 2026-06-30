// Package agent implements the Gotra CLI agent's tunneling client: it connects
// to the gateway over WebSocket, registers a tunnel, and forwards incoming
// public requests to the developer's local application (Backend Bible — Agent
// Architecture).
package agent

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/gotra/gotra/pkg/tunnelproto"
)

// Config configures the agent.
type Config struct {
	GatewayURL string // e.g. ws://localhost:8081/ws/agent
	Token      string // user access token (JWT)
	ProjectID  string
	LocalPort  int
}

// heartbeatInterval matches the Backend Bible's 15s heartbeat.
const heartbeatInterval = 15 * time.Second

// Run connects and serves until ctx is cancelled, reconnecting with backoff.
func Run(ctx context.Context, cfg Config) error {
	backoff := time.Second
	for {
		err := connectAndServe(ctx, cfg)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		fmt.Printf("gotra: connection lost (%v); reconnecting in %s\n", err, backoff)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

// conn wraps the agent WebSocket with a write mutex (multiple goroutines write).
type conn struct {
	ws        *websocket.Conn
	writeMu   sync.Mutex
	localPort int
	client    *http.Client
}

func connectAndServe(ctx context.Context, cfg Config) error {
	ws, _, err := websocket.DefaultDialer.DialContext(ctx, cfg.GatewayURL, nil)
	if err != nil {
		return fmt.Errorf("dial gateway: %w", err)
	}
	defer ws.Close()

	c := &conn{ws: ws, localPort: cfg.LocalPort, client: &http.Client{Timeout: 30 * time.Second}}

	// Closing the socket on cancellation unblocks the blocking ReadJSON below,
	// letting the agent shut down promptly on SIGINT/SIGTERM.
	go func() {
		<-ctx.Done()
		dc, _ := tunnelproto.NewEnvelope(tunnelproto.TypeDisconnect, "", nil)
		_ = c.write(dc)
		_ = ws.Close()
	}()

	// Handshake: REGISTER_TUNNEL → REGISTERED.
	reg, err := tunnelproto.NewEnvelope(tunnelproto.TypeRegisterTunnel, "", tunnelproto.RegisterTunnelPayload{
		Token:     cfg.Token,
		ProjectID: cfg.ProjectID,
		LocalPort: cfg.LocalPort,
	})
	if err != nil {
		return err
	}
	if err := c.write(reg); err != nil {
		return err
	}

	var env tunnelproto.Envelope
	if err := ws.ReadJSON(&env); err != nil {
		return fmt.Errorf("read registration reply: %w", err)
	}
	if env.Type == tunnelproto.TypeError {
		var e tunnelproto.ErrorPayload
		_ = env.Decode(&e)
		return fmt.Errorf("gateway rejected tunnel: %s", e.Message)
	}
	if env.Type != tunnelproto.TypeRegistered {
		return fmt.Errorf("unexpected reply: %s", env.Type)
	}
	var repaid tunnelproto.RegisteredPayload
	_ = env.Decode(&repaid)
	fmt.Printf("\n  Tunnel online ✓\n  %s  →  http://localhost:%d\n\n", repaid.PublicURL, cfg.LocalPort)

	// Heartbeat loop.
	hbCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go c.heartbeat(hbCtx)

	// Read loop: forward each request concurrently.
	for {
		var msg tunnelproto.Envelope
		if err := ws.ReadJSON(&msg); err != nil {
			return fmt.Errorf("read: %w", err)
		}
		if msg.Type == tunnelproto.TypeRequestForward {
			go c.handleRequest(msg)
		}
	}
}

func (c *conn) write(env tunnelproto.Envelope) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.ws.WriteJSON(env)
}

func (c *conn) heartbeat(ctx context.Context) {
	t := time.NewTicker(heartbeatInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			env, _ := tunnelproto.NewEnvelope(tunnelproto.TypeHeartbeat, "", nil)
			if err := c.write(env); err != nil {
				return
			}
		}
	}
}

// handleRequest replays a forwarded request against the local app and returns
// the response to the gateway.
func (c *conn) handleRequest(msg tunnelproto.Envelope) {
	var fr tunnelproto.ForwardedRequest
	if err := msg.Decode(&fr); err != nil {
		return
	}

	resp := c.proxyLocal(&fr)
	env, err := tunnelproto.NewEnvelope(tunnelproto.TypeResponseForward, msg.RequestID, resp)
	if err != nil {
		return
	}
	_ = c.write(env)
}

func (c *conn) proxyLocal(fr *tunnelproto.ForwardedRequest) *tunnelproto.ForwardedResponse {
	url := fmt.Sprintf("http://localhost:%d%s", c.localPort, fr.Path)
	req, err := http.NewRequest(fr.Method, url, bytes.NewReader(fr.Body))
	if err != nil {
		return errorResponse(fmt.Sprintf("agent: build request: %v", err))
	}
	for k, vs := range fr.Headers {
		if isHopByHop(k) {
			continue
		}
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}

	res, err := c.client.Do(req)
	if err != nil {
		return errorResponse(fmt.Sprintf("agent: local app unreachable on port %d: %v", c.localPort, err))
	}
	defer res.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(res.Body, 32<<20))
	headers := make(map[string][]string, len(res.Header))
	for k, vs := range res.Header {
		if isHopByHop(k) {
			continue
		}
		headers[k] = vs
	}
	return &tunnelproto.ForwardedResponse{Status: res.StatusCode, Headers: headers, Body: body}
}

func errorResponse(msg string) *tunnelproto.ForwardedResponse {
	return &tunnelproto.ForwardedResponse{
		Status:  http.StatusBadGateway,
		Headers: map[string][]string{"Content-Type": {"text/plain; charset=utf-8"}},
		Body:    []byte(msg),
	}
}

// isHopByHop reports whether a header is connection-specific and must not be
// forwarded across the tunnel.
func isHopByHop(h string) bool {
	switch strings.ToLower(h) {
	case "connection", "keep-alive", "proxy-authenticate", "proxy-authorization",
		"te", "trailer", "transfer-encoding", "upgrade":
		return true
	}
	return false
}
