package agent

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
)

func decodeClaudeEvidence(t *testing.T, raw string) *ExecutionEvidence {
	t.Helper()
	var msg claudeSDKMessage
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		t.Fatalf("decode Claude event: %v", err)
	}
	return claudeResultExecutionEvidence(msg)
}

func TestClaudeExecutionEvidencePreservesPartialUsage(t *testing.T) {
	got := decodeClaudeEvidence(t, `{"type":"result","model":"glm-5.3","usage":{"input_tokens":12,"output_tokens":0}}`)
	if got == nil || got.Usage.InputUncachedTokens == nil || *got.Usage.InputUncachedTokens != 12 {
		t.Fatalf("evidence = %#v", got)
	}
	if got.Usage.OutputTokens == nil || *got.Usage.OutputTokens != 0 {
		t.Fatalf("explicit output zero was not preserved: %#v", got.Usage.OutputTokens)
	}
	if got.Usage.InputCacheReadTokens != nil || got.Usage.Complete {
		t.Fatalf("partial usage was completed: %+v", got.Usage)
	}
}

func TestClaudeExecutionEvidencePreservesExplicitAllZeroUsage(t *testing.T) {
	got := decodeClaudeEvidence(t, `{"type":"result","model":"glm-5.3","usage":{"input_tokens":0,"output_tokens":0,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}`)
	if got == nil || !got.Usage.Complete {
		t.Fatalf("all-zero usage should be complete: %#v", got)
	}
	if got.Usage.InputUncachedTokens == nil || *got.Usage.InputUncachedTokens != 0 {
		t.Fatalf("explicit input zero missing: %#v", got.Usage)
	}
}

func TestClaudeExecutionEvidenceUsesModelUsageInsteadOfZeroAggregate(t *testing.T) {
	got := decodeClaudeEvidence(t, `{"type":"result","usage":{"input_tokens":0,"output_tokens":0,"cache_read_input_tokens":0,"cache_creation_input_tokens":0},"modelUsage":{"glm-5.3":{"inputTokens":98057,"outputTokens":5,"cacheReadInputTokens":0,"cacheCreationInputTokens":0,"costUSD":0.49041,"costBasis":"unknown"}}}`)
	if got.ProviderModel.Model == nil || *got.ProviderModel.Model != "glm-5.3" || got.ProviderModel.Source != EvidenceSourceModelUsage {
		t.Fatalf("provider model = %+v", got.ProviderModel)
	}
	if got.Usage.InputUncachedTokens == nil || *got.Usage.InputUncachedTokens != 98057 {
		t.Fatalf("usage = %+v", got.Usage)
	}
	if got.ProviderCost.AmountUSDTicks == nil || *got.ProviderCost.AmountUSDTicks != 4_904_100_000 {
		t.Fatalf("cost = %+v", got.ProviderCost)
	}
	if got.ProviderCost.Complete || got.ProviderCost.Basis != CostBasisUnknown {
		t.Fatalf("unknown-basis cost was treated as complete: %+v", got.ProviderCost)
	}
	if len(got.ModelUsage.Entries) != 1 || got.ModelUsage.Entries[0].Model != "glm-5.3" || !got.ModelUsage.Complete || got.ModelUsage.Source != EvidenceSourceModelUsage {
		t.Fatalf("model usage inventory = %+v", got.ModelUsage)
	}
	if got.ModelUsage.Entries[0].ProviderCost.Complete || got.ModelUsage.Entries[0].ProviderCost.Basis != CostBasisUnknown {
		t.Fatalf("entry cost = %+v", got.ModelUsage.Entries[0].ProviderCost)
	}
}

func TestClaudeModelUsageInventoryIsSortedAndCostMissingDoesNotMakeItIncomplete(t *testing.T) {
	got := decodeClaudeEvidence(t, `{"type":"result","modelUsage":{"z-model":{"inputTokens":3,"outputTokens":1,"cacheReadInputTokens":0,"cacheCreationInputTokens":0},"a-model":{"inputTokens":2,"outputTokens":4,"cacheReadInputTokens":1,"cacheCreationInputTokens":0,"costUSD":0.25,"costBasis":"provider_reported"}}}`)
	if len(got.ModelUsage.Entries) != 2 || got.ModelUsage.Entries[0].Model != "a-model" || got.ModelUsage.Entries[1].Model != "z-model" {
		t.Fatalf("entries = %+v", got.ModelUsage.Entries)
	}
	if !got.ModelUsage.Complete || got.ModelUsage.Truncated || got.ModelUsage.Source != EvidenceSourceModelUsage {
		t.Fatalf("inventory = %+v", got.ModelUsage)
	}
	if got.ModelUsage.Entries[1].ProviderCost.AmountUSDTicks != nil || got.ModelUsage.Entries[1].ProviderCost.Complete {
		t.Fatalf("missing entry cost = %+v", got.ModelUsage.Entries[1].ProviderCost)
	}
	if got.ProviderModel.Model != nil || got.ProviderModel.Source != EvidenceSourceMissing {
		t.Fatalf("multiple model keys claimed a main model: %+v", got.ProviderModel)
	}
}

func TestClaudeModelUsageOmitsInvalidIdentityButAggregatesItsRawBucket(t *testing.T) {
	invalid := "bad\nmodel"
	raw := fmt.Sprintf(`{"type":"result","modelUsage":{"good-model":{"inputTokens":2,"outputTokens":1,"cacheReadInputTokens":0,"cacheCreationInputTokens":0},%q:{"inputTokens":5,"outputTokens":3,"cacheReadInputTokens":0,"cacheCreationInputTokens":0}}}`, invalid)
	got := decodeClaudeEvidence(t, raw)

	if got.Usage.InputUncachedTokens == nil || *got.Usage.InputUncachedTokens != 7 || got.Usage.OutputTokens == nil || *got.Usage.OutputTokens != 4 {
		t.Fatalf("aggregate omitted invalid-name bucket: %+v", got.Usage)
	}
	if len(got.ModelUsage.Entries) != 1 || got.ModelUsage.Entries[0].Model != "good-model" || got.ModelUsage.Complete {
		t.Fatalf("inventory = %+v", got.ModelUsage)
	}
	if got.ProviderModel.Model != nil || got.ProviderModel.Source != EvidenceSourceMissing {
		t.Fatalf("invalid key changed multi-key provider model semantics: %+v", got.ProviderModel)
	}
}

func TestClaudeModelUsageRejectsInvalidSoleMainModelIdentity(t *testing.T) {
	for _, model := range []string{"", "bad\x00model", strings.Repeat("m", 256)} {
		raw, err := json.Marshal(map[string]any{
			"type": "result",
			"modelUsage": map[string]any{model: map[string]any{
				"inputTokens": 1, "outputTokens": 1, "cacheReadInputTokens": 0, "cacheCreationInputTokens": 0,
			}},
		})
		if err != nil {
			t.Fatalf("encode Claude event: %v", err)
		}
		got := decodeClaudeEvidence(t, string(raw))
		if got.ProviderModel.Model != nil || got.ProviderModel.Source != EvidenceSourceMissing {
			t.Fatalf("model %q was reported as main identity: %+v", model, got.ProviderModel)
		}
		if len(got.ModelUsage.Entries) != 0 || got.ModelUsage.Complete {
			t.Fatalf("model %q inventory = %+v", model, got.ModelUsage)
		}
	}
}

func TestClaudeProviderSummaryRejectsInvalidMainModelIdentity(t *testing.T) {
	got := decodeClaudeEvidence(t, `{"type":"result","model":"bad\nmodel","usage":{"input_tokens":1,"output_tokens":1,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}`)
	if got.ProviderModel.Model != nil || got.ProviderModel.Source != EvidenceSourceMissing {
		t.Fatalf("provider model = %+v", got.ProviderModel)
	}
	if got.ModelUsage.Source != EvidenceSourceMissing || got.ModelUsage.Complete || len(got.ModelUsage.Entries) != 0 {
		t.Fatalf("summary inferred model usage inventory: %+v", got.ModelUsage)
	}
}

func TestClaudeModelUsageTruncatesSortedEntriesButAggregatesAllBuckets(t *testing.T) {
	var raw strings.Builder
	raw.WriteString(`{"type":"result","modelUsage":{`)
	for i := 12; i >= 0; i-- {
		if i != 12 {
			raw.WriteByte(',')
		}
		fmt.Fprintf(&raw, `"model-%02d":{"inputTokens":1,"outputTokens":0,"cacheReadInputTokens":0,"cacheCreationInputTokens":0}`, i)
	}
	raw.WriteString(`}}`)
	got := decodeClaudeEvidence(t, raw.String())

	if len(got.ModelUsage.Entries) != 12 || got.ModelUsage.Entries[0].Model != "model-00" || got.ModelUsage.Entries[11].Model != "model-11" {
		t.Fatalf("entries = %+v", got.ModelUsage.Entries)
	}
	if !got.ModelUsage.Truncated || got.ModelUsage.Complete {
		t.Fatalf("inventory = %+v", got.ModelUsage)
	}
	if got.Usage.InputUncachedTokens == nil || *got.Usage.InputUncachedTokens != 13 {
		t.Fatalf("aggregate usage = %+v", got.Usage)
	}
}

func TestClaudeModelUsageNegativeAndOverflowTokensStayUnknown(t *testing.T) {
	raw := fmt.Sprintf(`{"type":"result","modelUsage":{"a":{"inputTokens":-1,"outputTokens":0,"cacheReadInputTokens":0,"cacheCreationInputTokens":0},"b":{"inputTokens":%d,"outputTokens":0,"cacheReadInputTokens":0,"cacheCreationInputTokens":0},"c":{"inputTokens":2,"outputTokens":0,"cacheReadInputTokens":0,"cacheCreationInputTokens":0}}}`, int64(math.MaxInt64))
	got := decodeClaudeEvidence(t, raw)

	if got.Usage.InputUncachedTokens != nil || got.Usage.Complete {
		t.Fatalf("aggregate input recovered after invalid values: %+v", got.Usage)
	}
	if got.ModelUsage.Complete || got.ModelUsage.Entries[0].Usage.InputUncachedTokens != nil || got.ModelUsage.Entries[0].Usage.Complete {
		t.Fatalf("negative entry usage = %+v", got.ModelUsage)
	}
}

func TestClaudeModelUsageCostRangeCheckedBeforeInt64Conversion(t *testing.T) {
	got := decodeClaudeEvidence(t, `{"type":"result","modelUsage":{"a":{"inputTokens":1,"outputTokens":0,"cacheReadInputTokens":0,"cacheCreationInputTokens":0,"costUSD":1e30,"costBasis":"provider_reported"}}}`)
	if got.ProviderCost.AmountUSDTicks != nil || got.ProviderCost.Complete || got.ProviderCost.Authority != CostAuthorityMissing || got.ProviderCost.Basis != CostBasisMissing || got.ProviderCost.Source != EvidenceSourceMissing {
		t.Fatalf("aggregate cost = %+v", got.ProviderCost)
	}
	entry := got.ModelUsage.Entries[0]
	if entry.ProviderCost.AmountUSDTicks != nil || entry.ProviderCost.Complete || entry.ProviderCost.Authority != CostAuthorityMissing || entry.ProviderCost.Basis != CostBasisMissing || entry.ProviderCost.Source != EvidenceSourceMissing {
		t.Fatalf("entry cost = %+v", entry.ProviderCost)
	}
	if !got.ModelUsage.Complete {
		t.Fatalf("invalid cost alone made inventory incomplete: %+v", got.ModelUsage)
	}
}

func TestClaudeModelUsageAllUnknownCountersUseMissingUsageSource(t *testing.T) {
	got := decodeClaudeEvidence(t, `{"type":"result","modelUsage":{"a":{"inputTokens":-1}}}`)
	entry := got.ModelUsage.Entries[0]
	if entry.Usage.Source != EvidenceSourceMissing || entry.Usage.Complete {
		t.Fatalf("entry usage = %+v", entry.Usage)
	}
	if got.Usage.Source != EvidenceSourceMissing || got.Usage.Complete {
		t.Fatalf("aggregate usage = %+v", got.Usage)
	}
	if got.ModelUsage.Source != EvidenceSourceModelUsage || got.ModelUsage.Complete {
		t.Fatalf("inventory = %+v", got.ModelUsage)
	}
}
