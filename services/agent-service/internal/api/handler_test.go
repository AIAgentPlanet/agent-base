package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"agent-base/services/agent-service/internal/integrity"
	"agent-base/services/agent-service/internal/runtime"
	"agent-base/services/agent-service/internal/token"
)

func TestDiscover(t *testing.T) {
	handler := NewHandler(runtime.NewMemoryStore()).Routes()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/runtime/discover", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["name"] != "agent-service" {
		t.Fatalf("unexpected discover response: %v", body)
	}
}

func TestCreateAgentValidation(t *testing.T) {
	handler := NewHandler(runtime.NewMemoryStore()).Routes()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/agents", bytes.NewBufferString(`{"type":"hermes"}`))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", response.Code)
	}
}

func TestAuditStatusUsesATHClientWhenConfigured(t *testing.T) {
	store := runtime.NewMemoryStore()
	session, err := store.CreateSession(runtime.CreateSessionInput{
		OwnerUserID: "user_a",
		Metadata:    map[string]string{"ath_handshake_id": "hsk_123"},
	})
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(store, WithATHAuditClient(&fakeAuditClient{configured: true})).Routes()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/"+session.ID+"/audit", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "verified" {
		t.Fatalf("unexpected audit status: %v", body)
	}
	if body["ath_configured"] != true {
		t.Fatalf("expected ath_configured true: %v", body)
	}
}

func TestAuditStatusReturnsStubWithoutATHClient(t *testing.T) {
	store := runtime.NewMemoryStore()
	session, err := store.CreateSession(runtime.CreateSessionInput{OwnerUserID: "user_a"})
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(store).Routes()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/"+session.ID+"/audit", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "stub" || body["ath_configured"] != false {
		t.Fatalf("unexpected audit stub: %v", body)
	}
}

func TestNextTurnRequiresParticipantToken(t *testing.T) {
	store := runtime.NewMemoryStore()
	agentA, err := store.CreateAgent(runtime.CreateAgentInput{UserID: "user_a", Identity: "https://a.example/.well-known/agent.json"})
	if err != nil {
		t.Fatal(err)
	}
	agentB, err := store.CreateAgent(runtime.CreateAgentInput{UserID: "user_b", Identity: "https://b.example/.well-known/agent.json"})
	if err != nil {
		t.Fatal(err)
	}
	session, err := store.CreateSession(runtime.CreateSessionInput{OwnerUserID: "user_a", Policy: runtime.SessionPolicy{MaxTurns: 2}})
	if err != nil {
		t.Fatal(err)
	}
	participantA, err := store.JoinSession(runtime.JoinSessionInput{SessionID: session.ID, UserID: "user_a", AgentID: agentA.ID, Role: "affirmative"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.JoinSession(runtime.JoinSessionInput{SessionID: session.ID, UserID: "user_b", AgentID: agentB.ID, Role: "negative"}); err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(store).Routes()

	request := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/"+session.ID+"/participants/"+participantA.ID+"/next-turn", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", response.Code)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/v1/sessions/"+session.ID+"/participants/"+participantA.ID+"/next-turn", nil)
	request.Header.Set("Authorization", "Bearer "+participantA.SessionToken)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200 with token, got %d: %s", response.Code, response.Body.String())
	}
}

func TestNextTurnAcceptsConfiguredATHJWT(t *testing.T) {
	store := runtime.NewMemoryStore()
	agentA, err := store.CreateAgent(runtime.CreateAgentInput{UserID: "user_a", Identity: "https://a.example/.well-known/agent.json"})
	if err != nil {
		t.Fatal(err)
	}
	agentB, err := store.CreateAgent(runtime.CreateAgentInput{UserID: "user_b", Identity: "https://b.example/.well-known/agent.json"})
	if err != nil {
		t.Fatal(err)
	}
	session, err := store.CreateSession(runtime.CreateSessionInput{
		OwnerUserID: "user_a",
		Metadata:    map[string]string{"ath_handshake_id": "hsk_123"},
		Policy:      runtime.SessionPolicy{MaxTurns: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	participantA, err := store.JoinSession(runtime.JoinSessionInput{SessionID: session.ID, UserID: "user_a", AgentID: agentA.ID, Role: "affirmative"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.JoinSession(runtime.JoinSessionInput{SessionID: session.ID, UserID: "user_b", AgentID: agentB.ID, Role: "negative"}); err != nil {
		t.Fatal(err)
	}
	verifier := &fakeTokenVerifier{configured: true}
	handler := NewHandler(store, WithATHTokenVerifier(verifier)).Routes()

	request := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/"+session.ID+"/participants/"+participantA.ID+"/next-turn", nil)
	request.Header.Set("Authorization", "Bearer header.payload.signature")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200 with ATH token, got %d: %s", response.Code, response.Body.String())
	}
	if verifier.input.SessionID != session.ID || verifier.input.HandshakeID != "hsk_123" || verifier.input.AgentIdentity != agentA.Identity {
		t.Fatalf("unexpected verifier input: %+v", verifier.input)
	}
}

func TestSubmitMessageWithATHJWTRequiresIntegrity(t *testing.T) {
	store, sessionID, participantID, turnID := testSessionReadyForSubmit(t)
	handler := NewHandler(store, WithATHTokenVerifier(&fakeTokenVerifier{configured: true})).Routes()
	body := `{"participant_id":"` + participantID + `","turn_id":"` + turnID + `","type":"argument","content":"hello"}`

	request := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/"+sessionID+"/messages", bytes.NewBufferString(body))
	request.Header.Set("Authorization", "Bearer header.payload.signature")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without integrity, got %d", response.Code)
	}

	handler = NewHandler(
		store,
		WithATHTokenVerifier(&fakeTokenVerifier{configured: true}),
		WithATHIntegrityVerifier(&fakeIntegrityVerifier{configured: true}),
	).Routes()
	request = httptest.NewRequest(http.MethodPost, "/api/v1/sessions/"+sessionID+"/messages", bytes.NewBufferString(body))
	request.Header.Set("Authorization", "Bearer header.payload.signature")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200 with integrity, got %d: %s", response.Code, response.Body.String())
	}
}

func TestConnectionDeliveryHTTPFlow(t *testing.T) {
	store := runtime.NewMemoryStore()
	agentA, err := store.CreateAgent(runtime.CreateAgentInput{UserID: "user_a", Identity: "https://a.example/.well-known/agent.json", ConnectionMode: runtime.ConnectionMCP})
	if err != nil {
		t.Fatal(err)
	}
	agentB, err := store.CreateAgent(runtime.CreateAgentInput{UserID: "user_b", Identity: "https://b.example/.well-known/agent.json"})
	if err != nil {
		t.Fatal(err)
	}
	session, err := store.CreateSession(runtime.CreateSessionInput{OwnerUserID: "user_a", Policy: runtime.SessionPolicy{MaxTurns: 2}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.JoinSession(runtime.JoinSessionInput{SessionID: session.ID, UserID: "user_a", AgentID: agentA.ID, Role: "affirmative"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.JoinSession(runtime.JoinSessionInput{SessionID: session.ID, UserID: "user_b", AgentID: agentB.ID, Role: "negative"}); err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(store).Routes()

	createBody := `{"agent_id":"` + agentA.ID + `","mode":"mcp"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/connections", bytes.NewBufferString(createBody))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200 creating connection, got %d: %s", response.Code, response.Body.String())
	}
	var connection runtime.Connection
	if err := json.Unmarshal(response.Body.Bytes(), &connection); err != nil {
		t.Fatal(err)
	}
	if connection.ConnectionToken == "" {
		t.Fatalf("expected connection token: %+v", connection)
	}

	nextPath := "/api/v1/connections/" + connection.ID + "/deliveries/next"
	request = httptest.NewRequest(http.MethodGet, nextPath, nil)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without connection token, got %d", response.Code)
	}

	request = httptest.NewRequest(http.MethodGet, nextPath, nil)
	request.Header.Set("X-Agent-Connection-Token", connection.ConnectionToken)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200 pulling delivery, got %d: %s", response.Code, response.Body.String())
	}
	var delivery runtime.Delivery
	if err := json.Unmarshal(response.Body.Bytes(), &delivery); err != nil {
		t.Fatal(err)
	}
	if delivery.Type != "turn.available" || delivery.Status != runtime.DeliveryStatusPending {
		t.Fatalf("unexpected delivery: %+v", delivery)
	}

	ackPath := "/api/v1/connections/" + connection.ID + "/deliveries/" + delivery.ID + "/ack"
	request = httptest.NewRequest(http.MethodPost, ackPath, bytes.NewBufferString(`{"status":"ok"}`))
	request.Header.Set("Authorization", "Bearer "+connection.ConnectionToken)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200 acking delivery, got %d: %s", response.Code, response.Body.String())
	}
	var acked runtime.Delivery
	if err := json.Unmarshal(response.Body.Bytes(), &acked); err != nil {
		t.Fatal(err)
	}
	if acked.Status != runtime.DeliveryStatusAcked {
		t.Fatalf("expected acked delivery, got %+v", acked)
	}
}

func TestConnectionWebSocketDeliveryFlow(t *testing.T) {
	store := runtime.NewMemoryStore()
	agentA, err := store.CreateAgent(runtime.CreateAgentInput{UserID: "user_a", Identity: "https://a.example/.well-known/agent.json", ConnectionMode: runtime.ConnectionWebSocket})
	if err != nil {
		t.Fatal(err)
	}
	agentB, err := store.CreateAgent(runtime.CreateAgentInput{UserID: "user_b", Identity: "https://b.example/.well-known/agent.json"})
	if err != nil {
		t.Fatal(err)
	}
	session, err := store.CreateSession(runtime.CreateSessionInput{OwnerUserID: "user_a", Policy: runtime.SessionPolicy{MaxTurns: 2}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.JoinSession(runtime.JoinSessionInput{SessionID: session.ID, UserID: "user_a", AgentID: agentA.ID, Role: "affirmative"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.JoinSession(runtime.JoinSessionInput{SessionID: session.ID, UserID: "user_b", AgentID: agentB.ID, Role: "negative"}); err != nil {
		t.Fatal(err)
	}
	connection, err := store.CreateConnection(runtime.CreateConnectionInput{AgentID: agentA.ID, Mode: runtime.ConnectionWebSocket})
	if err != nil {
		t.Fatal(err)
	}
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/connections/"+connection.ID+"/ws?token="+connection.ConnectionToken, nil)
	request.Header.Set("Upgrade", "websocket")
	request.Header.Set("Connection", "Upgrade")
	request.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	request.Header.Set("Sec-WebSocket-Version", "13")
	writer := &fakeHijackWriter{conn: serverConn, rw: bufio.NewReadWriter(bufio.NewReader(serverConn), bufio.NewWriter(serverConn)), header: http.Header{}}
	done := make(chan struct{})
	go func() {
		NewHandler(store).connectionWebSocket(writer, request, connection.ID)
		close(done)
	}()
	rw := bufio.NewReadWriter(bufio.NewReader(clientConn), bufio.NewWriter(clientConn))
	readWebSocketHandshake(t, rw.Reader)

	connected := readWebSocketEnvelope(t, rw.Reader)
	if connected.Type != "gateway.connected" || connected.Connection != connection.ID {
		t.Fatalf("unexpected connected envelope: %+v", connected)
	}
	envelope := readWebSocketEnvelope(t, rw.Reader)
	if envelope.Type != "delivery" || envelope.Delivery == nil {
		t.Fatalf("unexpected delivery envelope: %+v", envelope)
	}
	if envelope.Delivery.Status != runtime.DeliveryStatusPending || envelope.Delivery.Type != "turn.available" {
		t.Fatalf("unexpected delivery: %+v", envelope.Delivery)
	}
	writeClientTextFrame(t, clientConn, map[string]string{
		"type":        "delivery.ack",
		"delivery_id": envelope.Delivery.ID,
		"status":      "ok",
	})
	idle := readWebSocketEnvelope(t, rw.Reader)
	if idle.Type != "gateway.idle" {
		t.Fatalf("expected gateway.idle after ack, got %+v", idle)
	}
	<-done

	acked, err := store.AckDelivery(runtime.AckDeliveryInput{ConnectionID: "other", DeliveryID: envelope.Delivery.ID})
	if !errors.Is(err, runtime.ErrUnauthorized) || acked.ID != "" {
		t.Fatalf("expected delivery to already exist and reject wrong connection, got delivery=%+v err=%v", acked, err)
	}
	if _, err := store.NextDelivery(connection.ID); !errors.Is(err, runtime.ErrNoTurnAvailable) {
		t.Fatalf("expected no duplicate delivery after ack, got %v", err)
	}
}

func testSessionReadyForSubmit(t *testing.T) (*runtime.MemoryStore, string, string, string) {
	t.Helper()
	store := runtime.NewMemoryStore()
	agentA, err := store.CreateAgent(runtime.CreateAgentInput{UserID: "user_a", Identity: "https://a.example/.well-known/agent.json"})
	if err != nil {
		t.Fatal(err)
	}
	agentB, err := store.CreateAgent(runtime.CreateAgentInput{UserID: "user_b", Identity: "https://b.example/.well-known/agent.json"})
	if err != nil {
		t.Fatal(err)
	}
	session, err := store.CreateSession(runtime.CreateSessionInput{OwnerUserID: "user_a", Policy: runtime.SessionPolicy{MaxTurns: 2}})
	if err != nil {
		t.Fatal(err)
	}
	participantA, err := store.JoinSession(runtime.JoinSessionInput{SessionID: session.ID, UserID: "user_a", AgentID: agentA.ID, Role: "affirmative"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.JoinSession(runtime.JoinSessionInput{SessionID: session.ID, UserID: "user_b", AgentID: agentB.ID, Role: "negative"}); err != nil {
		t.Fatal(err)
	}
	turn, err := store.GetNextTurn(session.ID, participantA.ID)
	if err != nil {
		t.Fatal(err)
	}
	return store, session.ID, participantA.ID, turn.ID
}

func readWebSocketHandshake(t *testing.T, r *bufio.Reader) {
	t.Helper()
	status, err := r.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(status, "101") {
		t.Fatalf("expected 101 switching protocols, got %q", status)
	}
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if line == "\r\n" {
			break
		}
	}
}

func readWebSocketEnvelope(t *testing.T, r *bufio.Reader) websocketEnvelope {
	t.Helper()
	payload, err := readTextFrame(r)
	if err != nil {
		t.Fatal(err)
	}
	var envelope websocketEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		t.Fatal(err)
	}
	return envelope
}

func writeClientTextFrame(t *testing.T, w io.Writer, value any) {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	mask := [4]byte{1, 2, 3, 4}
	header := []byte{0x81}
	switch {
	case len(payload) < 126:
		header = append(header, 0x80|byte(len(payload)))
	case len(payload) <= 65535:
		header = append(header, 0x80|126, byte(len(payload)>>8), byte(len(payload)))
	default:
		header = append(header, 0x80|127)
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], uint64(len(payload)))
		header = append(header, size[:]...)
	}
	if _, err := w.Write(header); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(mask[:]); err != nil {
		t.Fatal(err)
	}
	for i := range payload {
		payload[i] ^= mask[i%4]
	}
	if _, err := w.Write(payload); err != nil {
		t.Fatal(err)
	}
}

type fakeHijackWriter struct {
	conn       net.Conn
	rw         *bufio.ReadWriter
	header     http.Header
	statusCode int
	body       bytes.Buffer
}

func (w *fakeHijackWriter) Header() http.Header {
	return w.header
}

func (w *fakeHijackWriter) Write(payload []byte) (int, error) {
	return w.body.Write(payload)
}

func (w *fakeHijackWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
}

func (w *fakeHijackWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.conn, w.rw, nil
}

type fakeAuditClient struct {
	configured bool
}

type fakeTokenVerifier struct {
	configured bool
	input      token.VerifyInput
}

type fakeIntegrityVerifier struct {
	configured bool
	input      integrity.Input
}

func (v *fakeIntegrityVerifier) Configured() bool {
	return v.configured
}

func (v *fakeIntegrityVerifier) Verify(_ context.Context, input integrity.Input) error {
	v.input = input
	return nil
}

func (v *fakeTokenVerifier) Configured() bool {
	return v.configured
}

func (v *fakeTokenVerifier) Verify(input token.VerifyInput) (*token.ATHClaims, error) {
	v.input = input
	return &token.ATHClaims{Type: "ath_access", SessionID: input.SessionID, AgentID: input.AgentIdentity, Scope: input.RequiredScope}, nil
}

func (c *fakeAuditClient) Configured() bool {
	return c.configured
}

func (c *fakeAuditClient) Verify(context.Context) (map[string]any, error) {
	return map[string]any{"valid": true, "record_count": float64(2)}, nil
}

func (c *fakeAuditClient) Query(_ context.Context, handshakeID string, _ int) (map[string]any, error) {
	return map[string]any{"records": []any{map[string]any{"handshake_id": handshakeID}}}, nil
}

func (c *fakeAuditClient) Introspect(context.Context, string) (map[string]any, error) {
	return map[string]any{"active": true}, nil
}
