package auth

import "testing"

func TestHashPasswordAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}

	if err := VerifyPassword(hash, "correct horse battery staple"); err != nil {
		t.Fatalf("VerifyPassword returned error for valid password: %v", err)
	}
}

func TestVerifyPasswordRejectsWrongPassword(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}

	err = VerifyPassword(hash, "wrong password")
	if err == nil {
		t.Fatal("VerifyPassword returned nil error for wrong password")
	}
	if err != ErrInvalidCredentials {
		t.Fatalf("VerifyPassword returned wrong error: got %v want %v", err, ErrInvalidCredentials)
	}
}

func TestVerifyPasswordRejectsInvalidStoredHash(t *testing.T) {
	err := VerifyPassword("invalid-hash", "password")
	if err == nil {
		t.Fatal("VerifyPassword returned nil error for invalid stored hash")
	}
}
