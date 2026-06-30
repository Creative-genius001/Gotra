// Package gateway implements the Gotra tunnel gateway: it accepts agent
// WebSocket connections, registers tunnels, and forwards public HTTP traffic to
// the matching agent (Backend Bible — Gateway Architecture).
package gateway

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"encoding/json"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/acme/autocert"

	"github.com/gotra/gotra/internal/analytics"
	"github.com/gotra/gotra/internal/config"
	"github.com/gotra/gotra/internal/requests"
	"github.com/gotra/gotra/internal/tunnels"
	"github.com/gotra/gotra/pkg/cache"
	"github.com/gotra/gotra/pkg/database"
	"github.com/gotra/gotra/pkg/security"
	"github.com/gotra/gotra/pkg/tunnelproto"
)

// agentReadDeadline bounds silence from an agent; heartbeats (15s) keep it alive.
const agentReadDeadline = 60 * time.Second

// Server is the tunnel gateway.
type Server struct {
	cfg      *config.Config
	log      *slog.Logger
	registry *Registry
	tokens    *security.TokenManager
	tunnels   *tunnels.Repository
	captures  *requests.Repository
	analytics *analytics.Store
	upgrader  websocket.Upgrader
}

// New constructs a gateway Server. A non-nil cache enables multi-instance
// coordination (route ownership + cross-instance forwarding); nil runs the
// gateway as a single instance.
func New(cfg *config.Config, log *slog.Logger, db *database.DB, c *cache.Cache) *Server {
	var redisClient *redis.Client
	if c != nil {
		redisClient = c.Client
	}
	return &Server{
		cfg:       cfg,
		log:       log,
		registry:  NewRegistry(redisClient),
		tokens:    security.NewTokenManager(cfg.JWTSecret, cfg.AccessTokenTTL),
		tunnels:   tunnels.NewRepository(db.Pool),
		captures:  requests.NewRepository(db.Pool),
		analytics: analytics.Open(context.Background(), cfg.ClickHouseURL, log),
		// Agents are non-browser clients; origin checks do not apply.
		upgrader: websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }},
	}
}

// Run starts the gateway HTTP server until the context is cancelled.
func (s *Server) Run(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"status":"ok","service":"gotra-gateway"}`)
	})
	mux.HandleFunc("/ws/agent", s.handleAgent)
	mux.HandleFunc("/", s.handlePublic) // public tunnel traffic by Host subdomain

	// Multi-instance coordination: renew owned routes and serve peer forwards.
	if s.registry.Clustered() {
		go s.registry.refreshLoop(ctx)
		go s.runCluster(ctx)
		s.log.Info("gateway clustering enabled", "instance", s.registry.instanceID)
	}

	srv := &http.Server{
		Addr:              ":" + s.cfg.GatewayPort,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	serve := s.listenFunc(srv)

	errCh := make(chan error, 1)
	go func() {
		s.log.Info("gateway listening", "addr", srv.Addr, "base_domain", s.cfg.TunnelBaseDomain, "tls", srv.TLSConfig != nil || s.cfg.GatewayTLSCertFile != "")
		if err := serve(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		s.analytics.Close()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}

// listenFunc selects how the gateway serves: plain HTTP (dev), a static TLS
// certificate (e.g. a wildcard cert), or ACME autocert which issues a
// certificate per tunnel hostname on demand (Backend Bible — TLS termination).
func (s *Server) listenFunc(srv *http.Server) func() error {
	switch {
	case s.cfg.GatewayTLSCertFile != "" && s.cfg.GatewayTLSKeyFile != "":
		return func() error {
			return srv.ListenAndServeTLS(s.cfg.GatewayTLSCertFile, s.cfg.GatewayTLSKeyFile)
		}

	case s.cfg.GatewayAutocertEnabled:
		baseSuffix := "." + s.cfg.TunnelBaseDomain
		m := &autocert.Manager{
			Prompt: autocert.AcceptTOS,
			Cache:  autocert.DirCache(s.cfg.GatewayAutocertCacheDir),
			Email:  s.cfg.GatewayAutocertEmail,
			// Allow the base domain and any tunnel subdomain; ACME issues a
			// cert per host via the TLS-ALPN-01 challenge on first request.
			HostPolicy: func(_ context.Context, host string) error {
				if host == s.cfg.TunnelBaseDomain || strings.HasSuffix(host, baseSuffix) {
					return nil
				}
				return fmt.Errorf("autocert: host %q not permitted", host)
			},
		}
		srv.TLSConfig = m.TLSConfig()
		return func() error { return srv.ListenAndServeTLS("", "") }

	default:
		return srv.ListenAndServe
	}
}

// --- Agent control plane ----------------------------------------------------

func (s *Server) handleAgent(w http.ResponseWriter, r *http.Request) {
	ws, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return // Upgrade already wrote an error response.
	}

	conn, err := s.register(r.Context(), ws)
	if err != nil {
		_ = ws.WriteJSON(mustEnvelope(tunnelproto.TypeError, "", tunnelproto.ErrorPayload{Message: err.Error()}))
		_ = ws.Close()
		return
	}

	s.log.Info("tunnel registered", "tunnel_id", conn.tunnelID, "subdomain", conn.subdomain)
	s.readLoop(conn)
}

// register handles the REGISTER_TUNNEL handshake: validate token, authorize the
// project, create the tunnel row, and register the connection.
func (s *Server) register(ctx context.Context, ws *websocket.Conn) (*AgentConn, error) {
	_ = ws.SetReadDeadline(time.Now().Add(agentReadDeadline))
	var env tunnelproto.Envelope
	if err := ws.ReadJSON(&env); err != nil {
		return nil, fmt.Errorf("read register: %w", err)
	}
	if env.Type != tunnelproto.TypeRegisterTunnel {
		return nil, fmt.Errorf("expected REGISTER_TUNNEL, got %s", env.Type)
	}

	var p tunnelproto.RegisterTunnelPayload
	if err := env.Decode(&p); err != nil {
		return nil, fmt.Errorf("decode register: %w", err)
	}

	claims, err := s.tokens.ParseAccessToken(p.Token)
	if err != nil {
		return nil, fmt.Errorf("invalid token")
	}
	projectID, err := uuid.Parse(p.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("invalid project_id")
	}

	role, err := s.tunnels.ProjectRole(ctx, claims.UserID, projectID)
	if err != nil {
		return nil, fmt.Errorf("not a member of project")
	}
	if role == string(security.RoleViewer) {
		return nil, fmt.Errorf("viewers may not open tunnels")
	}
	if p.LocalPort <= 0 || p.LocalPort > 65535 {
		return nil, fmt.Errorf("invalid local port")
	}

	sub := s.newSubdomain()
	publicURL := fmt.Sprintf("https://%s.%s", sub, s.cfg.TunnelBaseDomain)

	tunnel, err := s.tunnels.Create(ctx, projectID, publicURL, p.LocalPort)
	if err != nil {
		return nil, fmt.Errorf("create tunnel: %w", err)
	}
	_ = s.tunnels.SetStatus(ctx, tunnel.ID, "active")

	conn := newAgentConn(ws, tunnel.ID, projectID, sub)
	s.registry.add(ctx, sub, conn)

	if err := conn.writeEnvelope(mustEnvelope(tunnelproto.TypeRegistered, "", tunnelproto.RegisteredPayload{
		TunnelID:  tunnel.ID.String(),
		PublicURL: publicURL,
	})); err != nil {
		s.registry.remove(ctx, sub, conn)
		return nil, err
	}
	return conn, nil
}

// readLoop pumps messages from the agent until the connection drops.
func (s *Server) readLoop(conn *AgentConn) {
	defer func() {
		conn.close()
		s.registry.remove(context.Background(), conn.subdomain, conn)
		_ = s.tunnels.SetStatus(context.Background(), conn.tunnelID, "disconnected")
		s.log.Info("tunnel disconnected", "tunnel_id", conn.tunnelID, "subdomain", conn.subdomain)
	}()

	for {
		_ = conn.ws.SetReadDeadline(time.Now().Add(agentReadDeadline))
		var env tunnelproto.Envelope
		if err := conn.ws.ReadJSON(&env); err != nil {
			return
		}
		switch env.Type {
		case tunnelproto.TypeResponseForward:
			var resp tunnelproto.ForwardedResponse
			if err := env.Decode(&resp); err == nil {
				conn.deliver(env.RequestID, &resp)
			}
		case tunnelproto.TypeHeartbeat:
			// Deadline already extended above.
		case tunnelproto.TypeDisconnect:
			return
		}
	}
}

// --- Public data plane ------------------------------------------------------

func (s *Server) handlePublic(w http.ResponseWriter, r *http.Request) {
	sub := s.subdomainFromHost(r.Host)
	if sub == "" {
		http.Error(w, "no tunnel host", http.StatusBadGateway)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 32<<20)) // 32 MiB cap
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}
	path := r.URL.Path
	if r.URL.RawQuery != "" {
		path += "?" + r.URL.RawQuery
	}
	fwReq := tunnelproto.ForwardedRequest{Method: r.Method, Path: path, Headers: r.Header, Body: body}

	var resp *tunnelproto.ForwardedResponse

	if conn, ok := s.registry.localLookup(sub); ok {
		// The agent is connected to this instance — serve and capture here.
		resp = s.serveLocally(conn, fwReq)
	} else {
		// Find the owning instance and forward across the cluster.
		owner, err := s.registry.ownerInstance(r.Context(), sub)
		if err != nil || owner == "" {
			http.Error(w, "no active tunnel for "+sub, http.StatusBadGateway)
			return
		}
		resp, err = s.registry.forwardRemote(r.Context(), owner, sub, fwReq)
		if err != nil {
			http.Error(w, "tunnel error: "+err.Error(), http.StatusBadGateway)
			return
		}
	}

	for k, vs := range resp.Headers {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.Status)
	_, _ = w.Write(resp.Body)
}

// serveLocally forwards a request to a locally-connected agent, captures the
// exchange (request/response store + analytics), and returns the response. It is
// used both for direct public requests and for forwards received from peers.
func (s *Server) serveLocally(conn *AgentConn, fwReq tunnelproto.ForwardedRequest) *tunnelproto.ForwardedResponse {
	ctx, cancel := context.WithTimeout(context.Background(), forwardTimeout)
	defer cancel()

	start := time.Now()
	resp, err := conn.forward(ctx, &fwReq)
	if err != nil {
		return &tunnelproto.ForwardedResponse{
			Status:  http.StatusBadGateway,
			Headers: map[string][]string{"Content-Type": {"text/plain; charset=utf-8"}},
			Body:    []byte("tunnel error: " + err.Error()),
		}
	}
	durationMs := int(time.Since(start).Milliseconds())
	s.capture(conn, fwReq, resp, durationMs)
	return resp
}

// capture persists a request/response pair and mirrors it to analytics. Failures
// never affect the proxied response.
func (s *Server) capture(conn *AgentConn, fwReq tunnelproto.ForwardedRequest, resp *tunnelproto.ForwardedResponse, durationMs int) {
	path, query := fwReq.Path, ""
	if i := strings.IndexByte(path, '?'); i >= 0 {
		path, query = path[:i], path[i+1:]
	}

	c := requests.Capture{
		TunnelID:    conn.tunnelID,
		ProjectID:   conn.projectID,
		Method:      fwReq.Method,
		Path:        path,
		Query:       query,
		ReqHeaders:  fwReq.Headers,
		ReqBody:     fwReq.Body,
		Status:      resp.Status,
		RespHeaders: resp.Headers,
		RespBody:    resp.Body,
		DurationMs:  durationMs,
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := s.captures.SaveCapture(ctx, c); err != nil {
			s.log.Warn("capture failed", "tunnel_id", conn.tunnelID, "error", err)
		}
	}()

	s.analytics.Record(analytics.Event{
		ProjectID:  conn.projectID,
		TunnelID:   conn.tunnelID,
		Method:     fwReq.Method,
		Path:       path,
		Status:     resp.Status,
		DurationMs: durationMs,
		ReceivedAt: time.Now(),
	})
}

// --- Cluster control plane --------------------------------------------------

// runCluster subscribes to this instance's RPC and reply channels and dispatches
// cross-instance forward requests/responses.
func (s *Server) runCluster(ctx context.Context) {
	rpcChan := rpcChanPrefix + s.registry.instanceID
	replyChan := replyChanPrefix + s.registry.instanceID
	sub := s.registry.redis.Subscribe(ctx, rpcChan, replyChan)
	defer sub.Close()

	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			switch msg.Channel {
			case rpcChan:
				go s.handleClusterRequest([]byte(msg.Payload))
			case replyChan:
				var cr clusterResponse
				if err := json.Unmarshal([]byte(msg.Payload), &cr); err == nil {
					s.registry.deliverReply(cr)
				}
			}
		}
	}
}

// handleClusterRequest serves a forward request originating from a peer instance.
func (s *Server) handleClusterRequest(payload []byte) {
	var req clusterRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return
	}

	reply := clusterResponse{RequestID: req.RequestID}
	if conn, ok := s.registry.localLookup(req.Subdomain); ok {
		reply.Resp = s.serveLocally(conn, req.Req)
	} else {
		reply.Error = "no active tunnel for " + req.Subdomain
	}

	out, _ := json.Marshal(reply)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.registry.redis.Publish(ctx, replyChanPrefix+req.Origin, out).Err()
}

// subdomainFromHost extracts the tunnel subdomain from the request Host.
func (s *Server) subdomainFromHost(host string) string {
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	suffix := "." + s.cfg.TunnelBaseDomain
	if strings.HasSuffix(host, suffix) {
		return strings.TrimSuffix(host, suffix)
	}
	return ""
}

// newSubdomain mints a random lowercase-alphanumeric subdomain label.
func (s *Server) newSubdomain() string {
	tok, err := security.GenerateOpaqueToken(6)
	if err != nil {
		return uuid.NewString()[:12]
	}
	return strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(tok))
}

func mustEnvelope(t tunnelproto.MessageType, requestID string, payload any) tunnelproto.Envelope {
	env, _ := tunnelproto.NewEnvelope(t, requestID, payload)
	return env
}
