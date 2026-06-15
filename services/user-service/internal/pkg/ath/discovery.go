package ath

import "fmt"

// DiscoveryDocument represents the ATH discovery document
type DiscoveryDocument struct {
	ATHVersion                string         `json:"ath_version"`
	GatewayID                 string         `json:"gateway_id"`
	AgentRegistrationEndpoint string         `json:"agent_registration_endpoint"`
	HandshakeEndpoint         string         `json:"handshake_endpoint"`
	AuthorizationEndpoint     string         `json:"authorization_endpoint"`
	TokenEndpoint             string         `json:"token_endpoint"`
	RevocationEndpoint        string         `json:"revocation_endpoint"`
	AgentStatusEndpoint       string         `json:"agent_status_endpoint"`
	SupportedCapabilities     []string       `json:"supported_capabilities"`
	RequestFreshnessSeconds   int            `json:"request_freshness_seconds"`
	AuditQueryEndpoint        string         `json:"audit_query_endpoint"`
	AuditVerificationEndpoint string         `json:"audit_verification_endpoint"`
	AuditHeadEndpoint         string         `json:"audit_head_endpoint"`
	AnchorStatusEndpoint      string         `json:"anchor_status_endpoint"`
	SupportedProviders        []ProviderInfo `json:"supported_providers"`
}

// ProviderInfo describes an available upstream provider
type ProviderInfo struct {
	ProviderID            string   `json:"provider_id"`
	DisplayName           string   `json:"display_name"`
	Description           string   `json:"description,omitempty"`
	Categories            []string `json:"categories,omitempty"`
	AvailableScopes       []string `json:"available_scopes"`
	AuthMode              string   `json:"auth_mode"`
	AgentApprovalRequired bool     `json:"agent_approval_required"`
}

// Scope describes an available scope
type Scope struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ProviderApproval represents approved provider and scopes for an agent
type ProviderApproval struct {
	ProviderID     string   `json:"provider_id"`
	ApprovedScopes []string `json:"approved_scopes"`
	DeniedScopes   []string `json:"denied_scopes"`
	DenialReason   string   `json:"denial_reason,omitempty"`
}

// GetDiscoveryDocument returns the ATH discovery document for the service.
func GetDiscoveryDocument(baseURL string) *DiscoveryDocument {
	return &DiscoveryDocument{
		ATHVersion:                "0.1",
		GatewayID:                 baseURL,
		AgentRegistrationEndpoint: fmt.Sprintf("%s/api/v1/ath/agents/register", baseURL),
		HandshakeEndpoint:         fmt.Sprintf("%s/api/v1/ath/handshakes", baseURL),
		AuthorizationEndpoint:     fmt.Sprintf("%s/api/v1/ath/authorize", baseURL),
		TokenEndpoint:             fmt.Sprintf("%s/api/v1/ath/token", baseURL),
		RevocationEndpoint:        fmt.Sprintf("%s/api/v1/ath/revoke", baseURL),
		AgentStatusEndpoint:       fmt.Sprintf("%s/api/v1/ath/agents", baseURL),
		SupportedCapabilities: []string{
			"ES256", "SHA-256", "OAuth2", "PKCE-S256",
			"ECDH-P256", "HKDF-SHA256", "HMAC-SHA256",
			"SIGNING-KEY-ROTATION", "AUDIT-OUTBOX", "EXTERNAL-ANCHOR",
		},
		RequestFreshnessSeconds:   300,
		AuditQueryEndpoint:        fmt.Sprintf("%s/api/v1/ath/audit/query", baseURL),
		AuditVerificationEndpoint: fmt.Sprintf("%s/api/v1/ath/audit/verify", baseURL),
		AuditHeadEndpoint:         fmt.Sprintf("%s/.well-known/ath-audit-head.json", baseURL),
		AnchorStatusEndpoint:      fmt.Sprintf("%s/api/v1/ath/audit/anchor/status", baseURL),
		SupportedProviders: []ProviderInfo{
			{
				ProviderID:            "user-service",
				DisplayName:           "User Service",
				Description:           "User management and authentication APIs",
				Categories:            []string{"identity", "authentication"},
				AvailableScopes:       []string{"user:read", "user:write", "oauth:read", "oauth:write"},
				AuthMode:              "OAUTH2",
				AgentApprovalRequired: true,
			},
		},
	}
}

func SupportedScope(providerID, scope string) bool {
	for _, provider := range GetDiscoveryDocument("").SupportedProviders {
		if provider.ProviderID != providerID {
			continue
		}
		for _, available := range provider.AvailableScopes {
			if available == scope {
				return true
			}
		}
	}
	return false
}
