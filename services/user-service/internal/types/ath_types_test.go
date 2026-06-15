package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestATHRegisterRequestOfficialAndLegacyAliases(t *testing.T) {
	var official ATHRegisterRequest
	require.NoError(t, json.Unmarshal([]byte(`{
		"agent_id":"https://agent.example/agent.json",
		"agent_attestation":"official-token",
		"developer":{"name":"Example","id":"dev-1"},
		"requested_providers":[{"provider_id":"user-service","scopes":["user:read"]}]
	}`), &official))
	require.Equal(t, "official-token", official.EffectiveAttestation())
	require.Len(t, official.EffectiveProviders(), 1)

	var legacy ATHRegisterRequest
	require.NoError(t, json.Unmarshal([]byte(`{
		"agent_id":"https://agent.example/agent.json",
		"attestation":"legacy-token",
		"developer":{"name":"Example","id":"dev-1"},
		"providers":[{"provider_id":"user-service","scopes":["user:read"]}]
	}`), &legacy))
	require.Equal(t, "legacy-token", legacy.EffectiveAttestation())
	require.Len(t, legacy.EffectiveProviders(), 1)
}

func TestATHAuthorizeRedirectAlias(t *testing.T) {
	request := ATHAuthorizeRequest{
		UserRedirectURI: "https://agent.example/callback",
		RedirectURI:     "https://legacy.example/callback",
	}
	require.Equal(t, "https://agent.example/callback", request.EffectiveRedirectURI())
}
