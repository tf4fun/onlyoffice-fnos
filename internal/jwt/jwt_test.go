package jwt

import (
	"testing"
	"time"

	"pgregory.net/rapid"
)

// Property 10: JWT 密钥随机性
// *For any* 两次调用 GenerateSecret()，生成的密钥应不同
// **Validates: Requirements 4.2**
func TestProperty10_JWTSecretRandomness(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		m := NewManager()

		secret1 := m.GenerateSecret()
		secret2 := m.GenerateSecret()

		if secret1 == secret2 {
			t.Fatalf("GenerateSecret() should produce different values, got same: %s", secret1)
		}

		// Verify length is 32 characters (16 bytes hex encoded)
		if len(secret1) != 32 {
			t.Fatalf("GenerateSecret() should produce 32-character string, got %d", len(secret1))
		}
		if len(secret2) != 32 {
			t.Fatalf("GenerateSecret() should produce 32-character string, got %d", len(secret2))
		}
	})
}

// Property 2: JWT 签名验证
// *For any* 无效或过期的 JWT，验证应失败
// **Validates: Requirements 2.1, 4.2**
func TestProperty2_JWTSignatureVerification(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		m := NewManager()

		// Generate random secret and claims
		secret := m.GenerateSecret()
		wrongSecret := m.GenerateSecret()

		claims := map[string]interface{}{
			"key": rapid.String().Draw(t, "claimKey"),
		}

		// Sign with correct secret
		token, err := m.Sign(secret, claims)
		if err != nil {
			t.Fatalf("failed to sign token: %v", err)
		}

		// Verify with wrong secret should fail
		_, err = m.Verify(wrongSecret, token)
		if err == nil {
			t.Fatal("verification with wrong secret should fail")
		}

		// Verify with correct secret should succeed
		_, err = m.Verify(secret, token)
		if err != nil {
			t.Fatalf("verification with correct secret should succeed: %v", err)
		}

		// Verify invalid token format should fail
		_, err = m.Verify(secret, "invalid.token.format")
		if err == nil {
			t.Fatal("verification of invalid token should fail")
		}

		// Verify empty token should fail
		_, err = m.Verify(secret, "")
		if err == nil {
			t.Fatal("verification of empty token should fail")
		}
	})
}

// Property 2 (continued): Expired JWT verification should fail
func TestProperty2_ExpiredJWTVerification(t *testing.T) {
	m := NewManager()
	secret := m.GenerateSecret()

	claims := map[string]interface{}{
		"data": "test",
	}

	// Sign with very short expiry (already expired)
	token, err := m.SignWithExpiry(secret, claims, -1*time.Hour)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	// Verify expired token should fail
	_, err = m.Verify(secret, token)
	if err != ErrExpiredToken {
		t.Fatalf("verification of expired token should return ErrExpiredToken, got: %v", err)
	}
}

// Unit test: Sign and Verify round-trip
func TestSignVerifyRoundTrip(t *testing.T) {
	m := NewManager()
	secret := m.GenerateSecret()

	claims := map[string]interface{}{
		"user_id": "123",
		"name":    "test user",
	}

	token, err := m.Sign(secret, claims)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	verified, err := m.Verify(secret, token)
	if err != nil {
		t.Fatalf("failed to verify token: %v", err)
	}

	if verified["user_id"] != claims["user_id"] {
		t.Errorf("user_id mismatch: expected %v, got %v", claims["user_id"], verified["user_id"])
	}
	if verified["name"] != claims["name"] {
		t.Errorf("name mismatch: expected %v, got %v", claims["name"], verified["name"])
	}
}

// Unit test: GenerateSecret produces valid hex strings
func TestGenerateSecretFormat(t *testing.T) {
	m := NewManager()

	for i := 0; i < 10; i++ {
		secret := m.GenerateSecret()

		// Check length
		if len(secret) != 32 {
			t.Errorf("secret length should be 32, got %d", len(secret))
		}

		// Check it's valid hex
		for _, c := range secret {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				t.Errorf("secret contains invalid hex character: %c", c)
			}
		}
	}
}
