package agent

import (
	"math"
	"reflect"
	"testing"
)

func evidenceInt64(value int64) *int64 { return &value }

func TestMergeExecutionEvidenceKeepsMissingRetryUsageIncomplete(t *testing.T) {
	first := &ExecutionEvidence{Usage: UsageEvidence{
		InputUncachedTokens: evidenceInt64(10),
		OutputTokens:        evidenceInt64(2),
		Source:              EvidenceSourceProviderEvent,
	}}

	got := MergeExecutionEvidence(first, nil)
	if got == nil || got.Usage.InputUncachedTokens == nil || *got.Usage.InputUncachedTokens != 10 {
		t.Fatalf("merged input = %#v, want 10", got)
	}
	if got.Usage.Complete {
		t.Fatal("usage with a missing retry was marked complete")
	}
}

func TestMergeExecutionEvidenceSumsDisjointBucketsAcrossRetries(t *testing.T) {
	first := &ExecutionEvidence{Usage: UsageEvidence{
		InputUncachedTokens: evidenceInt64(10), InputCacheReadTokens: evidenceInt64(3),
		InputCacheWriteTokens: evidenceInt64(1), OutputTokens: evidenceInt64(2),
		Complete: true, Source: EvidenceSourceProviderEvent,
	}}
	second := &ExecutionEvidence{Usage: UsageEvidence{
		InputUncachedTokens: evidenceInt64(20), InputCacheReadTokens: evidenceInt64(4),
		InputCacheWriteTokens: evidenceInt64(2), OutputTokens: evidenceInt64(5),
		Complete: true, Source: EvidenceSourceProviderEvent,
	}}

	got := MergeExecutionEvidence(first, second)
	if *got.Usage.InputUncachedTokens != 30 || *got.Usage.InputCacheReadTokens != 7 ||
		*got.Usage.InputCacheWriteTokens != 3 || *got.Usage.OutputTokens != 7 {
		t.Fatalf("merged usage = %+v", got.Usage)
	}
	if !got.Usage.Complete {
		t.Fatal("two complete usage reports were marked incomplete")
	}
}

func TestMergeExecutionEvidenceDoesNotClaimModelWhenOneAttemptIsMissingIt(t *testing.T) {
	model := "glm-5.3"
	first := &ExecutionEvidence{ProviderModel: ProviderModelEvidence{Model: &model, Source: EvidenceSourceModelUsage}}
	second := &ExecutionEvidence{ProviderModel: ProviderModelEvidence{Source: EvidenceSourceMissing}}

	got := MergeExecutionEvidence(first, second)
	if got.ProviderModel.Model != nil || got.ProviderModel.Source != EvidenceSourceMissing {
		t.Fatalf("provider model = %+v, want missing", got.ProviderModel)
	}
}

func TestMergeExecutionEvidenceMarksOverflowedBucketUnknown(t *testing.T) {
	first := &ExecutionEvidence{Usage: UsageEvidence{InputUncachedTokens: evidenceInt64(math.MaxInt64), Complete: true, Source: EvidenceSourceProviderEvent}}
	second := &ExecutionEvidence{Usage: UsageEvidence{InputUncachedTokens: evidenceInt64(1), Complete: true, Source: EvidenceSourceProviderEvent}}

	got := MergeExecutionEvidence(first, second)
	if got.Usage.InputUncachedTokens != nil || got.Usage.Complete {
		t.Fatalf("overflowed usage = %+v, want unknown incomplete bucket", got.Usage)
	}
}

func TestMergeExecutionEvidenceNormalizesEmptySourcesToMissing(t *testing.T) {
	got := MergeExecutionEvidence(&ExecutionEvidence{}, nil)
	if got.Usage.Source != EvidenceSourceMissing || got.ProviderModel.Source != EvidenceSourceMissing || got.ProviderCost.Source != EvidenceSourceMissing {
		t.Fatalf("sources = usage:%q model:%q cost:%q", got.Usage.Source, got.ProviderModel.Source, got.ProviderCost.Source)
	}
	if got.ModelUsage.Source != EvidenceSourceMissing || got.ModelUsage.Complete || len(got.ModelUsage.Entries) != 0 {
		t.Fatalf("model usage = %+v, want empty missing inventory", got.ModelUsage)
	}
}

func TestMergeExecutionEvidenceMergesModelUsageByExactIdentity(t *testing.T) {
	first := &ExecutionEvidence{ModelUsage: ModelUsageInventoryEvidence{
		Entries: []ModelUsageEvidence{
			{Model: "z-model", Usage: completeUsageEvidence(3, 0, 0, 1), ProviderCost: completeCostEvidence(30)},
			{Model: "a-model", Usage: completeUsageEvidence(2, 1, 0, 4), ProviderCost: completeCostEvidence(20)},
		},
		Complete: true,
		Source:   EvidenceSourceModelUsage,
	}}
	second := &ExecutionEvidence{ModelUsage: ModelUsageInventoryEvidence{
		Entries: []ModelUsageEvidence{
			{Model: "m-model", Usage: completeUsageEvidence(5, 0, 2, 1), ProviderCost: completeCostEvidence(50)},
			{Model: "a-model", Usage: completeUsageEvidence(7, 3, 1, 2), ProviderCost: completeCostEvidence(70)},
		},
		Complete: true,
		Source:   EvidenceSourceModelUsage,
	}}

	got := MergeExecutionEvidence(first, second)
	models := []string{got.ModelUsage.Entries[0].Model, got.ModelUsage.Entries[1].Model, got.ModelUsage.Entries[2].Model}
	if !reflect.DeepEqual(models, []string{"a-model", "m-model", "z-model"}) {
		t.Fatalf("models = %v", models)
	}
	a := got.ModelUsage.Entries[0]
	if a.Usage.InputUncachedTokens == nil || *a.Usage.InputUncachedTokens != 9 || a.ProviderCost.AmountUSDTicks == nil || *a.ProviderCost.AmountUSDTicks != 90 {
		t.Fatalf("merged a-model = %+v", a)
	}
	if !got.ModelUsage.Complete || got.ModelUsage.Truncated || got.ModelUsage.Source != EvidenceSourceModelUsage {
		t.Fatalf("inventory = %+v", got.ModelUsage)
	}
}

func TestMergeExecutionEvidenceModelUsageMissingAttemptIsIncompleteAndDeepCloned(t *testing.T) {
	model := ModelUsageEvidence{Model: "a-model", Usage: completeUsageEvidence(2, 0, 0, 1)}
	first := &ExecutionEvidence{ModelUsage: ModelUsageInventoryEvidence{
		Entries:  []ModelUsageEvidence{model},
		Complete: true,
		Source:   EvidenceSourceModelUsage,
	}}

	got := MergeExecutionEvidence(first, nil)
	if got.ModelUsage.Complete || got.ModelUsage.Source != EvidenceSourceModelUsage || len(got.ModelUsage.Entries) != 1 {
		t.Fatalf("inventory = %+v", got.ModelUsage)
	}
	*got.ModelUsage.Entries[0].Usage.InputUncachedTokens = 99
	got.ModelUsage.Entries[0].Model = "changed"
	if *first.ModelUsage.Entries[0].Usage.InputUncachedTokens != 2 || first.ModelUsage.Entries[0].Model != "a-model" {
		t.Fatalf("merge aliased input inventory: %+v", first.ModelUsage)
	}
}

func TestExecutionEvidenceCloneNormalizesMissingModelUsageInventory(t *testing.T) {
	attempt := &ExecutionEvidence{ModelUsage: ModelUsageInventoryEvidence{
		Entries:  []ModelUsageEvidence{{Model: "a-model", Usage: completeUsageEvidence(1, 0, 0, 0)}},
		Complete: true,
		Source:   EvidenceSourceMissing,
	}}

	got := appendExecutionEvidenceAttempt(nil, attempt, false)
	if got.ModelUsage.Source != EvidenceSourceMissing || got.ModelUsage.Complete || len(got.ModelUsage.Entries) != 0 {
		t.Fatalf("model usage = %+v, want empty missing inventory", got.ModelUsage)
	}
}

func TestExecutionEvidenceCloneKeepsUsagePointersIndependent(t *testing.T) {
	original := &ExecutionEvidence{Usage: completeUsageEvidence(1, 0, 3, 4)}
	cloned := cloneExecutionEvidence(original)
	if !reflect.DeepEqual(cloned.Usage, original.Usage) {
		t.Fatal("clone changed usage values")
	}
	for index, pair := range [][2]*int64{
		{original.Usage.InputUncachedTokens, cloned.Usage.InputUncachedTokens},
		{original.Usage.InputCacheReadTokens, cloned.Usage.InputCacheReadTokens},
		{original.Usage.InputCacheWriteTokens, cloned.Usage.InputCacheWriteTokens},
		{original.Usage.OutputTokens, cloned.Usage.OutputTokens},
	} {
		if pair[0] == pair[1] {
			t.Fatalf("clone aliased usage field %d", index)
		}
	}
}

func TestMergeExecutionEvidenceModelUsagePropagatesTruncation(t *testing.T) {
	first := &ExecutionEvidence{ModelUsage: ModelUsageInventoryEvidence{
		Entries:   []ModelUsageEvidence{{Model: "a-model", Usage: completeUsageEvidence(1, 0, 0, 0)}},
		Complete:  false,
		Truncated: true,
		Source:    EvidenceSourceModelUsage,
	}}
	second := &ExecutionEvidence{ModelUsage: ModelUsageInventoryEvidence{
		Entries:  []ModelUsageEvidence{{Model: "b-model", Usage: completeUsageEvidence(1, 0, 0, 0)}},
		Complete: true,
		Source:   EvidenceSourceModelUsage,
	}}

	got := MergeExecutionEvidence(first, second)
	if !got.ModelUsage.Truncated || got.ModelUsage.Complete || got.ModelUsage.Source != EvidenceSourceModelUsage {
		t.Fatalf("inventory = %+v", got.ModelUsage)
	}
}

func TestMergeExecutionEvidenceOverflowedTokenDoesNotRecoverOnLaterRetry(t *testing.T) {
	first := &ExecutionEvidence{Usage: completeUsageEvidence(math.MaxInt64, 0, 0, 0)}
	second := &ExecutionEvidence{Usage: completeUsageEvidence(1, 0, 0, 0)}
	third := &ExecutionEvidence{Usage: completeUsageEvidence(1, 0, 0, 0)}

	got := MergeExecutionEvidence(MergeExecutionEvidence(first, second), third)
	if got.Usage.InputUncachedTokens != nil || got.Usage.Complete {
		t.Fatalf("overflowed usage recovered after retry: %+v", got.Usage)
	}
}

func TestExecutionEvidenceCloneRejectsInvalidProviderModelIdentity(t *testing.T) {
	model := "bad\nmodel"
	attempt := &ExecutionEvidence{ProviderModel: ProviderModelEvidence{Model: &model, Source: EvidenceSourceProviderEvent}}

	got := appendExecutionEvidenceAttempt(nil, attempt, false)
	if got.ProviderModel.Model != nil || got.ProviderModel.Source != EvidenceSourceMissing {
		t.Fatalf("provider model = %+v, want missing", got.ProviderModel)
	}
}

func TestMergeExecutionEvidenceModelUsageOverflowDoesNotRecoverOnLaterRetry(t *testing.T) {
	attempt := func(input int64) *ExecutionEvidence {
		return &ExecutionEvidence{ModelUsage: ModelUsageInventoryEvidence{
			Entries:  []ModelUsageEvidence{{Model: "a-model", Usage: completeUsageEvidence(input, 0, 0, 0)}},
			Complete: true,
			Source:   EvidenceSourceModelUsage,
		}}
	}

	got := MergeExecutionEvidence(MergeExecutionEvidence(attempt(math.MaxInt64), attempt(1)), attempt(1))
	entry := got.ModelUsage.Entries[0]
	if entry.Usage.InputUncachedTokens != nil || entry.Usage.Complete || got.ModelUsage.Complete {
		t.Fatalf("overflowed model usage recovered after retry: %+v", got.ModelUsage)
	}
}

func TestMergeExecutionEvidenceProviderCostInvalidAmountDoesNotRecoverOnLaterRetry(t *testing.T) {
	for _, firstAmount := range []int64{math.MaxInt64, -1} {
		attempt := func(amount int64) *ExecutionEvidence {
			return &ExecutionEvidence{ProviderCost: completeCostEvidence(amount)}
		}

		got := MergeExecutionEvidence(MergeExecutionEvidence(attempt(firstAmount), attempt(1)), attempt(1))
		if got.ProviderCost.AmountUSDTicks != nil || got.ProviderCost.Complete || got.ProviderCost.Authority != CostAuthorityMissing || got.ProviderCost.Basis != CostBasisMissing || got.ProviderCost.Source != EvidenceSourceMissing {
			t.Fatalf("first amount %d recovered after retry: %+v", firstAmount, got.ProviderCost)
		}
	}
}

func completeUsageEvidence(input, cacheRead, cacheWrite, output int64) UsageEvidence {
	return UsageEvidence{
		InputUncachedTokens: evidenceInt64(input), InputCacheReadTokens: evidenceInt64(cacheRead),
		InputCacheWriteTokens: evidenceInt64(cacheWrite), OutputTokens: evidenceInt64(output),
		Complete: true, Source: EvidenceSourceModelUsage,
	}
}

func completeCostEvidence(ticks int64) ProviderCostEvidence {
	return ProviderCostEvidence{
		AmountUSDTicks: evidenceInt64(ticks), Complete: true,
		Authority: CostAuthorityProviderReported, Basis: CostBasisProviderReported, Source: EvidenceSourceModelUsage,
	}
}
