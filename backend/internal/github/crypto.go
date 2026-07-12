package github

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
)

func encryptToken(secret, plainText string) (string, error) {
	key := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	cipherText := gcm.Seal(nonce, nonce, []byte(plainText), nil)
	return base64.RawURLEncoding.EncodeToString(cipherText), nil
}

func decryptToken(secret, encryptedText string) (string, error) {
	key := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	cipherText, err := base64.RawURLEncoding.DecodeString(encryptedText)
	if err != nil {
		return "", err
	}
	if len(cipherText) < gcm.NonceSize() {
		return "", fmt.Errorf("encrypted token is invalid")
	}

	nonce := cipherText[:gcm.NonceSize()]
	data := cipherText[gcm.NonceSize():]
	plainText, err := gcm.Open(nil, nonce, data, nil)
	if err != nil {
		return "", err
	}

	return string(plainText), nil
}
