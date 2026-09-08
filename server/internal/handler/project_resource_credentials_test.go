package handler

import "testing"

func TestGitRepositoryURLCannotCarryCredentials(t *testing.T) {
	for _, raw := range []string{
		"https://fixture-token@example.com/repo.git",
		"https://fixture-user:fixture-password@example.com/repo.git",
		"http://fixture-token@example.com/repo.git",
		"ssh://git:fixture-password@example.com/repo.git",
		"https://example.com/repo.git?access_token=fixture-token",
		"https://example.com/repo.git#fixture-token",
	} {
		if isValidGitRepoURL(raw) {
			t.Errorf("credential-bearing repository URL accepted: %q", raw)
		}
	}
	for _, raw := range []string{"https://example.com/repo.git", "ssh://git@example.com/repo.git", "git@example.com:repo.git"} {
		if !isValidGitRepoURL(raw) {
			t.Errorf("credential-free repository URL rejected: %q", raw)
		}
	}
}
