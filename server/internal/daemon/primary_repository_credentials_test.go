package daemon

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPrimaryRepositoryRejectsLegacyCredentialURLs(t *testing.T) {
	for _, raw := range []string{
		"https://fixture-token@example.com/repo.git",
		"https://fixture-user:fixture-password@example.com/repo.git",
		"ssh://git:fixture-password@example.com/repo.git",
		"https://example.com/repo.git?access_token=fixture-token",
		"https://example.com/repo.git#fixture-token",
	} {
		repo := RepoData{URL: raw, Ref: "main"}
		data, err := json.Marshal(repo)
		if err != nil {
			t.Fatal(err)
		}
		for _, resources := range [][]ProjectResourceData{nil, {{ResourceType: "github_repo", ResourceRef: data}}} {
			task := Task{IssueID: "issue", Repos: []RepoData{repo}, ProjectResources: resources}
			selected, err := primaryRepositoryForTask(task)
			if err == nil || selected != nil {
				t.Fatalf("credential-bearing primary repository accepted")
			}
			if strings.Contains(err.Error(), "fixture-token") || strings.Contains(err.Error(), "fixture-password") {
				t.Fatal("credential leaked in selection error")
			}
		}
	}
}

func TestPrimaryRepositoryPromptDoesNotExposeLegacyURL(t *testing.T) {
	state := &preparedPrimaryRepository{URL: "https://fixture-user:fixture-password@example.com/repo.git?access_token=fixture-token", WorkDir: "/tmp/checkout", Ref: "main"}
	for _, prompt := range []string{
		BuildPrompt(Task{IssueID: "issue"}, "codex", WithPrimaryRepository(state)),
		BuildDirectPrompt(Task{IssueID: "issue"}, WithPrimaryRepository(state)),
	} {
		for _, secret := range []string{"fixture-user", "fixture-password", "fixture-token"} {
			if strings.Contains(prompt, secret) {
				t.Fatalf("primary repository prompt exposed %s", secret)
			}
		}
		if !strings.Contains(prompt, "already prepared") || !strings.Contains(prompt, "/tmp/checkout") {
			t.Fatal("safe primary repository context was lost")
		}
	}
}
