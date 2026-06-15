package handler

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestATHScopeIntersectionUsesAllConstraints(t *testing.T) {
	agentApproved := []string{"user:read", "user:write", "oauth:read"}
	userConsented := []string{"user:read", "oauth:read"}
	requested := []string{"user:read", "user:write"}

	effective := intersectScopes(intersectScopes(agentApproved, userConsented), requested)
	require.Equal(t, []string{"user:read"}, effective)
}

func TestLimitTokenLifetimeToSecureSession(t *testing.T) {
	require.LessOrEqual(t, limitTokenLifetime(3600, time.Now().Add(5*time.Minute).Unix()), 300)
	require.GreaterOrEqual(t, limitTokenLifetime(3600, time.Now().Add(5*time.Minute).Unix()), 298)
	require.Equal(t, 60, limitTokenLifetime(60, time.Now().Add(time.Hour).Unix()))
	require.LessOrEqual(t, limitTokenLifetime(3600, time.Now().Add(-time.Minute).Unix()), 0)
}

func TestAppendUnique(t *testing.T) {
	values := appendUnique(nil, "user:read")
	values = appendUnique(values, "user:read")
	values = appendUnique(values, "user:write")
	require.Equal(t, []string{"user:read", "user:write"}, values)
}

func TestInterfaceStrings(t *testing.T) {
	require.Equal(t, []string{"user:read", "user:write"}, interfaceStrings([]interface{}{"user:read", 42, "user:write"}))
	require.Nil(t, interfaceStrings("user:read"))
}
