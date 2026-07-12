package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gitXsingh/knowell/backend/internal/common/config"
)

const ollamaRequestTimeout = 120 * time.Second

type ollamaProvider struct {
	baseURL string
	model   string
	client  *http.Client
}

func newProvider(cfg config.Config) Provider {
	if cfg.AIProvider == "ollama" {
		return &ollamaProvider{
			baseURL: strings.TrimRight(cfg.OllamaBaseURL, "/"),
			model:   cfg.OllamaModel,
			client:  &http.Client{Timeout: ollamaRequestTimeout},
		}
	}
	return builtinProvider{}
}

type ollamaChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaChatRequest struct {
	Model    string              `json:"model"`
	Messages []ollamaChatMessage `json:"messages"`
	Stream   bool                `json:"stream"`
}

type ollamaChatResponse struct {
	Message ollamaChatMessage `json:"message"`
	Done    bool              `json:"done"`
}

func (o *ollamaProvider) Name() string {
	return "ollama"
}

func (o *ollamaProvider) GenerateCommitDraft(ctx context.Context, input CommitInput) (*DraftOutput, error) {
	prompt := fmt.Sprintf(`You are a technical documentation assistant analyzing a git commit.

First, determine if this change is meaningful:
- If the commit is trivial (typo fix, formatting, whitespace, dependency update, README edit, comment fix), set "importance": 0.
- If the commit is meaningful, generate full knowledge.

For meaningful commits, generate these fields:
- "title": a concise descriptive title (max 80 chars)
- "summary": a 1-3 sentence explanation of what this commit does and why it matters
- "importance": integer 1-4 (1=trivial-but-noted, 2=minor, 3=significant, 4=critical)
- "reason": a short justification for the importance rating
- "decision_body": A RICH engineering decision record for teammates. Include what changed, why (context/problem), how it was implemented, trade-offs considered, impact on the project, related features. Use full markdown. Write in first-person plural ("We decided...", "We chose..."). Minimum 3 paragraphs.
- "agents_md": An AGENTS.md entry formatted for AI agent onboarding. Include brief description, files affected, conventions established, rules to follow, patterns introduced. Use concise bullet-point style.

Commit SHA: %s
Author: %s <%s>
Message: %s

Respond with ONLY valid JSON in this format:
{"title":"...","summary":"...","importance":N,"reason":"...","decision_body":"...","agents_md":"..."}
If importance is 0, return: {"importance":0}`,
		trimSHA(input.SHA), safeAuthor(input.AuthorName, input.AuthorEmail), input.AuthorEmail, input.Message)

	return o.generate(ctx, prompt, map[string]any{"source": "commit", "sha": input.SHA})
}

func (o *ollamaProvider) GeneratePullRequestDraft(ctx context.Context, input PullRequestInput) (*DraftOutput, error) {
	prompt := fmt.Sprintf(`You are a technical documentation assistant analyzing a pull request.

First, determine if this change is meaningful:
- If the PR is trivial (typo fix, formatting, dependency update, minor docs), set "importance": 0.
- If the PR is meaningful, generate full knowledge.

For meaningful PRs, generate these fields:
- "title": a concise descriptive title (max 80 chars)
- "summary": a 1-3 sentence explanation of what this PR does and why it matters
- "importance": integer 1-4 (1=trivial-but-noted, 2=minor, 3=significant, 4=critical)
- "reason": a short justification for the importance rating
- "decision_body": A RICH engineering decision record for teammates. Include what changed, why (context/problem), how it was implemented, trade-offs considered, impact on the project, related features. Use full markdown. Write in first-person plural ("We decided...", "We chose..."). Minimum 3 paragraphs.
- "agents_md": An AGENTS.md entry formatted for AI agent onboarding. Include brief description, files affected, conventions established, rules to follow, patterns introduced. Use concise bullet-point style.

PR #%d
Title: %s
Description: %s
Branch: %s
State: %s

Respond with ONLY valid JSON in this format:
{"title":"...","summary":"...","importance":N,"reason":"...","decision_body":"...","agents_md":"..."}
If importance is 0, return: {"importance":0}`,
		input.Number, input.Title, input.Description, input.BaseBranch, input.State)

	return o.generate(ctx, prompt, map[string]any{"source": "pull_request", "number": input.Number})
}

func (o *ollamaProvider) generate(ctx context.Context, prompt string, rawInput map[string]any) (*DraftOutput, error) {
	reqBody := ollamaChatRequest{
		Model: o.model,
		Messages: []ollamaChatMessage{
			{Role: "system", Content: "You are a helpful AI that analyzes code changes and generates structured knowledge drafts. Always respond with valid JSON only, no other text."},
			{Role: "user", Content: prompt},
		},
		Stream: false,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("ollama: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ollama: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ollama: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ollama: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama: API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var chatResp ollamaChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("ollama: parse response: %w", err)
	}

	return parseDraftOutput(chatResp.Message.Content, rawInput)
}

func parseDraftOutput(content string, rawInput map[string]any) (*DraftOutput, error) {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var result struct {
		Title        string `json:"title"`
		Summary      string `json:"summary"`
		Importance   int    `json:"importance"`
		Reason       string `json:"reason"`
		DecisionBody string `json:"decision_body"`
		AgentsMd     string `json:"agents_md"`
	}
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("ollama: parse draft output: %w", err)
	}

	if result.Importance > 4 {
		result.Importance = 4
	}

	return &DraftOutput{
		SuggestedTitle: result.Title,
		Summary:        result.Summary,
		Importance:     result.Importance,
		Reason:         result.Reason,
		DecisionBody:   result.DecisionBody,
		AgentsMd:       result.AgentsMd,
		RawInputJSON:   rawInput,
	}, nil
}

type builtinProvider struct{}

func (builtinProvider) Name() string {
	return "builtin"
}

func (builtinProvider) GenerateCommitDraft(ctx context.Context, input CommitInput) (*DraftOutput, error) {
	_ = ctx

	title := strings.TrimSpace(input.Message)
	if title == "" {
		title = "Commit " + trimSHA(input.SHA)
	}

	categoryReason := "Captured from a repository push event."
	importance := 2
	lowered := strings.ToLower(input.Message)
	switch {
	case strings.Contains(lowered, "fix"), strings.Contains(lowered, "bug"):
		importance = 4
		categoryReason = "Commit message suggests a bug fix or corrective change."
	case strings.Contains(lowered, "refactor"), strings.Contains(lowered, "cleanup"):
		importance = 2
		categoryReason = "Commit message suggests maintenance or structural cleanup."
	case strings.Contains(lowered, "feat"), strings.Contains(lowered, "add"):
		importance = 3
		categoryReason = "Commit message suggests a new feature or meaningful behavior change."
	}

	summary := "Commit " + trimSHA(input.SHA) + " by " + safeAuthor(input.AuthorName, input.AuthorEmail) + " updated the project with message: " + strings.TrimSpace(input.Message) + "."
	decisionBody := fmt.Sprintf("## Decision: %s\n\n**Context:** This commit was authored by %s.\n\n**Change:** %s\n\n**Impact:** See commit message for details.", title, safeAuthor(input.AuthorName, input.AuthorEmail), input.Message)
	agentsMd := fmt.Sprintf("- **%s**: %s\n  - Files affected: commit %s\n  - Importance: %d/4", title, input.Message, trimSHA(input.SHA), importance)
	return &DraftOutput{
		SuggestedTitle: title,
		Summary:        summary,
		Importance:     importance,
		Reason:         categoryReason,
		DecisionBody:   decisionBody,
		AgentsMd:       agentsMd,
		RawInputJSON: map[string]any{
			"source": "commit",
		},
	}, nil
}

func (builtinProvider) GeneratePullRequestDraft(ctx context.Context, input PullRequestInput) (*DraftOutput, error) {
	_ = ctx

	title := strings.TrimSpace(input.Title)
	if title == "" {
		title = "Pull request #" + intToString(input.Number)
	}

	importance := 3
	reason := "Captured from a pull request webhook event."
	if input.State == "merged" {
		importance = 4
		reason = "Merged pull requests usually represent reviewed project knowledge worth preserving."
	}

	summary := "Pull request #" + intToString(input.Number) + " on " + input.BaseBranch + " is " + input.State + ". " + strings.TrimSpace(input.Description)
	if strings.TrimSpace(input.MergedByName) != "" {
		summary += " Merged by " + input.MergedByName + "."
	}

	decisionBody := fmt.Sprintf("## PR #%d: %s\n\n**Context:** This PR was %s on branch `%s`.\n\n**Description:** %s\n\n**Impact:** Merged by %s.", input.Number, input.Title, input.State, input.BaseBranch, input.Description, input.MergedByName)
	agentsMd := fmt.Sprintf("- **PR #%d - %s**: %s\n  - Branch: `%s`\n  - State: %s\n  - Importance: %d/4", input.Number, input.Title, input.Description, input.BaseBranch, input.State, importance)
	return &DraftOutput{
		SuggestedTitle: title,
		Summary:        strings.TrimSpace(summary),
		Importance:     importance,
		Reason:         reason,
		DecisionBody:   decisionBody,
		AgentsMd:       agentsMd,
		RawInputJSON: map[string]any{
			"source": "pull_request",
		},
	}, nil
}

func trimSHA(sha string) string {
	sha = strings.TrimSpace(sha)
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}

func safeAuthor(name, email string) string {
	name = strings.TrimSpace(name)
	if name != "" {
		return name
	}
	email = strings.TrimSpace(email)
	if email != "" {
		return email
	}
	return "unknown author"
}

func intToString(value int) string {
	return strconv.Itoa(value)
}
