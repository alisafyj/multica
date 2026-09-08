package agent

import "testing"

func TestCodexResponseMetadataExecutionEvidenceAddsOnlyObservedProviderModel(t *testing.T) {
	input, cacheRead, cacheWrite, output, cost := int64(11), int64(3), int64(2), int64(5), int64(700)
	existing := &ExecutionEvidence{
		Usage: UsageEvidence{
			InputUncachedTokens: &input, InputCacheReadTokens: &cacheRead,
			InputCacheWriteTokens: &cacheWrite, OutputTokens: &output,
			Complete: true, Source: EvidenceSourceProviderEvent,
		},
		ProviderModel: ProviderModelEvidence{Source: EvidenceSourceMissing},
		ProviderCost: ProviderCostEvidence{
			AmountUSDTicks: &cost, Complete: true,
			Authority: CostAuthorityProviderReported, Basis: CostBasisProviderReported,
			Source: EvidenceSourceProviderSummary,
		},
	}
	model := "provider-observed-model"
	digest := codexResponseMetadataDigest(model)
	projection := &codexResponseMetadataProjection{
		Schema: codexResponseMetadataSchema, Status: "observed", Source: codexResponseMetadataSource,
		ActualModel: &model, ActualModelDigest: &digest, ModelSource: "provider_reported",
		UsageStatus: "observed",
		Usage: &codexResponseMetadataUsage{
			UncachedInputTokens: 999, CacheReadInputTokens: 999,
			CacheWriteInputTokens: 999, OutputTokens: 999,
		},
	}

	got := codexResponseMetadataExecutionEvidence(existing, projection)
	if got == nil || got.ProviderModel.Model == nil || *got.ProviderModel.Model != model || got.ProviderModel.Source != EvidenceSourceProviderEvent {
		t.Fatalf("provider model = %+v", got)
	}
	if got.Usage.InputUncachedTokens == nil || *got.Usage.InputUncachedTokens != input ||
		got.Usage.InputCacheReadTokens == nil || *got.Usage.InputCacheReadTokens != cacheRead ||
		got.Usage.InputCacheWriteTokens == nil || *got.Usage.InputCacheWriteTokens != cacheWrite ||
		got.Usage.OutputTokens == nil || *got.Usage.OutputTokens != output ||
		!got.Usage.Complete || got.Usage.Source != EvidenceSourceProviderEvent {
		t.Fatalf("usage changed by model projection: %+v", got.Usage)
	}
	if got.ProviderCost.AmountUSDTicks == nil || *got.ProviderCost.AmountUSDTicks != cost ||
		!got.ProviderCost.Complete || got.ProviderCost.Authority != CostAuthorityProviderReported ||
		got.ProviderCost.Basis != CostBasisProviderReported || got.ProviderCost.Source != EvidenceSourceProviderSummary {
		t.Fatalf("provider cost changed by model projection: %+v", got.ProviderCost)
	}
	if existing.ProviderModel.Model != nil || existing.ProviderModel.Source != EvidenceSourceMissing {
		t.Fatalf("input evidence was mutated: %+v", existing.ProviderModel)
	}
}

func TestCodexResponseMetadataExecutionEvidenceCreatesModelOnlyEvidence(t *testing.T) {
	model := "provider-observed-model"
	digest := codexResponseMetadataDigest(model)
	got := codexResponseMetadataExecutionEvidence(nil, &codexResponseMetadataProjection{
		Schema: codexResponseMetadataSchema, Status: "observed", Source: codexResponseMetadataSource,
		ActualModel: &model, ActualModelDigest: &digest, ModelSource: "provider_reported",
	})

	if got == nil || got.ProviderModel.Model == nil || *got.ProviderModel.Model != model || got.ProviderModel.Source != EvidenceSourceProviderEvent {
		t.Fatalf("provider model = %+v", got)
	}
	if got.Usage.Source != EvidenceSourceMissing || got.ProviderCost.Source != EvidenceSourceMissing ||
		got.ProviderCost.Authority != CostAuthorityMissing || got.ProviderCost.Basis != CostBasisMissing {
		t.Fatalf("model-only evidence was not normalized: %+v", got)
	}
}

func TestCodexResponseMetadataExecutionEvidenceRejectsUnverifiedModel(t *testing.T) {
	model := "provider-observed-model"
	digest := codexResponseMetadataDigest(model)
	valid := codexResponseMetadataProjection{
		Schema: codexResponseMetadataSchema, Status: "observed", Source: codexResponseMetadataSource,
		ActualModel: &model, ActualModelDigest: &digest, ModelSource: "provider_reported",
	}

	cases := map[string]*codexResponseMetadataProjection{
		"default off":     nil,
		"missing":         func() *codexResponseMetadataProjection { p := valid; p.Status = "missing"; return &p }(),
		"conflicting":     func() *codexResponseMetadataProjection { p := valid; p.Status = "conflicting"; return &p }(),
		"failed":          func() *codexResponseMetadataProjection { p := valid; p.Status = "failed"; return &p }(),
		"wrong schema":    func() *codexResponseMetadataProjection { p := valid; p.Schema = "other"; return &p }(),
		"wrong source":    func() *codexResponseMetadataProjection { p := valid; p.Source = "requested"; return &p }(),
		"requested model": func() *codexResponseMetadataProjection { p := valid; p.ModelSource = "requested"; return &p }(),
		"missing model":   func() *codexResponseMetadataProjection { p := valid; p.ActualModel = nil; return &p }(),
		"wrong digest": func() *codexResponseMetadataProjection {
			p := valid
			d := codexResponseMetadataDigest("other")
			p.ActualModelDigest = &d
			return &p
		}(),
	}

	for name, projection := range cases {
		t.Run(name, func(t *testing.T) {
			got := codexResponseMetadataExecutionEvidence(nil, projection)
			if got != nil {
				t.Fatalf("unverified projection produced evidence: %+v", got)
			}
		})
	}
}
