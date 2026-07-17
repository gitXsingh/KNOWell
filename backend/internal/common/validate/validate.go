package validate

import (
	"fmt"
	"net/mail"
	"strings"
)

func Email(email string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return fmt.Errorf("email is required")
	}
	if len(email) > 255 {
		return fmt.Errorf("email must be at most 255 characters")
	}
	_, err := mail.ParseAddress(email)
	if err != nil {
		return fmt.Errorf("invalid email format")
	}
	return nil
}

func Password(password string) error {
	if password == "" {
		return fmt.Errorf("password is required")
	}
	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	if len(password) > 128 {
		return fmt.Errorf("password must be at most 128 characters")
	}
	return nil
}

func Name(name string, maxLen int) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("name is required")
	}
	if len(name) > maxLen {
		return fmt.Errorf("name must be at most %d characters", maxLen)
	}
	return nil
}

func StringLen(value string, field string, maxLen int) error {
	if len(value) > maxLen {
		return fmt.Errorf("%s must be at most %d characters", field, maxLen)
	}
	return nil
}
