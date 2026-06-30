// Package tunnelproto defines the WebSocket message protocol spoken between the
// Gotra CLI agent and the tunnel gateway (Backend Bible — Agent-Gateway
// Protocol). Messages are JSON envelopes; request/response bodies are carried as
// byte slices (base64 in JSON).
package tunnelproto

import "encoding/json"

// MessageType enumerates the protocol message kinds.
type MessageType string

const (
	// TypeRegisterTunnel: agent → gateway, opens a tunnel.
	TypeRegisterTunnel MessageType = "REGISTER_TUNNEL"
	// TypeRegistered: gateway → agent, confirms the tunnel and its public URL.
	TypeRegistered MessageType = "REGISTERED"
	// TypeHeartbeat: agent → gateway keepalive (15s interval).
	TypeHeartbeat MessageType = "HEARTBEAT"
	// TypeRequestForward: gateway → agent, a captured public request to replay locally.
	TypeRequestForward MessageType = "REQUEST_FORWARD"
	// TypeResponseForward: agent → gateway, the local app's response.
	TypeResponseForward MessageType = "RESPONSE_FORWARD"
	// TypeDisconnect: either direction, graceful shutdown.
	TypeDisconnect MessageType = "DISCONNECT"
	// TypeError: either direction, carries an error message.
	TypeError MessageType = "ERROR"
)

// Envelope wraps every protocol message. RequestID correlates a forwarded
// request with its response across the multiplexed connection.
type Envelope struct {
	Type      MessageType     `json:"type"`
	RequestID string          `json:"request_id,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

// RegisterTunnelPayload is sent by the agent to open a tunnel.
type RegisterTunnelPayload struct {
	Token     string `json:"token"`      // user access token (JWT)
	ProjectID string `json:"project_id"` // project the tunnel belongs to
	LocalPort int    `json:"local_port"` // local port being exposed
}

// RegisteredPayload confirms a tunnel.
type RegisteredPayload struct {
	TunnelID  string `json:"tunnel_id"`
	PublicURL string `json:"public_url"`
}

// ForwardedRequest is a public HTTP request to be replayed against the local app.
type ForwardedRequest struct {
	Method  string              `json:"method"`
	Path    string              `json:"path"` // path + raw query
	Headers map[string][]string `json:"headers"`
	Body    []byte              `json:"body,omitempty"`
}

// ForwardedResponse is the local app's response, returned to the gateway.
type ForwardedResponse struct {
	Status  int                 `json:"status"`
	Headers map[string][]string `json:"headers"`
	Body    []byte              `json:"body,omitempty"`
}

// ErrorPayload carries a human-readable error.
type ErrorPayload struct {
	Message string `json:"message"`
}

// NewEnvelope builds an envelope, marshalling the payload.
func NewEnvelope(t MessageType, requestID string, payload any) (Envelope, error) {
	env := Envelope{Type: t, RequestID: requestID}
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return Envelope{}, err
		}
		env.Payload = raw
	}
	return env, nil
}

// Decode unmarshals an envelope's payload into v.
func (e Envelope) Decode(v any) error {
	return json.Unmarshal(e.Payload, v)
}
