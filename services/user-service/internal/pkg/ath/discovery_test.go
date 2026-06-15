package ath

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDiscoveryDocumentV01Contract(t *testing.T) {
	document := GetDiscoveryDocument("https://gateway.example.com")
	raw, err := json.Marshal(document)
	require.NoError(t, err)

	var fields map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &fields))
	for _, required := range []string{
		"ath_version",
		"gateway_id",
		"agent_registration_endpoint",
		"supported_providers",
	} {
		require.Contains(t, fields, required)
	}
	require.NotContains(t, fields, "version")
	require.NotContains(t, fields, "providers")
	require.Equal(t, "0.1", document.ATHVersion)
	require.Equal(t, "https://gateway.example.com/api/v1/ath/agents/register", document.AgentRegistrationEndpoint)
	require.Equal(t, "https://gateway.example.com/api/v1/ath/handshakes", document.HandshakeEndpoint)
	require.Contains(t, document.SupportedCapabilities, "ECDH-P256")
	require.Contains(t, document.SupportedCapabilities, "HMAC-SHA256")
	require.Contains(t, document.SupportedCapabilities, "SIGNING-KEY-ROTATION")
	require.Contains(t, document.SupportedCapabilities, "EXTERNAL-ANCHOR")
	require.Equal(t, 300, document.RequestFreshnessSeconds)
	require.Equal(t, "https://gateway.example.com/.well-known/ath-audit-head.json", document.AuditHeadEndpoint)
	require.True(t, SupportedScope("user-service", "user:read"))
	require.False(t, SupportedScope("user-service", "admin:all"))
}
