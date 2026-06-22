package runtime

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"
)

var (
	ErrNotFound        = errors.New("not found")
	ErrInvalidArgument = errors.New("invalid argument")
	ErrScopeDenied     = errors.New("scope denied")
	ErrNotYourTurn     = errors.New("not your turn")
	ErrNoTurnAvailable = errors.New("no turn available")
	ErrSchemaInvalid   = errors.New("schema invalid")
	ErrUnauthorized    = errors.New("unauthorized")
)

type CreateAgentInput struct {
	UserID         string            `json:"user_id"`
	Type           string            `json:"type"`
	Identity       string            `json:"identity"`
	DisplayName    string            `json:"display_name"`
	ConnectionMode ConnectionMode    `json:"connection_mode"`
	Capabilities   []string          `json:"capabilities"`
	Metadata       map[string]string `json:"metadata"`
}

type CreateSessionInput struct {
	Type        string            `json:"type"`
	OwnerUserID string            `json:"owner_user_id"`
	Policy      SessionPolicy     `json:"policy"`
	Metadata    map[string]string `json:"metadata"`
}

type JoinSessionInput struct {
	SessionID string   `json:"session_id"`
	UserID    string   `json:"user_id"`
	AgentID   string   `json:"agent_id"`
	Role      string   `json:"role"`
	Scopes    []string `json:"scopes"`
}

type SubmitMessageInput struct {
	SessionID     string `json:"session_id"`
	ParticipantID string `json:"participant_id"`
	TurnID        string `json:"turn_id"`
	Type          string `json:"type"`
	Content       string `json:"content"`
	AuditRef      string `json:"audit_ref"`
}

type CreateConnectionInput struct {
	AgentID  string            `json:"agent_id"`
	Mode     ConnectionMode    `json:"mode"`
	Metadata map[string]string `json:"metadata"`
}

type AckDeliveryInput struct {
	ConnectionID string `json:"connection_id"`
	DeliveryID   string `json:"delivery_id"`
	Status       string `json:"status"`
}

type Store interface {
	CreateAgent(input CreateAgentInput) (Agent, error)
	GetAgent(id string) (Agent, error)
	CreateConnection(input CreateConnectionInput) (Connection, error)
	ValidateConnectionToken(connectionID, token string) error
	NextDelivery(connectionID string) (Delivery, error)
	AckDelivery(input AckDeliveryInput) (Delivery, error)
	CreateSession(input CreateSessionInput) (Session, error)
	GetSession(id string) (Session, error)
	JoinSession(input JoinSessionInput) (Participant, error)
	ValidateParticipantToken(sessionID, participantID, token string) error
	GetNextTurn(sessionID, participantID string) (Turn, error)
	SubmitMessage(input SubmitMessageInput) (Message, error)
	ListMessages(sessionID string) ([]Message, error)
	GetAuditStatus(sessionID string) (AuditStatus, error)
}

type MemoryStore struct {
	mu              sync.Mutex
	next            int
	agents          map[string]Agent
	connections     map[string]Connection
	sessions        map[string]Session
	participants    map[string]Participant
	turns           map[string]Turn
	messages        map[string]Message
	deliveries      map[string]Delivery
	sessionTurns    map[string][]string
	sessionMessages map[string][]string
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		agents:          make(map[string]Agent),
		connections:     make(map[string]Connection),
		sessions:        make(map[string]Session),
		participants:    make(map[string]Participant),
		turns:           make(map[string]Turn),
		messages:        make(map[string]Message),
		deliveries:      make(map[string]Delivery),
		sessionTurns:    make(map[string][]string),
		sessionMessages: make(map[string][]string),
	}
}

func (s *MemoryStore) CreateConnection(input CreateConnectionInput) (Connection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	agent, ok := s.agents[input.AgentID]
	if !ok {
		return Connection{}, ErrNotFound
	}
	mode := input.Mode
	if mode == "" {
		mode = agent.ConnectionMode
	}
	token, tokenHash, err := newToken("act")
	if err != nil {
		return Connection{}, err
	}
	now := time.Now().UTC()
	connection := Connection{
		ID: s.id("conn"), AgentID: agent.ID, UserID: agent.UserID,
		Mode: mode, Status: ConnectionStatusOnline, TokenHash: tokenHash,
		Metadata: cloneMap(input.Metadata), LastSeenAt: now, CreatedAt: now, UpdatedAt: now,
	}
	s.connections[connection.ID] = connection
	response := publicConnection(connection)
	response.ConnectionToken = token
	return response, nil
}

func (s *MemoryStore) ValidateConnectionToken(connectionID, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	connection, ok := s.connections[connectionID]
	if !ok {
		return ErrNotFound
	}
	if connection.Status == ConnectionStatusRevoked {
		return ErrUnauthorized
	}
	if token == "" || connection.TokenHash == "" {
		return ErrUnauthorized
	}
	if subtle.ConstantTimeCompare([]byte(hashToken(token)), []byte(connection.TokenHash)) != 1 {
		return ErrUnauthorized
	}
	connection.LastSeenAt = time.Now().UTC()
	connection.UpdatedAt = connection.LastSeenAt
	s.connections[connection.ID] = connection
	return nil
}

func (s *MemoryStore) NextDelivery(connectionID string) (Delivery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	connection, ok := s.connections[connectionID]
	if !ok {
		return Delivery{}, ErrNotFound
	}
	for _, delivery := range s.deliveries {
		if delivery.ConnectionID == connectionID && delivery.Status == DeliveryStatusPending {
			return delivery, nil
		}
	}
	for _, turnID := range orderedTurnIDs(s.sessionTurns) {
		turn := s.turns[turnID]
		if turn.Status != TurnStatusOpen {
			continue
		}
		if s.deliveryExistsLocked(connectionID, turn.ID) {
			continue
		}
		participant := s.participants[turn.ParticipantID]
		if participant.AgentID != connection.AgentID {
			continue
		}
		delivery := s.newTurnDeliveryLocked(connection, participant, turn)
		return delivery, nil
	}
	return Delivery{}, ErrNoTurnAvailable
}

func (s *MemoryStore) deliveryExistsLocked(connectionID, turnID string) bool {
	for _, delivery := range s.deliveries {
		if delivery.ConnectionID == connectionID && delivery.TurnID == turnID {
			return true
		}
	}
	return false
}

func (s *MemoryStore) AckDelivery(input AckDeliveryInput) (Delivery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delivery, ok := s.deliveries[input.DeliveryID]
	if !ok {
		return Delivery{}, ErrNotFound
	}
	if delivery.ConnectionID != input.ConnectionID {
		return Delivery{}, ErrUnauthorized
	}
	now := time.Now().UTC()
	delivery.Status = DeliveryStatusAcked
	delivery.AckedAt = &now
	delivery.UpdatedAt = now
	s.deliveries[delivery.ID] = delivery
	return delivery, nil
}

func (s *MemoryStore) CreateAgent(input CreateAgentInput) (Agent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if strings.TrimSpace(input.UserID) == "" || strings.TrimSpace(input.Identity) == "" {
		return Agent{}, ErrInvalidArgument
	}
	if input.ConnectionMode == "" {
		input.ConnectionMode = ConnectionMCP
	}
	now := time.Now().UTC()
	agent := Agent{
		ID:             s.id("agt"),
		UserID:         input.UserID,
		Type:           defaultString(input.Type, "custom"),
		Identity:       input.Identity,
		DisplayName:    defaultString(input.DisplayName, input.Identity),
		ConnectionMode: input.ConnectionMode,
		Capabilities:   append([]string(nil), input.Capabilities...),
		Status:         AgentStatusBound,
		Metadata:       cloneMap(input.Metadata),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	s.agents[agent.ID] = agent
	return agent, nil
}

func (s *MemoryStore) GetAgent(id string) (Agent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	agent, ok := s.agents[id]
	if !ok {
		return Agent{}, ErrNotFound
	}
	return agent, nil
}

func (s *MemoryStore) CreateSession(input CreateSessionInput) (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if strings.TrimSpace(input.OwnerUserID) == "" {
		return Session{}, ErrInvalidArgument
	}
	if input.Type == "" {
		input.Type = "debate"
	}
	if input.Policy.TurnPolicy == "" {
		input.Policy.TurnPolicy = "alternate"
	}
	if input.Policy.MaxTurns <= 0 {
		input.Policy.MaxTurns = 6
	}
	if len(input.Policy.AllowedMessageTypes) == 0 {
		input.Policy.AllowedMessageTypes = []string{"argument", "rebuttal", "closing", "judge_result", "message"}
	}
	now := time.Now().UTC()
	session := Session{
		ID:           s.id("ses"),
		Type:         input.Type,
		OwnerUserID:  input.OwnerUserID,
		Status:       SessionStatusWaitingParticipants,
		Policy:       input.Policy,
		Participants: []Participant{},
		Metadata:     cloneMap(input.Metadata),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	s.sessions[session.ID] = session
	return session, nil
}

func (s *MemoryStore) GetSession(id string) (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.getSessionLocked(id)
}

func (s *MemoryStore) JoinSession(input JoinSessionInput) (Participant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[input.SessionID]
	if !ok {
		return Participant{}, ErrNotFound
	}
	agent, ok := s.agents[input.AgentID]
	if !ok {
		return Participant{}, ErrNotFound
	}
	if strings.TrimSpace(input.UserID) == "" || strings.TrimSpace(input.Role) == "" {
		return Participant{}, ErrInvalidArgument
	}
	if agent.UserID != input.UserID {
		return Participant{}, ErrScopeDenied
	}
	if len(input.Scopes) == 0 {
		input.Scopes = []string{"session:read", "session:speak"}
	}
	if !hasScope(input.Scopes, "session:read") {
		return Participant{}, ErrScopeDenied
	}

	now := time.Now().UTC()
	token, tokenHash, err := newSessionToken()
	if err != nil {
		return Participant{}, err
	}
	participant := Participant{
		ID:            s.id("par"),
		SessionID:     input.SessionID,
		UserID:        input.UserID,
		AgentID:       input.AgentID,
		AgentIdentity: agent.Identity,
		Role:          input.Role,
		Scopes:        append([]string(nil), input.Scopes...),
		Status:        ParticipantStatusReady,
		TokenHash:     tokenHash,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	s.participants[participant.ID] = participant
	session.Participants = append(session.Participants, publicParticipant(participant))
	session.Status = sessionStatusForParticipants(len(session.Participants))
	session.UpdatedAt = now
	s.sessions[session.ID] = session
	s.rebuildTurnsLocked(session)
	response := publicParticipant(participant)
	response.SessionToken = token
	return response, nil
}

func (s *MemoryStore) ValidateParticipantToken(sessionID, participantID, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	participant, ok := s.participants[participantID]
	if !ok || participant.SessionID != sessionID {
		return ErrNotFound
	}
	if token == "" || participant.TokenHash == "" {
		return ErrUnauthorized
	}
	if subtle.ConstantTimeCompare([]byte(hashToken(token)), []byte(participant.TokenHash)) != 1 {
		return ErrUnauthorized
	}
	return nil
}

func (s *MemoryStore) GetNextTurn(sessionID, participantID string) (Turn, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	participant, ok := s.participants[participantID]
	if !ok || participant.SessionID != sessionID {
		return Turn{}, ErrNotFound
	}
	if !hasScope(participant.Scopes, "session:speak") {
		return Turn{}, ErrScopeDenied
	}
	for _, turnID := range s.sessionTurns[sessionID] {
		turn := s.turns[turnID]
		if turn.Status == TurnStatusOpen {
			if turn.ParticipantID != participantID {
				return Turn{}, ErrNotYourTurn
			}
			return turn, nil
		}
	}
	return Turn{}, ErrNoTurnAvailable
}

func (s *MemoryStore) SubmitMessage(input SubmitMessageInput) (Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[input.SessionID]
	if !ok {
		return Message{}, ErrNotFound
	}
	participant, ok := s.participants[input.ParticipantID]
	if !ok || participant.SessionID != input.SessionID {
		return Message{}, ErrNotFound
	}
	if !hasScope(participant.Scopes, "session:speak") {
		return Message{}, ErrScopeDenied
	}
	turn, ok := s.turns[input.TurnID]
	if !ok || turn.SessionID != input.SessionID {
		return Message{}, ErrNotFound
	}
	if turn.ParticipantID != input.ParticipantID || turn.Status != TurnStatusOpen {
		return Message{}, ErrNotYourTurn
	}
	if strings.TrimSpace(input.Content) == "" || !slices.Contains(session.Policy.AllowedMessageTypes, input.Type) {
		return Message{}, ErrSchemaInvalid
	}

	now := time.Now().UTC()
	message := Message{
		ID:            s.id("msg"),
		SessionID:     input.SessionID,
		ParticipantID: input.ParticipantID,
		TurnID:        input.TurnID,
		Type:          input.Type,
		Content:       input.Content,
		AuditRef:      input.AuditRef,
		CreatedAt:     now,
	}
	turn.Status = TurnStatusSubmitted
	turn.UpdatedAt = now
	s.turns[turn.ID] = turn
	s.messages[message.ID] = message
	s.sessionMessages[input.SessionID] = append(s.sessionMessages[input.SessionID], message.ID)
	s.updateSessionCompletionLocked(session.ID)
	return message, nil
}

func (s *MemoryStore) ListMessages(sessionID string) ([]Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.sessions[sessionID]; !ok {
		return nil, ErrNotFound
	}
	messages := make([]Message, 0, len(s.sessionMessages[sessionID]))
	for _, id := range s.sessionMessages[sessionID] {
		messages = append(messages, s.messages[id])
	}
	return messages, nil
}

func (s *MemoryStore) GetAuditStatus(sessionID string) (AuditStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.sessions[sessionID]; !ok {
		return AuditStatus{}, ErrNotFound
	}
	return AuditStatus{
		SessionID: sessionID,
		Status:    "stub",
		Message:   "MVP stores audit_ref on messages; ATH audit-chain verification will be delegated to user-service.",
	}, nil
}

func (s *MemoryStore) getSessionLocked(id string) (Session, error) {
	session, ok := s.sessions[id]
	if !ok {
		return Session{}, ErrNotFound
	}
	session.Participants = append([]Participant{}, session.Participants...)
	for i := range session.Participants {
		session.Participants[i] = publicParticipant(session.Participants[i])
	}
	return session, nil
}

func (s *MemoryStore) rebuildTurnsLocked(session Session) {
	if len(session.Participants) < 2 || len(s.sessionTurns[session.ID]) > 0 {
		return
	}
	now := time.Now().UTC()
	for i := 0; i < session.Policy.MaxTurns; i++ {
		participant := session.Participants[i%len(session.Participants)]
		turn := Turn{
			ID:            s.id("turn"),
			SessionID:     session.ID,
			ParticipantID: participant.ID,
			Index:         i + 1,
			Phase:         debatePhase(i, session.Policy.MaxTurns),
			Status:        TurnStatusOpen,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		s.turns[turn.ID] = turn
		s.sessionTurns[session.ID] = append(s.sessionTurns[session.ID], turn.ID)
	}
	session.Status = SessionStatusActive
	session.UpdatedAt = now
	s.sessions[session.ID] = session
}

func (s *MemoryStore) updateSessionCompletionLocked(sessionID string) {
	for _, turnID := range s.sessionTurns[sessionID] {
		if s.turns[turnID].Status == TurnStatusOpen {
			return
		}
	}
	session := s.sessions[sessionID]
	session.Status = SessionStatusCompleted
	session.UpdatedAt = time.Now().UTC()
	s.sessions[sessionID] = session
}

func (s *MemoryStore) newTurnDeliveryLocked(connection Connection, participant Participant, turn Turn) Delivery {
	now := time.Now().UTC()
	session := s.sessions[turn.SessionID]
	delivery := Delivery{
		ID: s.id("deliv"), ConnectionID: connection.ID, AgentID: connection.AgentID,
		SessionID: turn.SessionID, ParticipantID: participant.ID, TurnID: turn.ID,
		Type: "turn.available", Status: DeliveryStatusPending,
		Payload: map[string]any{
			"session_id": turn.SessionID, "participant_id": participant.ID, "turn_id": turn.ID,
			"session_type": session.Type, "role": participant.Role, "phase": turn.Phase,
			"turn_index": turn.Index, "metadata": session.Metadata,
		},
		CreatedAt: now, UpdatedAt: now,
	}
	s.deliveries[delivery.ID] = delivery
	return delivery
}

func (s *MemoryStore) id(prefix string) string {
	s.next++
	return fmt.Sprintf("%s_%06d", prefix, s.next)
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func cloneMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func newSessionToken() (string, string, error) {
	return newToken("ast")
}

func newToken(prefix string) (string, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	token := prefix + "_" + base64.RawURLEncoding.EncodeToString(raw)
	return token, hashToken(token), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func publicParticipant(participant Participant) Participant {
	participant.SessionToken = ""
	participant.TokenHash = ""
	return participant
}

func publicConnection(connection Connection) Connection {
	connection.ConnectionToken = ""
	connection.TokenHash = ""
	return connection
}

func orderedTurnIDs(sessionTurns map[string][]string) []string {
	ids := make([]string, 0)
	for _, turnIDs := range sessionTurns {
		ids = append(ids, turnIDs...)
	}
	return ids
}

func hasScope(scopes []string, scope string) bool {
	return slices.Contains(scopes, scope)
}

func sessionStatusForParticipants(count int) SessionStatus {
	if count >= 2 {
		return SessionStatusReady
	}
	return SessionStatusWaitingParticipants
}

func debatePhase(index, maxTurns int) string {
	switch {
	case index == 0:
		return "opening"
	case index >= maxTurns-2:
		return "closing"
	default:
		return "rebuttal"
	}
}
