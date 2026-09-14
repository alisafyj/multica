package agent

import "testing"

func TestCodexUsageEvidenceKeepsMissingCacheWriteUnknown(t *testing.T) {
	got := codexUsageExecutionEvidence(map[string]any{
		"input_tokens": 100.0, "cached_input_tokens": 20.0, "output_tokens": 7.0,
	})
	if got.Usage.InputUncachedTokens == nil || *got.Usage.InputUncachedTokens != 80 {
		t.Fatalf("uncached input = %#v", got.Usage.InputUncachedTokens)
	}
	if got.Usage.InputCacheWriteTokens != nil || got.Usage.Complete {
		t.Fatalf("partial usage = %+v", got.Usage)
	}
}

func TestCodexUsageEvidencePreservesExplicitAllZeroBuckets(t *testing.T) {
	got := codexUsageExecutionEvidence(map[string]any{
		"input_tokens": 0.0, "cached_input_tokens": 0.0,
		"cache_write_input_tokens": 0.0, "output_tokens": 0.0,
		"reasoning_output_tokens": 99.0,
	})
	if !got.Usage.Complete || got.Usage.OutputTokens == nil || *got.Usage.OutputTokens != 0 {
		t.Fatalf("all-zero usage = %+v", got.Usage)
	}
	if got.ProviderModel.Model != nil || got.ProviderModel.Source != EvidenceSourceMissing {
		t.Fatalf("Codex client-effective model leaked into provider identity: %+v", got.ProviderModel)
	}
}

func TestCodexUsageEvidenceDoesNotAddReasoningSubsetToOutput(t *testing.T) {
	got := codexUsageExecutionEvidence(map[string]any{
		"input_tokens": 10.0, "cached_input_tokens": 0.0,
		"cache_write_input_tokens": 0.0, "output_tokens": 7.0,
		"reasoning_output_tokens": 5.0,
	})
	if got.Usage.OutputTokens == nil || *got.Usage.OutputTokens != 7 {
		t.Fatalf("output tokens = %#v, want provider total 7", got.Usage.OutputTokens)
	}
}
