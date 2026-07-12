package ai

import (
	"context"
	"testing"

	"github.com/gitXsingh/knowell/backend/internal/common/config"
)

func TestNewProviderDefaultsToBuiltin(t *testing.T) {
	provider := newProvider(config.Config{})
	if provider.Name() != "builtin" {
		t.Fatalf("newProvider returned wrong provider name: got %q want %q", provider.Name(), "builtin")
	}
}

func TestBuiltinProviderGenerateCommitDraft(t *testing.T) {
	provider := builtinProvider{}
	output, err := provider.GenerateCommitDraft(context.Background(), CommitInput{
		SHA:         "abcdef1234567890",
		Message:     "fix auth redirect loop",
		AuthorName:  "Ada Lovelace",
		AuthorEmail: "ada@example.com",
	})
	if err != nil {
		t.Fatalf("GenerateCommitDraft returned error: %v", err)
	}

	if output.SuggestedTitle != "fix auth redirect loop" {
		t.Fatalf("GenerateCommitDraft returned wrong title: got %q", output.SuggestedTitle)
	}
	if output.Importance != 4 {
		t.Fatalf("GenerateCommitDraft returned wrong importance: got %d want 4", output.Importance)
	}
	if output.RawInputJSON["source"] != "commit" {
		t.Fatalf("GenerateCommitDraft returned wrong source payload: %#v", output.RawInputJSON)
	}
}

func TestBuiltinProviderGeneratePullRequestDraft(t *testing.T) {
	provider := builtinProvider{}
	output, err := provider.GeneratePullRequestDraft(context.Background(), PullRequestInput{
		Number:       42,
		Title:        "Merge auth cleanup",
		Description:  "Reduces redirect edge cases.",
		State:        "merged",
		BaseBranch:   "main",
		MergedByName: "grace",
	})
	if err != nil {
		t.Fatalf("GeneratePullRequestDraft returned error: %v", err)
	}

	if output.SuggestedTitle != "Merge auth cleanup" {
		t.Fatalf("GeneratePullRequestDraft returned wrong title: got %q", output.SuggestedTitle)
	}
	if output.Importance != 4 {
		t.Fatalf("GeneratePullRequestDraft returned wrong importance: got %d want 4", output.Importance)
	}
	if output.RawInputJSON["source"] != "pull_request" {
		t.Fatalf("GeneratePullRequestDraft returned wrong source payload: %#v", output.RawInputJSON)
	}
}
