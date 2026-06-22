package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"agent-base/services/agent-service/internal/integrity"
	"agent-base/services/agent-service/internal/runtime"
	"agent-base/services/agent-service/internal/token"
)

type Handler struct {
	store              runtime.Store
	athAudit           ATHAuditClient
	athTokenVerify     ATHTokenVerifier
	athIntegrityVerify ATHIntegrityVerifier
}

type ATHAuditClient interface {
	Configured() bool
	Verify(ctx context.Context) (map[string]any, error)
	Query(ctx context.Context, handshakeID string, limit int) (map[string]any, error)
	Introspect(ctx context.Context, token string) (map[string]any, error)
}

type ATHTokenVerifier interface {
	Configured() bool
	Verify(input token.VerifyInput) (*token.ATHClaims, error)
}

type ATHIntegrityVerifier interface {
	Configured() bool
	Verify(ctx context.Context, input integrity.Input) error
}

type Option func(*Handler)

func WithATHAuditClient(client ATHAuditClient) Option {
	return func(h *Handler) {
		h.athAudit = client
	}
}

func WithATHTokenVerifier(verifier ATHTokenVerifier) Option {
	return func(h *Handler) {
		h.athTokenVerify = verifier
	}
}

func WithATHIntegrityVerifier(verifier ATHIntegrityVerifier) Option {
	return func(h *Handler) {
		h.athIntegrityVerify = verifier
	}
}

func NewHandler(store runtime.Store, options ...Option) *Handler {
	handler := &Handler{store: store}
	for _, option := range options {
		option(handler)
	}
	return handler
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.healthz)
	mux.HandleFunc("GET /api/v1/runtime/discover", h.discover)
	mux.HandleFunc("POST /api/v1/agents", h.createAgent)
	mux.HandleFunc("GET /api/v1/agents/", h.getAgent)
	mux.HandleFunc("POST /api/v1/connections", h.createConnection)
	mux.HandleFunc("GET /api/v1/connections/", h.connectionRoute)
	mux.HandleFunc("POST /api/v1/connections/", h.connectionRoute)
	mux.HandleFunc("POST /api/v1/sessions", h.createSession)
	mux.HandleFunc("/api/v1/sessions/", h.sessionRoute)
	return mux
}

func (h *Handler) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) discover(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"name": "agent-service",
		"connection_modes": []string{
			string(runtime.ConnectionHTTPSWebhook),
			string(runtime.ConnectionWebSocket),
			string(runtime.ConnectionMCP),
		},
		"session_types":            []string{"debate"},
		"delivery_modes":           []string{"http_polling", "websocket"},
		"ath_audit_configured":     h.athAudit != nil && h.athAudit.Configured(),
		"ath_token_configured":     h.athTokenVerify != nil && h.athTokenVerify.Configured(),
		"ath_integrity_configured": h.athIntegrityVerify != nil && h.athIntegrityVerify.Configured(),
		"mvp_limits": []string{
			"in-memory store",
			"HTTP polling and WebSocket gateway are implemented for turn.available deliveries",
			"ATH token revocation requires ATH_AUDIT_* introspection config",
			"writable MCP adapter is not implemented yet",
		},
	})
}

func (h *Handler) createAgent(w http.ResponseWriter, r *http.Request) {
	var input runtime.CreateAgentInput
	if !decodeJSON(w, r, &input) {
		return
	}
	agent, err := h.store.CreateAgent(input)
	writeResult(w, agent, err)
}

func (h *Handler) getAgent(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/agents/")
	agent, err := h.store.GetAgent(id)
	writeResult(w, agent, err)
}

func (h *Handler) createConnection(w http.ResponseWriter, r *http.Request) {
	var input runtime.CreateConnectionInput
	if !decodeJSON(w, r, &input) {
		return
	}
	connection, err := h.store.CreateConnection(input)
	writeResult(w, connection, err)
}

func (h *Handler) connectionRoute(w http.ResponseWriter, r *http.Request) {
	parts := splitPath(strings.TrimPrefix(r.URL.Path, "/api/v1/connections/"))
	if len(parts) == 3 && parts[1] == "deliveries" && parts[2] == "next" && r.Method == http.MethodGet {
		connectionID := parts[0]
		if !h.requireConnectionToken(w, r, connectionID) {
			return
		}
		delivery, err := h.store.NextDelivery(connectionID)
		writeResult(w, delivery, err)
		return
	}
	if len(parts) == 4 && parts[1] == "deliveries" && parts[3] == "ack" && r.Method == http.MethodPost {
		connectionID := parts[0]
		if !h.requireConnectionToken(w, r, connectionID) {
			return
		}
		var input runtime.AckDeliveryInput
		if !decodeJSON(w, r, &input) {
			return
		}
		input.ConnectionID = connectionID
		input.DeliveryID = parts[2]
		delivery, err := h.store.AckDelivery(input)
		writeResult(w, delivery, err)
		return
	}
	if len(parts) == 2 && parts[1] == "ws" && r.Method == http.MethodGet {
		h.connectionWebSocket(w, r, parts[0])
		return
	}
	writeError(w, http.StatusNotFound, "not found")
}

func (h *Handler) createSession(w http.ResponseWriter, r *http.Request) {
	var input runtime.CreateSessionInput
	if !decodeJSON(w, r, &input) {
		return
	}
	session, err := h.store.CreateSession(input)
	writeResult(w, session, err)
}

func (h *Handler) sessionRoute(w http.ResponseWriter, r *http.Request) {
	parts := splitPath(strings.TrimPrefix(r.URL.Path, "/api/v1/sessions/"))
	if len(parts) == 0 {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	sessionID := parts[0]
	if len(parts) == 1 && r.Method == http.MethodGet {
		session, err := h.store.GetSession(sessionID)
		writeResult(w, session, err)
		return
	}
	if len(parts) == 2 && parts[1] == "participants" && r.Method == http.MethodPost {
		h.joinSession(w, r, sessionID)
		return
	}
	if len(parts) == 2 && parts[1] == "messages" && r.Method == http.MethodGet {
		messages, err := h.store.ListMessages(sessionID)
		writeResult(w, messages, err)
		return
	}
	if len(parts) == 2 && parts[1] == "audit" && r.Method == http.MethodGet {
		h.getAuditStatus(w, r, sessionID)
		return
	}
	if len(parts) == 4 && parts[1] == "participants" && parts[3] == "next-turn" && r.Method == http.MethodGet {
		if _, ok := h.requireParticipantToken(w, r, sessionID, parts[2]); !ok {
			return
		}
		turn, err := h.store.GetNextTurn(sessionID, parts[2])
		writeResult(w, turn, err)
		return
	}
	if len(parts) == 2 && parts[1] == "messages" && r.Method == http.MethodPost {
		h.submitMessage(w, r, sessionID)
		return
	}
	writeError(w, http.StatusNotFound, "not found")
}

func (h *Handler) getAuditStatus(w http.ResponseWriter, r *http.Request, sessionID string) {
	session, err := h.store.GetSession(sessionID)
	if err != nil {
		writeResult(w, nil, err)
		return
	}
	messages, err := h.store.ListMessages(sessionID)
	if err != nil {
		writeResult(w, nil, err)
		return
	}
	status := runtime.AuditStatus{
		SessionID:        sessionID,
		Status:           "stub",
		Message:          "MVP stores audit_ref on messages; configure ATH_AUDIT_* env vars to proxy verification to user-service.",
		ATHConfigured:    h.athAudit != nil && h.athAudit.Configured(),
		HandshakeID:      session.Metadata["ath_handshake_id"],
		MessageAuditRefs: auditRefs(messages),
	}
	if !status.ATHConfigured {
		writeJSON(w, http.StatusOK, status)
		return
	}

	verification, err := h.athAudit.Verify(r.Context())
	if err != nil {
		status.Status = "error"
		status.Message = err.Error()
		writeJSON(w, http.StatusBadGateway, status)
		return
	}
	status.Status = "verified"
	status.Message = "ATH audit verification completed through user-service."
	status.Verification = verification

	if status.HandshakeID != "" {
		records, err := h.athAudit.Query(r.Context(), status.HandshakeID, 100)
		if err != nil {
			status.Status = "partial"
			status.Message = err.Error()
			writeJSON(w, http.StatusBadGateway, status)
			return
		}
		status.Records = records["records"]
	}
	writeJSON(w, http.StatusOK, status)
}

func (h *Handler) joinSession(w http.ResponseWriter, r *http.Request, sessionID string) {
	var input runtime.JoinSessionInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.SessionID = sessionID
	participant, err := h.store.JoinSession(input)
	writeResult(w, participant, err)
}

func (h *Handler) submitMessage(w http.ResponseWriter, r *http.Request, sessionID string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	var input runtime.SubmitMessageInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.SessionID = sessionID
	authKind, ok := h.requireParticipantToken(w, r, sessionID, input.ParticipantID)
	if !ok {
		return
	}
	if authKind == "ath" && !h.verifyATHRequestIntegrity(w, r, sessionID, input.ParticipantID, body) {
		return
	}
	message, err := h.store.SubmitMessage(input)
	writeResult(w, message, err)
}

func (h *Handler) requireParticipantToken(w http.ResponseWriter, r *http.Request, sessionID, participantID string) (string, bool) {
	rawToken := participantToken(r)
	if h.athTokenVerify != nil && h.athTokenVerify.Configured() && strings.Count(rawToken, ".") == 2 {
		if h.verifyATHParticipantToken(w, r.Context(), rawToken, sessionID, participantID) {
			return "ath", true
		}
		return "", false
	}
	err := h.store.ValidateParticipantToken(sessionID, participantID, rawToken)
	if err == nil {
		return "session", true
	}
	writeResult(w, nil, err)
	return "", false
}

func (h *Handler) requireConnectionToken(w http.ResponseWriter, r *http.Request, connectionID string) bool {
	err := h.store.ValidateConnectionToken(connectionID, connectionToken(r))
	if err == nil {
		return true
	}
	writeResult(w, nil, err)
	return false
}

func (h *Handler) verifyATHParticipantToken(w http.ResponseWriter, ctx context.Context, rawToken, sessionID, participantID string) bool {
	session, err := h.store.GetSession(sessionID)
	if err != nil {
		writeResult(w, nil, err)
		return false
	}
	var participant runtime.Participant
	for _, candidate := range session.Participants {
		if candidate.ID == participantID {
			participant = candidate
			break
		}
	}
	if participant.ID == "" {
		writeResult(w, nil, runtime.ErrNotFound)
		return false
	}
	_, err = h.athTokenVerify.Verify(token.VerifyInput{
		Token:         rawToken,
		SessionID:     session.ID,
		HandshakeID:   session.Metadata["ath_handshake_id"],
		AgentIdentity: participant.AgentIdentity,
		RequiredScope: "session:speak",
	})
	if err != nil {
		writeResult(w, nil, runtime.ErrUnauthorized)
		return false
	}
	if h.athAudit != nil && h.athAudit.Configured() {
		result, err := h.athAudit.Introspect(ctx, rawToken)
		if err != nil || result["active"] != true {
			writeResult(w, nil, runtime.ErrUnauthorized)
			return false
		}
	}
	return true
}

func (h *Handler) verifyATHRequestIntegrity(w http.ResponseWriter, r *http.Request, sessionID, participantID string, body []byte) bool {
	if h.athIntegrityVerify == nil || !h.athIntegrityVerify.Configured() {
		writeResult(w, nil, runtime.ErrUnauthorized)
		return false
	}
	err := h.athIntegrityVerify.Verify(r.Context(), integrity.Input{
		Method:        r.Method,
		Path:          r.URL.Path,
		SessionID:     sessionID,
		ParticipantID: participantID,
		Body:          body,
		Timestamp:     r.Header.Get("X-ATH-Timestamp"),
		Nonce:         r.Header.Get("X-ATH-Nonce"),
		BodySHA256:    r.Header.Get("X-ATH-Body-SHA256"),
		Signature:     r.Header.Get("X-ATH-Signature"),
	})
	if err != nil {
		writeResult(w, nil, runtime.ErrUnauthorized)
		return false
	}
	return true
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return false
	}
	return true
}

func writeResult(w http.ResponseWriter, value any, err error) {
	if err == nil {
		writeJSON(w, http.StatusOK, value)
		return
	}
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, runtime.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, runtime.ErrInvalidArgument), errors.Is(err, runtime.ErrSchemaInvalid):
		status = http.StatusBadRequest
	case errors.Is(err, runtime.ErrScopeDenied):
		status = http.StatusForbidden
	case errors.Is(err, runtime.ErrUnauthorized):
		status = http.StatusUnauthorized
	case errors.Is(err, runtime.ErrNotYourTurn), errors.Is(err, runtime.ErrNoTurnAvailable):
		status = http.StatusConflict
	}
	writeError(w, status, err.Error())
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func splitPath(path string) []string {
	raw := strings.Split(strings.Trim(path, "/"), "/")
	parts := raw[:0]
	for _, part := range raw {
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}

func auditRefs(messages []runtime.Message) []string {
	refs := make([]string, 0, len(messages))
	for _, message := range messages {
		if message.AuditRef != "" {
			refs = append(refs, message.AuditRef)
		}
	}
	return refs
}

func participantToken(r *http.Request) string {
	if token := r.Header.Get("X-Agent-Session-Token"); token != "" {
		return token
	}
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	}
	return ""
}

func connectionToken(r *http.Request) string {
	if token := r.Header.Get("X-Agent-Connection-Token"); token != "" {
		return token
	}
	if token := r.URL.Query().Get("token"); token != "" {
		return token
	}
	return participantToken(r)
}
