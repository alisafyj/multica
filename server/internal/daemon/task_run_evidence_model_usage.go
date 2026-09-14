package daemon

import "github.com/multica-ai/multica/server/pkg/agent"

type TaskRunEvidenceModelUsage struct {
	Model        string                      `json:"model"`
	Usage        TaskRunEvidenceUsage        `json:"usage"`
	ProviderCost TaskRunEvidenceProviderCost `json:"provider_cost"`
}

type TaskRunEvidenceModelUsageInventory struct {
	Entries   []TaskRunEvidenceModelUsage `json:"entries"`
	Complete  bool                        `json:"complete"`
	Truncated bool                        `json:"truncated"`
	Source    string                      `json:"source"`
}

func modelUsageFromAgent(value agent.ModelUsageInventoryEvidence) *TaskRunEvidenceModelUsageInventory {
	result := &TaskRunEvidenceModelUsageInventory{
		Entries:  make([]TaskRunEvidenceModelUsage, 0, len(value.Entries)),
		Complete: value.Complete, Truncated: value.Truncated, Source: value.Source,
	}
	if result.Source == "" {
		result.Source = agent.EvidenceSourceMissing
	}
	for _, entry := range value.Entries {
		result.Entries = append(result.Entries, TaskRunEvidenceModelUsage{
			Model: entry.Model,
			Usage: TaskRunEvidenceUsage{
				InputUncachedTokens:   cloneInt64Pointer(entry.Usage.InputUncachedTokens),
				InputCacheReadTokens:  cloneInt64Pointer(entry.Usage.InputCacheReadTokens),
				InputCacheWriteTokens: cloneInt64Pointer(entry.Usage.InputCacheWriteTokens),
				OutputTokens:          cloneInt64Pointer(entry.Usage.OutputTokens),
				Complete:              entry.Usage.Complete, Source: entry.Usage.Source,
			},
			ProviderCost: TaskRunEvidenceProviderCost{
				AmountUSDTicks: cloneInt64Pointer(entry.ProviderCost.AmountUSDTicks),
				Complete:       entry.ProviderCost.Complete, Authority: entry.ProviderCost.Authority,
				Basis: entry.ProviderCost.Basis, Source: entry.ProviderCost.Source,
			},
		})
	}
	return result
}

func cloneTaskRunEvidenceModelUsage(value *TaskRunEvidenceModelUsageInventory) *TaskRunEvidenceModelUsageInventory {
	if value == nil {
		return nil
	}
	clone := *value
	clone.Entries = make([]TaskRunEvidenceModelUsage, len(value.Entries))
	for i, entry := range value.Entries {
		clone.Entries[i] = entry
		clone.Entries[i].Usage.InputUncachedTokens = cloneInt64Pointer(entry.Usage.InputUncachedTokens)
		clone.Entries[i].Usage.InputCacheReadTokens = cloneInt64Pointer(entry.Usage.InputCacheReadTokens)
		clone.Entries[i].Usage.InputCacheWriteTokens = cloneInt64Pointer(entry.Usage.InputCacheWriteTokens)
		clone.Entries[i].Usage.OutputTokens = cloneInt64Pointer(entry.Usage.OutputTokens)
		clone.Entries[i].ProviderCost.AmountUSDTicks = cloneInt64Pointer(entry.ProviderCost.AmountUSDTicks)
	}
	return &clone
}
