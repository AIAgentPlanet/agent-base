package ath

import "fmt"

// DiscoveryDocument represents the ATH discovery document
type DiscoveryDocument struct {
	GatewayURL            string            `json:"gateway_url"`
	RegistrationEndpoint  string            `json:"registration_endpoint"`
	AuthorizationEndpoint string            `json:"authorization_endpoint"`
	TokenEndpoint         string            `json:"token_endpoint"`
	RevocationEndpoint    string            `json:"revocation_endpoint"`
	AgentStatusEndpoint   string            `json:"agent_status_endpoint"`
	Providers             []ProviderInfo    `json:"providers"`
	Version               string            `json:"version"`
}

// ProviderInfo describes an available upstream provider
type ProviderInfo struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Scopes      []Scope  `json:"scopes"`
}

// Scope describes an available scope
type Scope struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ProviderApproval represents approved provider and scopes for an agent
type ProviderApproval struct {
	ProviderID string   `json:"provider_id"`
	Scopes     []string `json:"scopes"`
}

// GetDiscoveryDocument returns the ATH discovery document for the service.
func GetDiscoveryDocument(baseURL string) *DiscoveryDocument {
	return &DiscoveryDocument{
		GatewayURL:            baseURL,
		RegistrationEndpoint:  fmt.Sprintf("%s/api/v1/ath/agents/register", baseURL),
		AuthorizationEndpoint: fmt.Sprintf("%s/api/v1/ath/authorize", baseURL),
		TokenEndpoint:         fmt.Sprintf("%s/api/v1/ath/token", baseURL),
		RevocationEndpoint:    fmt.Sprintf("%s/api/v1/ath/revoke", baseURL),
		AgentStatusEndpoint:   fmt.Sprintf("%s/api/v1/ath/agents", baseURL),
		Version:               "0.1.0",
		Providers: []ProviderInfo{
			{
				ID:          "user-service",
				Name:        "User Service",
				Description: "User management and authentication APIs",
				Scopes: []Scope{
					{ID: "user:read", Name: "Read User", Description: "Read user profile and information"},
					{ID: "user:write", Name: "Write User", Description: "Create and update user information"},
					{ID: "oauth:read", Name: "Read OAuth", Description: "Read OAuth client information"},
					{ID: "oauth:write", Name: "Write OAuth", Description: "Manage OAuth clients"},
				},
			},
		},
	}
}
