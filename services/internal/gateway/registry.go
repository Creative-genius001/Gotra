package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"

	"github.com/gotra/gotra/pkg/tunnelproto"
)

// forwardTimeout bounds how long the gateway waits for a response (local or
// cross-instance) before giving up.
const forwardTimeout = 30 * time.Second

// Route ownership keys + channels for multi-instance coordination.
const (
	routeKeyPrefix = "gw:route:"      // gw:route:{sub} -> instance id
	rpcChanPrefix  = "gw:rpc:"        // gw:rpc:{instance} -> incoming forward requests
	replyChanPrefix = "gw:reply:"     // gw:reply:{instance} -> forward responses
	routeTTL        = 90 * time.Second
	routeRefresh    = 30 * time.Second
)

// ErrNoTunnel is returned when no agent is connected for a subdomain.
var ErrNoTunnel = errors.New("gateway: no active tunnel for host")

// ErrForwardTimeout is returned when the agent does not respond in time.
var ErrForwardTimeout = errors.New("gateway: timed out waiting for agent")

// AgentConn represents one connected agent and multiplexes concurrent public
// requests over its single WebSocket using per-request correlation IDs.
type AgentConn struct {
	ws        *websocket.Conn
	writeMu   sync.Mutex
	pending   sync.Map // requestID -> chan *tunnelproto.ForwardedResponse
	tunnelID  uuid.UUID
	projectID uuid.UUID
	subdomain string
	closeOnce sync.Once
	closed    chan struct{}
}

func newAgentConn(ws *websocket.Conn, tunnelID, projectID uuid.UUID, subdomain string) *AgentConn {
	return &AgentConn{ws: ws, tunnelID: tunnelID, projectID: projectID, subdomain: subdomain, closed: make(chan struct{})}
}

func (a *AgentConn) writeEnvelope(env tunnelproto.Envelope) error {
	a.writeMu.Lock()
	defer a.writeMu.Unlock()
	return a.ws.WriteJSON(env)
}

// forward sends a request to the agent and waits for the correlated response.
func (a *AgentConn) forward(ctx context.Context, req *tunnelproto.ForwardedRequest) (*tunnelproto.ForwardedResponse, error) {
	requestID := uuid.NewString()
	respCh := make(chan *tunnelproto.ForwardedResponse, 1)
	a.pending.Store(requestID, respCh)
	defer a.pending.Delete(requestID)

	env, err := tunnelproto.NewEnvelope(tunnelproto.TypeRequestForward, requestID, req)
	if err != nil {
		return nil, err
	}
	if err := a.writeEnvelope(env); err != nil {
		return nil, err
	}

	select {
	case resp := <-respCh:
		return resp, nil
	case <-time.After(forwardTimeout):
		return nil, ErrForwardTimeout
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-a.closed:
		return nil, ErrNoTunnel
	}
}

func (a *AgentConn) deliver(requestID string, resp *tunnelproto.ForwardedResponse) {
	if ch, ok := a.pending.Load(requestID); ok {
		select {
		case ch.(chan *tunnelproto.ForwardedResponse) <- resp:
		default:
		}
	}
}

func (a *AgentConn) close() {
	a.closeOnce.Do(func() {
		close(a.closed)
		_ = a.ws.Close()
	})
}

// clusterRequest is a forward request sent to the gateway instance that owns a
// tunnel (its agent is connected there).
type clusterRequest struct {
	RequestID string                       `json:"request_id"`
	Origin    string                       `json:"origin"`
	Subdomain string                       `json:"subdomain"`
	Req       tunnelproto.ForwardedRequest `json:"req"`
}

// clusterResponse is the reply to a clusterRequest.
type clusterResponse struct {
	RequestID string                        `json:"request_id"`
	Resp      *tunnelproto.ForwardedResponse `json:"resp,omitempty"`
	Error     string                        `json:"error,omitempty"`
}

// Registry tracks connected agents. With a Redis client it coordinates route
// ownership across gateway instances and forwards requests to the owner; without
// one it is a single-instance in-memory registry.
type Registry struct {
	instanceID string
	redis      *redis.Client

	mu    sync.RWMutex
	bySub map[string]*AgentConn

	pending sync.Map // cluster requestID -> chan clusterResponse
}

// NewRegistry constructs a Registry. redis may be nil for single-instance mode.
func NewRegistry(redis *redis.Client) *Registry {
	return &Registry{
		instanceID: uuid.NewString(),
		redis:      redis,
		bySub:      make(map[string]*AgentConn),
	}
}

// Clustered reports whether cross-instance coordination is active.
func (r *Registry) Clustered() bool { return r.redis != nil }

func (r *Registry) add(ctx context.Context, sub string, conn *AgentConn) {
	r.mu.Lock()
	if old, ok := r.bySub[sub]; ok {
		old.close()
	}
	r.bySub[sub] = conn
	r.mu.Unlock()

	if r.redis != nil {
		_ = r.redis.Set(ctx, routeKeyPrefix+sub, r.instanceID, routeTTL).Err()
	}
}

func (r *Registry) remove(ctx context.Context, sub string, conn *AgentConn) {
	r.mu.Lock()
	if cur, ok := r.bySub[sub]; ok && cur == conn {
		delete(r.bySub, sub)
	}
	r.mu.Unlock()

	if r.redis != nil {
		_ = r.redis.Del(ctx, routeKeyPrefix+sub).Err()
	}
}

// localLookup returns the agent connection if this instance owns the subdomain.
func (r *Registry) localLookup(sub string) (*AgentConn, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	conn, ok := r.bySub[sub]
	return conn, ok
}

// ownerInstance returns the gateway instance id that owns a subdomain.
func (r *Registry) ownerInstance(ctx context.Context, sub string) (string, error) {
	if r.redis == nil {
		return "", ErrNoTunnel
	}
	id, err := r.redis.Get(ctx, routeKeyPrefix+sub).Result()
	if err == redis.Nil {
		return "", ErrNoTunnel
	}
	return id, err
}

// forwardRemote ships a request to the owning instance over Redis and waits for
// the correlated response.
func (r *Registry) forwardRemote(ctx context.Context, owner, sub string, req tunnelproto.ForwardedRequest) (*tunnelproto.ForwardedResponse, error) {
	reqID := uuid.NewString()
	ch := make(chan clusterResponse, 1)
	r.pending.Store(reqID, ch)
	defer r.pending.Delete(reqID)

	payload, _ := json.Marshal(clusterRequest{RequestID: reqID, Origin: r.instanceID, Subdomain: sub, Req: req})
	if err := r.redis.Publish(ctx, rpcChanPrefix+owner, payload).Err(); err != nil {
		return nil, err
	}

	select {
	case cr := <-ch:
		if cr.Error != "" {
			return nil, errors.New(cr.Error)
		}
		return cr.Resp, nil
	case <-time.After(forwardTimeout):
		return nil, ErrForwardTimeout
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// deliverReply routes a clusterResponse to the waiting forwardRemote call.
func (r *Registry) deliverReply(cr clusterResponse) {
	if ch, ok := r.pending.Load(cr.RequestID); ok {
		select {
		case ch.(chan clusterResponse) <- cr:
		default:
		}
	}
}

// refreshLoop periodically renews the TTL on routes this instance owns so peers
// can keep finding it.
func (r *Registry) refreshLoop(ctx context.Context) {
	if r.redis == nil {
		return
	}
	ticker := time.NewTicker(routeRefresh)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.mu.RLock()
			subs := make([]string, 0, len(r.bySub))
			for sub := range r.bySub {
				subs = append(subs, sub)
			}
			r.mu.RUnlock()
			for _, sub := range subs {
				_ = r.redis.Set(ctx, routeKeyPrefix+sub, r.instanceID, routeTTL).Err()
			}
		}
	}
}
