package lark

import (
	"context"
	"errors"
	"sort"
	"strings"
)

// PRDTemplateReader supplies authoritative heading labels before delegation;
// it performs only reads and grants no publication capability to the worker.
type PRDTemplateReader interface {
	ReadPRDTemplateHeadings(context.Context, InstallationCredentials, string) ([]string, error)
}

func (c *httpAPIClient) ReadPRDTemplateHeadings(ctx context.Context, creds InstallationCredentials, wikiToken string) ([]string, error) {
	docID, err := c.ResolvePRDTemplate(ctx, creds, wikiToken)
	if err != nil {
		return nil, err
	}
	snapshot, err := c.prdSnapshot(ctx, creds, docID)
	if err != nil {
		return nil, err
	}
	headings := make([]string, 0)
	for _, block := range snapshot.Blocks {
		text, ok := block.heading()
		if !ok {
			continue
		}
		heading := strings.TrimSpace(text)
		if heading == "" || len(heading) > 500 {
			return nil, errors.New("PRD template has an empty or oversized heading")
		}
		if _, _, err := snapshot.locate(heading); err != nil {
			return nil, err
		}
		headings = append(headings, heading)
	}
	if len(headings) == 0 || len(headings) > 50 {
		return nil, errors.New("PRD template must have 1-50 unique writable headings")
	}
	// The snapshot is a map; deterministic ordering keeps repeated requests
	// idempotent. Publication still inserts under the native template headings.
	sort.Strings(headings)
	return headings, nil
}
