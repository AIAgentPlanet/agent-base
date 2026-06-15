package jwt

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestATHTokenBindsAgentProviderAndSession(t *testing.T) {
	SetConfig("test-secret-with-sufficient-entropy", "test-issuer", 1)
	token, err := GenerateATHToken(
		42,
		"https://agent.example.com/.well-known/agent.json",
		"client-1",
		"user-service",
		"session-1",
		"handshake-1",
		"user:read",
		"access",
		300,
	)
	require.NoError(t, err)

	claims, err := ParseATHToken(token)
	require.NoError(t, err)
	require.Equal(t, uint64(42), claims.UserID)
	require.Equal(t, "https://agent.example.com/.well-known/agent.json", claims.AgentID)
	require.Equal(t, "client-1", claims.ClientID)
	require.Equal(t, "user-service", claims.ProviderID)
	require.Equal(t, "session-1", claims.SessionID)
	require.Equal(t, "handshake-1", claims.HandshakeID)
	require.Equal(t, "user:read", claims.Scope)
	require.Contains(t, claims.Audience, "user-service")
	require.NotEmpty(t, claims.ID)
}
