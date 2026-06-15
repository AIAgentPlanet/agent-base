package oauth

import (
	"crypto/sha256"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSecurityTokensAreUniqueAndURLSafe(t *testing.T) {
	codeA := GenerateCode()
	codeB := GenerateCode()
	require.NotEqual(t, codeA, codeB)
	require.GreaterOrEqual(t, len(codeA), 43)
	_, err := base64.RawURLEncoding.DecodeString(codeA)
	require.NoError(t, err)

	require.NotEqual(t, GenerateClientID(), GenerateClientID())
	require.NotEqual(t, GenerateClientSecret(), GenerateClientSecret())
}

func TestValidatePKCES256(t *testing.T) {
	verifier := "a-cryptographically-random-pkce-verifier"
	digest := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(digest[:])

	require.True(t, ValidatePKCE(verifier, challenge, "S256"))
	require.False(t, ValidatePKCE("wrong", challenge, "S256"))
	require.False(t, ValidatePKCE("", challenge, "S256"))
}
