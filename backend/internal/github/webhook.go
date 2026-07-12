package github

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type webhookEnvelope struct {
	Action     string `json:"action"`
	Repository struct {
		Name  string `json:"name"`
		Owner struct {
			Login string `json:"login"`
		} `json:"owner"`
	} `json:"repository"`
	PullRequest struct {
		Merged bool `json:"merged"`
	} `json:"pull_request"`
}

func (s *Service) handleWebhook(w http.ResponseWriter, r *http.Request) {
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_payload", "Webhook payload could not be read")
		return
	}

	if len(payload) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_payload", "Webhook payload is required")
		return
	}

	var envelope webhookEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_payload", "Webhook payload is invalid")
		return
	}

	repository, err := s.findRepositoryByName(r.Context(), strings.TrimSpace(envelope.Repository.Owner.Login), strings.TrimSpace(envelope.Repository.Name))
	if err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "repository_not_found", "Repository is not connected")
			return
		}
		writeError(w, http.StatusInternalServerError, "webhook_error", "Webhook processing failed")
		return
	}

	signatureValid := validateWebhookSignature(repository.WebhookSecret, payload, r.Header.Get("X-Hub-Signature-256"))
	if !signatureValid {
		writeError(w, http.StatusUnauthorized, "invalid_signature", "Webhook signature is invalid")
		return
	}

	eventType := strings.TrimSpace(r.Header.Get("X-GitHub-Event"))
	deliveryID := strings.TrimSpace(r.Header.Get("X-GitHub-Delivery"))
	action := normalizedAction(eventType, envelope)
	status := webhookStatusFor(eventType, action)

	if err := s.storeWebhookEvent(r.Context(), repository.ID, deliveryID, eventType, action, payload, status); err != nil {
		writeError(w, http.StatusInternalServerError, "webhook_error", "Webhook could not be stored")
		return
	}

	if status == "received" && s.processor != nil {
		if err := s.processor.ProcessRepositoryDelivery(r.Context(), repository.ID, deliveryID); err != nil {
			writeError(w, http.StatusInternalServerError, "webhook_error", "Webhook could not be processed")
			return
		}
	}

	writeJSON(w, http.StatusAccepted, map[string]string{
		"status":          status,
		"event_type":      eventType,
		"repository_id":   repository.ID,
		"github_delivery": deliveryID,
	})
}

type storedRepository struct {
	ID            string
	WebhookSecret string
}

func (s *Service) findRepositoryByName(ctx context.Context, owner, repo string) (*storedRepository, error) {
	var record storedRepository
	err := s.db.QueryRowContext(ctx, `
		SELECT id, webhook_secret
		FROM repositories
		WHERE provider = 'github' AND lower(owner) = lower($1) AND lower(repo_name) = lower($2)
	`, owner, repo).Scan(&record.ID, &record.WebhookSecret)
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (s *Service) storeWebhookEvent(ctx context.Context, repositoryID, deliveryID, eventType, action string, payload []byte, status string) error {
	if strings.TrimSpace(deliveryID) == "" {
		deliveryID = randomNonce()
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO webhook_events (
			repository_id,
			github_delivery_id,
			event_type,
			action,
			signature_valid,
			payload_json,
			processing_status
		)
		VALUES ($1, $2, $3, $4, true, $5, $6)
		ON CONFLICT (repository_id, github_delivery_id) DO UPDATE
		SET action = EXCLUDED.action,
			signature_valid = EXCLUDED.signature_valid,
			payload_json = EXCLUDED.payload_json,
			processing_status = EXCLUDED.processing_status
	`, repositoryID, deliveryID, eventType, action, payload, status)
	return err
}

func validateWebhookSignature(secret string, payload []byte, header string) bool {
	if strings.TrimSpace(secret) == "" || strings.TrimSpace(header) == "" {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(payload)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(strings.TrimSpace(header)))
}

func normalizedAction(eventType string, envelope webhookEnvelope) string {
	switch eventType {
	case "push":
		return "push"
	case "pull_request":
		if envelope.Action == "opened" {
			return "opened"
		}
		if envelope.Action == "closed" && envelope.PullRequest.Merged {
			return "merged"
		}
		return strings.TrimSpace(envelope.Action)
	default:
		return strings.TrimSpace(envelope.Action)
	}
}

func webhookStatusFor(eventType, action string) string {
	switch eventType {
	case "push":
		return "received"
	case "pull_request":
		if action == "opened" || action == "merged" {
			return "received"
		}
		return "ignored"
	default:
		return "ignored"
	}
}

func webhookSummaryMessage(eventType, action string) string {
	return fmt.Sprintf("%s:%s", eventType, action)
}
