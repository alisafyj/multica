package service

// The contract that carries a delivered design into the task that implements
// it (DC-062).
//
// A design document is delivered by linking it to an issue. When an agent
// claims a task for that issue, the server resolves the link into this
// envelope, the daemon uses it to fetch and extract the package, and the agent
// finds the real prototype on disk. Nothing here is taken from the request
// side: the revision named is always the document's own saved pointer, so a
// task cannot ask for a draft or for another document's package.

// DesignDeliverySchema versions the envelope the daemon decodes.
const DesignDeliverySchema = "multica.design-delivery/v1"

// DesignDeliveryPage is one page of the delivered design, so the agent can see
// what it is expected to build before opening a single file.
type DesignDeliveryPage struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Entry string `json:"entry"`
}

// DesignDeliveryContext travels on an issue task's claim response.
type DesignDeliveryContext struct {
	SchemaVersion string `json:"schema_version"`
	// The document and the exact revision delivered. Always the saved one — a
	// draft is not a promise (P-011 / DC-034).
	DesignDocumentID string `json:"design_document_id"`
	RevisionID       string `json:"revision_id"`
	RevisionNumber   int32  `json:"revision_number"`
	ContentDigest    string `json:"content_digest"`
	Title            string `json:"title"`
	Platform         string `json:"platform"`
	// Whether the design run itself read the repository. An agent must not
	// assume the design was written against this codebase when it was not
	// (DC-053).
	RepositoryGrounded bool                 `json:"repository_grounded"`
	PrototypeEntry     string               `json:"prototype_entry,omitempty"`
	Pages              []DesignDeliveryPage `json:"pages"`
}
