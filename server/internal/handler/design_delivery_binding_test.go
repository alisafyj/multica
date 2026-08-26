package handler

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/daemon"
	"github.com/multica-ai/multica/server/internal/daemon/execenv"
	"github.com/multica-ai/multica/server/internal/service"
)

// The delivery envelope crosses a process boundary: the server writes it into
// the claim response, the daemon decodes it to fetch and extract the package.
// DC-059 is the cautionary tale — both sides independently described the same
// design-document context, disagreed, and every task was refused at claim with
// nobody noticing until a document sat "generating" for an afternoon. These
// tests fail at build time if the two descriptions drift again.
func TestDesignDeliverySchemaMatchesTheDaemonConstant(t *testing.T) {
	if service.DesignDeliverySchema != daemon.DesignDeliverySchemaForTest {
		t.Fatalf("server schema %q != daemon schema %q", service.DesignDeliverySchema, daemon.DesignDeliverySchemaForTest)
	}
}

// The daemon reads revision_id and content_digest off the envelope and refuses
// anything else, so those two field names are the contract.
func TestDesignDeliveryEnvelopeCarriesWhatTheDaemonReads(t *testing.T) {
	raw, err := json.Marshal(service.DesignDeliveryContext{
		SchemaVersion:    service.DesignDeliverySchema,
		DesignDocumentID: "11111111-1111-4111-8111-111111111111",
		RevisionID:       "22222222-2222-4222-8222-222222222222",
		RevisionNumber:   3,
		ContentDigest:    "sha256:" + strings.Repeat("a", 64),
		Title:            "订单总览",
		Platform:         "web",
		Pages:            []service.DesignDeliveryPage{{ID: "orders", Title: "订单列表", Entry: "prototype/orders.html"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		SchemaVersion string `json:"schema_version"`
		RevisionID    string `json:"revision_id"`
		ContentDigest string `json:"content_digest"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.SchemaVersion != service.DesignDeliverySchema || decoded.RevisionID == "" || decoded.ContentDigest == "" {
		t.Fatalf("envelope is missing what the daemon reads: %s", raw)
	}
}

// The agent has to be told two things the delivery alone cannot imply: the
// prototype is a specification rather than source to paste, and whether the
// design ever looked at this repository (DC-053).
func TestDeliveredDesignPromptStatesItsGroundingAndItsLimits(t *testing.T) {
	envelope := func(grounded bool) string {
		raw, err := json.Marshal(service.DesignDeliveryContext{
			SchemaVersion:      service.DesignDeliverySchema,
			RevisionNumber:     2,
			Title:              "订单总览",
			Platform:           "web",
			RepositoryGrounded: grounded,
			PrototypeEntry:     "prototype/index.html",
			Pages:              []service.DesignDeliveryPage{{Title: "订单列表", Entry: "prototype/orders.html"}},
		})
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}

	grounded := execenv.RenderDesignDeliverySectionForTest(envelope(true))
	if !strings.Contains(grounded, ".agent_context/design_delivery/package/") {
		t.Fatalf("prompt does not say where the package is:\n%s", grounded)
	}
	if !strings.Contains(grounded, "prototype/orders.html") || !strings.Contains(grounded, "订单列表") {
		t.Fatalf("prompt does not list the pages to implement:\n%s", grounded)
	}
	if !strings.Contains(grounded, "specification") || !strings.Contains(grounded, "Never paste") {
		t.Fatalf("prompt does not stop the agent from pasting the prototype into the product:\n%s", grounded)
	}
	if !strings.Contains(grounded, "read-only access to the repository") {
		t.Fatalf("grounded delivery does not say so:\n%s", grounded)
	}

	ungrounded := execenv.RenderDesignDeliverySectionForTest(envelope(false))
	if !strings.Contains(ungrounded, "WITHOUT reading the repository") {
		t.Fatalf("ungrounded delivery must not let the agent assume it fits the codebase:\n%s", ungrounded)
	}

	// No delivery, no section: an ordinary issue task's brief is unchanged.
	if section := execenv.RenderDesignDeliverySectionForTest("  "); section != "" {
		t.Fatalf("a task with no delivery got a design section:\n%s", section)
	}
	if section := execenv.RenderDesignDeliverySectionForTest("{not json"); section != "" {
		t.Fatalf("an undecodable envelope produced a section:\n%s", section)
	}
}
