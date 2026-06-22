package runtime

import (
	"errors"
	"strings"
	"testing"
)

func TestDebateSessionTurnFlow(t *testing.T) {
	store := NewMemoryStore()
	agentA, err := store.CreateAgent(CreateAgentInput{UserID: "user_a", Type: "hermes", Identity: "https://a.example/.well-known/agent.json"})
	if err != nil {
		t.Fatal(err)
	}
	agentB, err := store.CreateAgent(CreateAgentInput{UserID: "user_b", Type: "openclaw", Identity: "https://b.example/.well-known/agent.json"})
	if err != nil {
		t.Fatal(err)
	}
	session, err := store.CreateSession(CreateSessionInput{
		OwnerUserID: "user_a",
		Type:        "debate",
		Policy:      SessionPolicy{MaxTurns: 2, TurnPolicy: "alternate", AllowedMessageTypes: []string{"argument"}, AuditRequired: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	participantA, err := store.JoinSession(JoinSessionInput{SessionID: session.ID, UserID: "user_a", AgentID: agentA.ID, Role: "affirmative", Scopes: []string{"session:read", "session:speak"}})
	if err != nil {
		t.Fatal(err)
	}
	if participantA.SessionToken == "" {
		t.Fatal("expected participant session token")
	}
	if err := store.ValidateParticipantToken(session.ID, participantA.ID, participantA.SessionToken); err != nil {
		t.Fatal(err)
	}
	if err := store.ValidateParticipantToken(session.ID, participantA.ID, "wrong"); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
	participantB, err := store.JoinSession(JoinSessionInput{SessionID: session.ID, UserID: "user_b", AgentID: agentB.ID, Role: "negative", Scopes: []string{"session:read", "session:speak"}})
	if err != nil {
		t.Fatal(err)
	}

	firstTurn, err := store.GetNextTurn(session.ID, participantA.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetNextTurn(session.ID, participantB.ID); !errors.Is(err, ErrNotYourTurn) {
		t.Fatalf("expected ErrNotYourTurn, got %v", err)
	}
	if _, err := store.SubmitMessage(SubmitMessageInput{
		SessionID:     session.ID,
		ParticipantID: participantA.ID,
		TurnID:        firstTurn.ID,
		Type:          "argument",
		Content:       "正方观点",
		AuditRef:      "ath_audit:1",
	}); err != nil {
		t.Fatal(err)
	}
	secondTurn, err := store.GetNextTurn(session.ID, participantB.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SubmitMessage(SubmitMessageInput{
		SessionID:     session.ID,
		ParticipantID: participantB.ID,
		TurnID:        secondTurn.ID,
		Type:          "argument",
		Content:       "反方观点",
		AuditRef:      "ath_audit:2",
	}); err != nil {
		t.Fatal(err)
	}
	updated, err := store.GetSession(session.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, participant := range updated.Participants {
		if participant.SessionToken != "" || participant.TokenHash != "" {
			t.Fatalf("session leaked participant credentials: %+v", participant)
		}
	}
	if updated.Status != SessionStatusCompleted {
		t.Fatalf("expected completed session, got %s", updated.Status)
	}
}

func TestAgentCannotJoinForAnotherUser(t *testing.T) {
	store := NewMemoryStore()
	agent, err := store.CreateAgent(CreateAgentInput{UserID: "user_a", Identity: "https://a.example/.well-known/agent.json"})
	if err != nil {
		t.Fatal(err)
	}
	session, err := store.CreateSession(CreateSessionInput{OwnerUserID: "user_a"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.JoinSession(JoinSessionInput{SessionID: session.ID, UserID: "user_b", AgentID: agent.ID, Role: "negative"})
	if !errors.Is(err, ErrScopeDenied) {
		t.Fatalf("expected ErrScopeDenied, got %v", err)
	}
}

func TestConnectionDeliveryFlow(t *testing.T) {
	store := NewMemoryStore()
	agentA, err := store.CreateAgent(CreateAgentInput{UserID: "user_a", Identity: "https://a.example/.well-known/agent.json", ConnectionMode: ConnectionWebSocket})
	if err != nil {
		t.Fatal(err)
	}
	agentB, err := store.CreateAgent(CreateAgentInput{UserID: "user_b", Identity: "https://b.example/.well-known/agent.json"})
	if err != nil {
		t.Fatal(err)
	}
	session, err := store.CreateSession(CreateSessionInput{OwnerUserID: "user_a", Policy: SessionPolicy{MaxTurns: 2}})
	if err != nil {
		t.Fatal(err)
	}
	participantA, err := store.JoinSession(JoinSessionInput{SessionID: session.ID, UserID: "user_a", AgentID: agentA.ID, Role: "affirmative"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.JoinSession(JoinSessionInput{SessionID: session.ID, UserID: "user_b", AgentID: agentB.ID, Role: "negative"}); err != nil {
		t.Fatal(err)
	}

	connection, err := store.CreateConnection(CreateConnectionInput{AgentID: agentA.ID})
	if err != nil {
		t.Fatal(err)
	}
	if connection.ConnectionToken == "" || !strings.HasPrefix(connection.ConnectionToken, "act_") {
		t.Fatalf("expected one-time connection token, got %q", connection.ConnectionToken)
	}
	if err := store.ValidateConnectionToken(connection.ID, "wrong"); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized for bad connection token, got %v", err)
	}
	if err := store.ValidateConnectionToken(connection.ID, connection.ConnectionToken); err != nil {
		t.Fatal(err)
	}

	delivery, err := store.NextDelivery(connection.ID)
	if err != nil {
		t.Fatal(err)
	}
	if delivery.Type != "turn.available" || delivery.ParticipantID != participantA.ID || delivery.Status != DeliveryStatusPending {
		t.Fatalf("unexpected delivery: %+v", delivery)
	}
	if delivery.Payload["turn_id"] != delivery.TurnID || delivery.Payload["session_id"] != session.ID {
		t.Fatalf("unexpected delivery payload: %+v", delivery.Payload)
	}

	again, err := store.NextDelivery(connection.ID)
	if err != nil {
		t.Fatal(err)
	}
	if again.ID != delivery.ID {
		t.Fatalf("expected pending delivery to be reused, got %s then %s", delivery.ID, again.ID)
	}

	otherConnection, err := store.CreateConnection(CreateConnectionInput{AgentID: agentB.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.AckDelivery(AckDeliveryInput{ConnectionID: otherConnection.ID, DeliveryID: delivery.ID}); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized for wrong connection ack, got %v", err)
	}
	acked, err := store.AckDelivery(AckDeliveryInput{ConnectionID: connection.ID, DeliveryID: delivery.ID})
	if err != nil {
		t.Fatal(err)
	}
	if acked.Status != DeliveryStatusAcked || acked.AckedAt == nil {
		t.Fatalf("expected acked delivery, got %+v", acked)
	}
}
