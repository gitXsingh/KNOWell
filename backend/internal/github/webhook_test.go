package github

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestValidateWebhookSignature(t *testing.T) {
	payload := []byte(`{"zen":"testing"}`)
	mac := hmac.New(sha256.New, []byte("secret"))
	_, _ = mac.Write(payload)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	if !validateWebhookSignature("secret", payload, signature) {
		t.Fatal("validateWebhookSignature returned false for valid signature")
	}
	if validateWebhookSignature("wrong-secret", payload, signature) {
		t.Fatal("validateWebhookSignature returned true for wrong secret")
	}
}

func TestNormalizedAction(t *testing.T) {
	push := normalizedAction("push", webhookEnvelope{})
	if push != "push" {
		t.Fatalf("normalizedAction returned wrong push action: got %q", push)
	}

	opened := normalizedAction("pull_request", webhookEnvelope{Action: "opened"})
	if opened != "opened" {
		t.Fatalf("normalizedAction returned wrong opened action: got %q", opened)
	}

	merged := normalizedAction("pull_request", webhookEnvelope{
		Action: "closed",
		PullRequest: struct {
			Merged bool `json:"merged"`
		}{Merged: true},
	})
	if merged != "merged" {
		t.Fatalf("normalizedAction returned wrong merged action: got %q", merged)
	}
}

func TestWebhookStatusFor(t *testing.T) {
	if webhookStatusFor("push", "push") != "received" {
		t.Fatal("webhookStatusFor did not mark push as received")
	}
	if webhookStatusFor("pull_request", "opened") != "received" {
		t.Fatal("webhookStatusFor did not mark PR opened as received")
	}
	if webhookStatusFor("pull_request", "synchronize") != "ignored" {
		t.Fatal("webhookStatusFor did not mark unsupported PR action as ignored")
	}
	if webhookStatusFor("issues", "opened") != "ignored" {
		t.Fatal("webhookStatusFor did not mark unsupported event as ignored")
	}
}
