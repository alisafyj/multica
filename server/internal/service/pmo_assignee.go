package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const pmoCorporateEmailDomain = "@soyoung.com"

type pmoAgentCandidate struct {
	ID           string
	OwnerID      string
	RuntimeBound bool
}

// normalizePMOOwnerEmail converts a PM snapshot owner external_id into the
// workspace email key used for exact matching. Bare corporate accounts get the
// canonical @soyoung.com domain; anything that is not a single, safe account or
// email (display names, spaces, repeated prefixes) resolves to empty so it stays
// in the manual mapping queue instead of being guessed.
func normalizePMOOwnerEmail(externalID string) string {
	value := strings.ToLower(strings.TrimSpace(externalID))
	if value == "" {
		return ""
	}
	if strings.Contains(value, "@") {
		if strings.Count(value, "@") != 1 || strings.HasPrefix(value, "@") || strings.HasSuffix(value, "@") {
			return ""
		}
		return value
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' {
			continue
		}
		return ""
	}
	return value + pmoCorporateEmailDomain
}

// pmoSnapshotOwners returns every distinct owner named by the snapshot, keyed by
// the original external_id so mapping keys stay stable for diff/link persistence.
func pmoSnapshotOwners(snapshot PMOSnapshot) map[string]*PMOExternalOwner {
	owners := map[string]*PMOExternalOwner{}
	addOwner := func(o *PMOExternalOwner) {
		if o != nil && o.ExternalID != "" {
			owners[o.ExternalID] = o
		}
	}
	addOwner(snapshot.Parent.Owner)
	for _, child := range snapshot.Children {
		addOwner(child.Owner)
		for i := range child.Tasks {
			addOwner(child.Tasks[i].Owner)
		}
	}
	for i := range snapshot.Tasks {
		addOwner(snapshot.Tasks[i].Owner)
	}
	return owners
}

// matchPMOAgentMappings merges explicit Agent mappings with automatic exact
// email matches. A member is only an intermediate lookup: automatic mapping
// selects their first runtime-bound Agent in ListAgents creation order.
// ponytail: keep this deterministic default; use explicit mappings when role-specific routing matters.
func matchPMOAgentMappings(owners map[string]*PMOExternalOwner, memberEmailToUserID map[string]string, agents []pmoAgentCandidate, existing map[string]string) map[string]string {
	result := make(map[string]string, len(existing)+len(owners))
	eligibleAgentIDs := make(map[string]struct{}, len(agents))
	owned := map[string][]string{}
	for _, agent := range agents {
		if agent.RuntimeBound && agent.OwnerID != "" && agent.ID != "" {
			eligibleAgentIDs[agent.ID] = struct{}{}
			owned[agent.OwnerID] = append(owned[agent.OwnerID], agent.ID)
		}
	}
	for externalID, agentID := range existing {
		if _, eligible := eligibleAgentIDs[agentID]; externalID != "" && eligible {
			result[externalID] = agentID
		}
	}
	for externalID := range owners {
		if _, explicitlyMapped := existing[externalID]; explicitlyMapped {
			continue
		}
		email := normalizePMOOwnerEmail(externalID)
		if email == "" {
			continue
		}
		userID := memberEmailToUserID[strings.ToLower(email)]
		if candidates := owned[userID]; len(candidates) > 0 {
			result[externalID] = candidates[0]
		}
	}
	return result
}

// ResolvePMOAssigneeMappings combines explicit Agent mappings with exact
// workspace-member email matches followed by eligible Agent selection.
func ResolvePMOAssigneeMappings(
	ctx context.Context,
	qtx *db.Queries,
	workspaceID pgtype.UUID,
	snapshot PMOSnapshot,
	existing map[string]string,
) (map[string]string, error) {
	members, err := qtx.ListMembersWithUser(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("resolve pmo assignees: list workspace members: %w", err)
	}
	agents, err := listPMOAgentCandidates(ctx, qtx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("resolve pmo assignees: %w", err)
	}
	memberEmailToUserID := make(map[string]string, len(members))
	for _, member := range members {
		memberEmailToUserID[strings.ToLower(strings.TrimSpace(member.UserEmail))] = util.UUIDToString(member.UserID)
	}
	return matchPMOAgentMappings(pmoSnapshotOwners(snapshot), memberEmailToUserID, agents, existing), nil
}

func resolvePMOAssigneeMappingsFromLinks(
	ctx context.Context,
	qtx *db.Queries,
	workspaceID pgtype.UUID,
	snapshot PMOSnapshot,
	links []db.PmoSyncLink,
) (map[string]string, error) {
	explicit := map[string]string{}
	legacy := map[string]string{}
	for _, link := range links {
		if link.ExternalType != pmoExternalTypeAssignee || !link.LocalID.Valid {
			continue
		}
		switch link.LocalType.String {
		case pmoLocalTypeAgent:
			explicit[link.ExternalKey] = util.UUIDToString(link.LocalID)
		case pmoLocalTypeMember:
			legacy[link.ExternalKey] = util.UUIDToString(link.LocalID)
		}
	}

	mappings, err := ResolvePMOAssigneeMappings(ctx, qtx, workspaceID, snapshot, explicit)
	if err != nil {
		return nil, err
	}
	agents, err := listPMOAgentCandidates(ctx, qtx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("resolve pmo assignees: %w", err)
	}
	legacyAgents := resolvePMOLegacyMemberMappings(legacy, agents)
	for externalID, agentID := range legacyAgents {
		mappings[externalID] = agentID
	}
	for externalID := range legacy {
		if legacyAgents[externalID] == "" {
			delete(mappings, externalID)
		}
	}
	return mappings, nil
}

func listPMOAgentCandidates(ctx context.Context, qtx *db.Queries, workspaceID pgtype.UUID) ([]pmoAgentCandidate, error) {
	agentRows, err := qtx.ListAgents(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list workspace agents: %w", err)
	}
	agents := make([]pmoAgentCandidate, 0, len(agentRows))
	for _, agent := range agentRows {
		agents = append(agents, pmoAgentCandidate{
			ID:           util.UUIDToString(agent.ID),
			OwnerID:      util.UUIDToString(agent.OwnerID),
			RuntimeBound: agent.RuntimeID.Valid,
		})
	}
	return agents, nil
}

func resolvePMOLegacyMemberMappings(legacy map[string]string, agents []pmoAgentCandidate) map[string]string {
	owned := map[string][]string{}
	for _, agent := range agents {
		if agent.RuntimeBound && agent.OwnerID != "" && agent.ID != "" {
			owned[agent.OwnerID] = append(owned[agent.OwnerID], agent.ID)
		}
	}
	result := make(map[string]string, len(legacy))
	for externalID, memberID := range legacy {
		if candidates := owned[memberID]; len(candidates) > 0 {
			result[externalID] = candidates[0]
		}
	}
	return result
}
