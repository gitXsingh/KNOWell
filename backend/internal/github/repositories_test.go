package github

import (
	"testing"

	"github.com/gitXsingh/knowell/backend/internal/common/config"
)

func TestGitHubRepositoryURL(t *testing.T) {
	url := githubRepositoryURL("acme org", "api/service")
	expected := "https://api.github.com/repos/acme%20org/api%2Fservice"
	if url != expected {
		t.Fatalf("githubRepositoryURL returned wrong value: got %q want %q", url, expected)
	}
}

func TestMapRepositorySummary(t *testing.T) {
	repo := repositoryAPIResponse{
		Name:          "knowell",
		FullName:      "gitXsingh/knowell",
		DefaultBranch: "main",
		Private:       true,
	}
	repo.Owner.Login = "gitXsingh"

	summary := mapRepositorySummary(repo)
	if summary.Owner != "gitXsingh" || summary.RepoName != "knowell" || summary.FullName != "gitXsingh/knowell" || summary.DefaultBranch != "main" || !summary.Private {
		t.Fatalf("mapRepositorySummary returned unexpected summary: %#v", summary)
	}
}

func TestWebhookRequest(t *testing.T) {
	service := &Service{cfg: config.Config{GitHubWebhookURL: "http://localhost:8080/github/webhook"}}
	request := service.webhookRequest("secret")

	if request.Name != "web" || !request.Active {
		t.Fatalf("webhookRequest returned wrong webhook metadata: %#v", request)
	}
	if request.Config["url"] != "http://localhost:8080/github/webhook" {
		t.Fatalf("webhookRequest returned wrong callback URL: %#v", request.Config)
	}
	if request.Config["secret"] != "secret" {
		t.Fatalf("webhookRequest returned wrong secret: %#v", request.Config)
	}
	if len(request.Events) != 2 || request.Events[0] != "push" || request.Events[1] != "pull_request" {
		t.Fatalf("webhookRequest returned wrong event list: %#v", request.Events)
	}
}
