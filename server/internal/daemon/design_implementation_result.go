package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/multica-ai/multica/server/internal/daemon/repocache"
	"github.com/multica-ai/multica/server/internal/designimplementation"
)

func collectDesignImplementationReceipt(task Task, workDir string, now time.Time) (*designimplementation.Receipt, error) {
	identity, ok := designimplementation.ParseTaskIdentity(task.TriggerCommentContent)
	if !ok {
		return nil, nil
	}
	repositoryDir, err := designImplementationRepositoryDir(task, workDir, identity)
	if err != nil {
		return nil, err
	}
	receipt, err := designimplementation.CollectReceiptFromRepository(workDir, repositoryDir, now)
	if err != nil {
		return nil, err
	}
	if receipt.Identity.ProjectID != task.ProjectID || receipt.Identity.IssueID != task.IssueID ||
		receipt.Identity.ProjectResourceID != identity.ProjectResourceID || receipt.Identity.DesignRef != identity.DesignRef ||
		receipt.Identity.RevisionID != identity.RevisionID || receipt.Identity.ContentDigest != identity.ContentDigest ||
		len(receipt.Identity.FrameRefs) != 1 || receipt.Identity.FrameRefs[0] != identity.FrameRef {
		return nil, errors.New("design implementation receipt does not match the dispatched task identity")
	}
	return receipt, nil
}

func designImplementationRepositoryDir(task Task, workDir string, identity designimplementation.TaskIdentity) (string, error) {
	if _, err := os.Stat(filepath.Join(workDir, ".git")); err == nil {
		return workDir, nil
	}

	var selectedURL string
	peerURLs := make([]string, 0, len(task.Repos))
	for _, repository := range task.Repos {
		if url := strings.TrimSpace(repository.URL); url != "" {
			peerURLs = append(peerURLs, url)
		}
	}
	for _, resource := range task.ProjectResources {
		if resource.ID != identity.ProjectResourceID || resource.ResourceType != "github_repo" {
			continue
		}
		var reference struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal(resource.ResourceRef, &reference); err != nil {
			return "", fmt.Errorf("selected implementation repository is invalid: %w", err)
		}
		selectedURL = strings.TrimSpace(reference.URL)
		break
	}
	if selectedURL == "" {
		return "", errors.New("selected implementation repository is unavailable")
	}
	if len(peerURLs) == 0 {
		peerURLs = []string{selectedURL}
	}
	return filepath.Join(workDir, repocache.CheckoutDirName(selectedURL, peerURLs)), nil
}
