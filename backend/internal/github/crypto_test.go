package github

import "testing"

func TestEncryptAndDecryptTokenRoundTrip(t *testing.T) {
	encrypted, err := encryptToken("secret", "ghp_test_token")
	if err != nil {
		t.Fatalf("encryptToken returned error: %v", err)
	}

	decrypted, err := decryptToken("secret", encrypted)
	if err != nil {
		t.Fatalf("decryptToken returned error: %v", err)
	}
	if decrypted != "ghp_test_token" {
		t.Fatalf("decryptToken returned wrong value: got %q", decrypted)
	}
}

func TestDecryptTokenRejectsWrongSecret(t *testing.T) {
	encrypted, err := encryptToken("secret", "ghp_test_token")
	if err != nil {
		t.Fatalf("encryptToken returned error: %v", err)
	}

	if _, err := decryptToken("different-secret", encrypted); err == nil {
		t.Fatal("decryptToken returned nil error for wrong secret")
	}
}
