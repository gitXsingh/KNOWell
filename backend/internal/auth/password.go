package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

func HashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	hash := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
	return fmt.Sprintf("%s:%s", base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(hash)), nil
}

func VerifyPassword(encodedHash, password string) error {
	parts := strings.Split(encodedHash, ":")
	if len(parts) != 2 {
		return errors.New("invalid stored password hash")
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[0])
	if err != nil {
		return err
	}

	expected, err := base64.RawStdEncoding.DecodeString(parts[1])
	if err != nil {
		return err
	}

	actual := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, uint32(len(expected)))
	if subtle.ConstantTimeCompare(actual, expected) != 1 {
		return ErrInvalidCredentials
	}

	return nil
}
