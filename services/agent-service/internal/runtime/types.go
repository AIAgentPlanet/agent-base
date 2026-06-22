package runtime

import "time"

type AgentStatus string

const (
	AgentStatusPendingATH AgentStatus = "pending_ath"
	AgentStatusVerified   AgentStatus = "verified"
	AgentStatusBound      AgentStatus = "bound_to_user"
	AgentStatusRevoked    AgentStatus = "revoked"
)

type ConnectionMode string

const (
	ConnectionHTTPSWebhook ConnectionMode = "https_webhook"
	ConnectionWebSocket    ConnectionMode = "websocket"
	ConnectionMCP          ConnectionMode = "mcp"
)

type Agent struct {
	ID             string            `json:"id"`
	UserID         string            `json:"user_id"`
	Type           string            `json:"type"`
	Identity       string            `json:"identity"`
	DisplayName    string            `json:"display_name"`
	ConnectionMode ConnectionMode    `json:"connection_mode"`
	Capabilities   []string          `json:"capabilities"`
	Status         AgentStatus       `json:"status"`
	Metadata       map[string]string `json:"metadata,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

type ConnectionStatus string

const (
	ConnectionStatusOnline  ConnectionStatus = "online"
	ConnectionStatusOffline ConnectionStatus = "offline"
	ConnectionStatusRevoked ConnectionStatus = "revoked"
)

type Connection struct {
	ID              string            `json:"id"`
	AgentID         string            `json:"agent_id"`
	UserID          string            `json:"user_id"`
	Mode            ConnectionMode    `json:"mode"`
	Status          ConnectionStatus  `json:"status"`
	ConnectionToken string            `json:"connection_token,omitempty"`
	TokenHash       string            `json:"-"`
	Metadata        map[string]string `json:"metadata,omitempty"`
	LastSeenAt      time.Time         `json:"last_seen_at"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

type SessionStatus string

const (
	SessionStatusCreated             SessionStatus = "created"
	SessionStatusWaitingParticipants SessionStatus = "waiting_participants"
	SessionStatusReady               SessionStatus = "ready"
	SessionStatusActive              SessionStatus = "active"
	SessionStatusCompleted           SessionStatus = "completed"
	SessionStatusCancelled           SessionStatus = "cancelled"
)

type ParticipantStatus string

const (
	ParticipantStatusPendingAuthorization ParticipantStatus = "pending_authorization"
	ParticipantStatusReady                ParticipantStatus = "ready"
	ParticipantStatusRevoked              ParticipantStatus = "revoked"
)

type Session struct {
	ID           string            `json:"id"`
	Type         string            `json:"type"`
	OwnerUserID  string            `json:"owner_user_id"`
	Status       SessionStatus     `json:"status"`
	Policy       SessionPolicy     `json:"policy"`
	Participants []Participant     `json:"participants"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

type SessionPolicy struct {
	TurnPolicy          string   `json:"turn_policy"`
	MaxTurns            int      `json:"max_turns"`
	TurnTimeoutSeconds  int      `json:"turn_timeout_seconds,omitempty"`
	AllowedMessageTypes []string `json:"allowed_message_types,omitempty"`
	AuditRequired       bool     `json:"audit_required"`
}

type Participant struct {
	ID            string            `json:"id"`
	SessionID     string            `json:"session_id"`
	UserID        string            `json:"user_id"`
	AgentID       string            `json:"agent_id"`
	AgentIdentity string            `json:"agent_identity"`
	Role          string            `json:"role"`
	Scopes        []string          `json:"scopes"`
	Status        ParticipantStatus `json:"status"`
	SessionToken  string            `json:"session_token,omitempty"`
	TokenHash     string            `json:"-"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

type TurnStatus string

const (
	TurnStatusOpen      TurnStatus = "open"
	TurnStatusSubmitted TurnStatus = "submitted"
	TurnStatusSkipped   TurnStatus = "skipped"
)

type Turn struct {
	ID            string     `json:"id"`
	SessionID     string     `json:"session_id"`
	ParticipantID string     `json:"participant_id"`
	Index         int        `json:"index"`
	Phase         string     `json:"phase"`
	Status        TurnStatus `json:"status"`
	DeadlineAt    *time.Time `json:"deadline_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type Message struct {
	ID            string    `json:"id"`
	SessionID     string    `json:"session_id"`
	ParticipantID string    `json:"participant_id"`
	TurnID        string    `json:"turn_id"`
	Type          string    `json:"type"`
	Content       string    `json:"content"`
	AuditRef      string    `json:"audit_ref,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

type DeliveryStatus string

const (
	DeliveryStatusPending DeliveryStatus = "pending"
	DeliveryStatusAcked   DeliveryStatus = "acked"
)

type Delivery struct {
	ID            string         `json:"id"`
	ConnectionID  string         `json:"connection_id"`
	AgentID       string         `json:"agent_id"`
	SessionID     string         `json:"session_id"`
	ParticipantID string         `json:"participant_id"`
	TurnID        string         `json:"turn_id"`
	Type          string         `json:"type"`
	Status        DeliveryStatus `json:"status"`
	Payload       map[string]any `json:"payload"`
	AckedAt       *time.Time     `json:"acked_at,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

type AuditStatus struct {
	SessionID        string   `json:"session_id"`
	Status           string   `json:"status"`
	Message          string   `json:"message"`
	ATHConfigured    bool     `json:"ath_configured"`
	HandshakeID      string   `json:"handshake_id,omitempty"`
	MessageAuditRefs []string `json:"message_audit_refs,omitempty"`
	Verification     any      `json:"verification,omitempty"`
	Records          any      `json:"records,omitempty"`
}
