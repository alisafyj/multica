package agent

import (
	"math"
	"sort"
	"unicode"
	"unicode/utf8"
)

const (
	EvidenceSourceProviderEvent   = "provider_event"
	EvidenceSourceProviderSummary = "provider_summary"
	EvidenceSourceModelUsage      = "model_usage"
	EvidenceSourceMissing         = "missing"

	CostAuthorityProviderReported = "provider_reported"
	CostAuthorityMissing          = "missing"
	CostBasisProviderReported     = "provider_reported"
	CostBasisUnknown              = "unknown"
	CostBasisMissing              = "missing"

	maxModelUsageEvidenceEntries = 12
)

// ExecutionEvidence preserves provider observations separately from legacy
// TokenUsage, whose integer zero cannot distinguish missing from reported zero.
type ExecutionEvidence struct {
	Usage         UsageEvidence
	ProviderModel ProviderModelEvidence
	ProviderCost  ProviderCostEvidence
	ModelUsage    ModelUsageInventoryEvidence
}

type usageInvalidFields uint8

const (
	usageInvalidInputUncached usageInvalidFields = 1 << iota
	usageInvalidInputCacheRead
	usageInvalidInputCacheWrite
	usageInvalidOutput
)

type UsageEvidence struct {
	InputUncachedTokens   *int64
	InputCacheReadTokens  *int64
	InputCacheWriteTokens *int64
	OutputTokens          *int64
	Complete              bool
	Source                string
	invalid               usageInvalidFields
}

type ProviderModelEvidence struct {
	Model  *string
	Source string
}

// ProviderCostEvidence is a provider observation, not invoice evidence.
type ProviderCostEvidence struct {
	AmountUSDTicks *int64
	Complete       bool
	Authority      string
	Basis          string
	Source         string
	invalid        bool
}

type ModelUsageInventoryEvidence struct {
	Entries   []ModelUsageEvidence
	Complete  bool
	Truncated bool
	Source    string
}

type ModelUsageEvidence struct {
	Model        string
	Usage        UsageEvidence
	ProviderCost ProviderCostEvidence
}

func MergeExecutionEvidence(first, second *ExecutionEvidence) *ExecutionEvidence {
	if first == nil && second == nil {
		return nil
	}
	if first == nil {
		result := cloneExecutionEvidence(second)
		result.Usage.Complete = false
		result.ProviderModel = ProviderModelEvidence{Source: EvidenceSourceMissing}
		result.ProviderCost.Complete = false
		result.ModelUsage.Complete = false
		normalizeExecutionEvidenceSources(result)
		return result
	}
	if second == nil {
		result := cloneExecutionEvidence(first)
		result.Usage.Complete = false
		result.ProviderModel = ProviderModelEvidence{Source: EvidenceSourceMissing}
		result.ProviderCost.Complete = false
		result.ModelUsage.Complete = false
		normalizeExecutionEvidenceSources(result)
		return result
	}

	result := &ExecutionEvidence{
		Usage:         mergeUsageEvidence(first.Usage, second.Usage),
		ProviderModel: mergeProviderModel(first.ProviderModel, second.ProviderModel),
		ProviderCost:  mergeProviderCost(first.ProviderCost, second.ProviderCost),
		ModelUsage:    mergeModelUsageInventory(first.ModelUsage, second.ModelUsage),
	}
	return result
}

// appendObservedExecutionEvidence accumulates events that were actually
// observed. MergeExecutionEvidence deliberately treats a nil side as a missing
// execution attempt, so the first observed event must be seeded directly.
func appendObservedExecutionEvidence(current, observed *ExecutionEvidence) *ExecutionEvidence {
	if observed == nil {
		return current
	}
	return appendExecutionEvidenceAttempt(current, observed, current != nil)
}

func appendExecutionEvidenceAttempt(current, attempt *ExecutionEvidence, hasPriorAttempt bool) *ExecutionEvidence {
	if !hasPriorAttempt {
		return cloneExecutionEvidence(attempt)
	}
	return MergeExecutionEvidence(current, attempt)
}

func cloneExecutionEvidence(value *ExecutionEvidence) *ExecutionEvidence {
	if value == nil {
		return nil
	}
	clone := *value
	if value.ProviderModel.Model != nil && validModelEvidenceName(*value.ProviderModel.Model) {
		clone.ProviderModel.Model = cloneEvidenceString(value.ProviderModel.Model)
	} else {
		clone.ProviderModel = ProviderModelEvidence{Source: EvidenceSourceMissing}
	}
	clone.ProviderCost = normalizeProviderCostEvidence(value.ProviderCost)
	clone.ModelUsage = cloneModelUsageInventory(value.ModelUsage)
	clone.Usage = cloneUsageEvidence(value.Usage)
	return &clone
}

func cloneUsageEvidence(value UsageEvidence) UsageEvidence {
	value.InputUncachedTokens, value.invalid = cloneValidEvidenceToken(value.InputUncachedTokens, value.invalid, usageInvalidInputUncached)
	value.InputCacheReadTokens, value.invalid = cloneValidEvidenceToken(value.InputCacheReadTokens, value.invalid, usageInvalidInputCacheRead)
	value.InputCacheWriteTokens, value.invalid = cloneValidEvidenceToken(value.InputCacheWriteTokens, value.invalid, usageInvalidInputCacheWrite)
	value.OutputTokens, value.invalid = cloneValidEvidenceToken(value.OutputTokens, value.invalid, usageInvalidOutput)
	if value.invalid != 0 {
		value.Complete = false
	}
	if value.InputUncachedTokens == nil && value.InputCacheReadTokens == nil && value.InputCacheWriteTokens == nil && value.OutputTokens == nil {
		value.Source = EvidenceSourceMissing
	}
	return value
}

func cloneValidEvidenceToken(value *int64, invalid usageInvalidFields, field usageInvalidFields) (*int64, usageInvalidFields) {
	if invalid&field != 0 || value != nil && *value < 0 {
		return nil, invalid | field
	}
	return cloneEvidenceInt64(value), invalid
}

func mergeUsageEvidence(left, right UsageEvidence) UsageEvidence {
	left = cloneUsageEvidence(left)
	right = cloneUsageEvidence(right)
	result := UsageEvidence{Source: mergedEvidenceSource(left.Source, right.Source)}
	result.InputUncachedTokens, result.invalid = mergeEvidenceToken(left.InputUncachedTokens, right.InputUncachedTokens, left.invalid, right.invalid, result.invalid, usageInvalidInputUncached)
	result.InputCacheReadTokens, result.invalid = mergeEvidenceToken(left.InputCacheReadTokens, right.InputCacheReadTokens, left.invalid, right.invalid, result.invalid, usageInvalidInputCacheRead)
	result.InputCacheWriteTokens, result.invalid = mergeEvidenceToken(left.InputCacheWriteTokens, right.InputCacheWriteTokens, left.invalid, right.invalid, result.invalid, usageInvalidInputCacheWrite)
	result.OutputTokens, result.invalid = mergeEvidenceToken(left.OutputTokens, right.OutputTokens, left.invalid, right.invalid, result.invalid, usageInvalidOutput)
	result.Complete = left.Complete && right.Complete && result.invalid == 0
	if result.InputUncachedTokens == nil && result.InputCacheReadTokens == nil && result.InputCacheWriteTokens == nil && result.OutputTokens == nil {
		result.Source = EvidenceSourceMissing
	}
	return result
}

func mergeEvidenceToken(left, right *int64, leftInvalid, rightInvalid, invalid usageInvalidFields, field usageInvalidFields) (*int64, usageInvalidFields) {
	if leftInvalid&field != 0 || rightInvalid&field != 0 {
		return nil, invalid | field
	}
	value := sumEvidenceInt64(left, right)
	if evidenceSumOverflowed(left, right, value) {
		return nil, invalid | field
	}
	return value, invalid
}

func sumEvidenceInt64(left, right *int64) *int64 {
	if left == nil && right == nil {
		return nil
	}
	var sum int64
	if left != nil {
		if *left < 0 {
			return nil
		}
		sum = *left
	}
	if right != nil {
		if *right < 0 || sum > math.MaxInt64-*right {
			return nil
		}
		sum += *right
	}
	return &sum
}

func evidenceSumOverflowed(left, right, result *int64) bool {
	return left != nil && right != nil && result == nil
}

func cloneEvidenceInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func cloneEvidenceString(value *string) *string {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func mergedEvidenceSource(left, right string) string {
	if left == "" {
		left = EvidenceSourceMissing
	}
	if right == "" {
		right = EvidenceSourceMissing
	}
	if left == right {
		return left
	}
	if left == EvidenceSourceMissing {
		return right
	}
	if right == EvidenceSourceMissing {
		return left
	}
	return EvidenceSourceProviderSummary
}

func mergeProviderModel(left, right ProviderModelEvidence) ProviderModelEvidence {
	if left.Model != nil && right.Model != nil && validModelEvidenceName(*left.Model) && validModelEvidenceName(*right.Model) && *left.Model == *right.Model {
		return ProviderModelEvidence{Model: cloneEvidenceString(left.Model), Source: mergedEvidenceSource(left.Source, right.Source)}
	}
	return ProviderModelEvidence{Source: EvidenceSourceMissing}
}

func mergeProviderCost(left, right ProviderCostEvidence) ProviderCostEvidence {
	left = normalizeProviderCostEvidence(left)
	right = normalizeProviderCostEvidence(right)
	if left.invalid || right.invalid {
		return ProviderCostEvidence{
			Complete: false, Authority: CostAuthorityMissing, Basis: CostBasisMissing,
			Source: EvidenceSourceMissing, invalid: true,
		}
	}
	amount := sumEvidenceInt64(left.AmountUSDTicks, right.AmountUSDTicks)
	if evidenceSumOverflowed(left.AmountUSDTicks, right.AmountUSDTicks, amount) {
		return ProviderCostEvidence{
			Complete: false, Authority: CostAuthorityMissing, Basis: CostBasisMissing,
			Source: EvidenceSourceMissing, invalid: true,
		}
	}
	if amount == nil {
		return ProviderCostEvidence{Authority: CostAuthorityMissing, Basis: CostBasisMissing, Source: EvidenceSourceMissing}
	}
	basis := left.Basis
	if basis == "" || right.Basis == "" || basis != right.Basis {
		basis = CostBasisUnknown
	}
	return ProviderCostEvidence{
		AmountUSDTicks: amount,
		Complete:       left.Complete && right.Complete && basis != CostBasisUnknown,
		Authority:      CostAuthorityProviderReported,
		Basis:          basis,
		Source:         mergedEvidenceSource(left.Source, right.Source),
	}
}

func normalizeProviderCostEvidence(value ProviderCostEvidence) ProviderCostEvidence {
	value.AmountUSDTicks = cloneEvidenceInt64(value.AmountUSDTicks)
	if value.AmountUSDTicks == nil || *value.AmountUSDTicks < 0 {
		return ProviderCostEvidence{
			Complete: false, Authority: CostAuthorityMissing, Basis: CostBasisMissing,
			Source: EvidenceSourceMissing, invalid: value.invalid || value.AmountUSDTicks != nil,
		}
	}
	return value
}

func mergeModelUsageInventory(left, right ModelUsageInventoryEvidence) ModelUsageInventoryEvidence {
	left = cloneModelUsageInventory(left)
	right = cloneModelUsageInventory(right)
	byModel := make(map[string]ModelUsageEvidence, len(left.Entries)+len(right.Entries))
	for _, entry := range left.Entries {
		byModel[entry.Model] = entry
	}
	for _, entry := range right.Entries {
		if existing, ok := byModel[entry.Model]; ok {
			entry.Usage = mergeUsageEvidence(existing.Usage, entry.Usage)
			entry.ProviderCost = mergeProviderCost(existing.ProviderCost, entry.ProviderCost)
		}
		byModel[entry.Model] = entry
	}
	result := modelUsageInventoryFromMap(byModel)
	result.Truncated = result.Truncated || left.Truncated || right.Truncated
	result.Complete = left.Complete && right.Complete && !result.Truncated && modelUsageEntriesComplete(result.Entries)
	if left.Source == EvidenceSourceModelUsage || right.Source == EvidenceSourceModelUsage {
		result.Source = EvidenceSourceModelUsage
	} else {
		result.Source = EvidenceSourceMissing
	}
	return result
}

func cloneModelUsageInventory(value ModelUsageInventoryEvidence) ModelUsageInventoryEvidence {
	if value.Source != EvidenceSourceModelUsage {
		return ModelUsageInventoryEvidence{Source: EvidenceSourceMissing}
	}
	byModel := make(map[string]ModelUsageEvidence, len(value.Entries))
	valid := true
	for _, entry := range value.Entries {
		if !validModelEvidenceName(entry.Model) {
			valid = false
			continue
		}
		entry.Usage = cloneUsageEvidence(entry.Usage)
		entry.ProviderCost = normalizeProviderCostEvidence(entry.ProviderCost)
		if existing, ok := byModel[entry.Model]; ok {
			entry.Usage = mergeUsageEvidence(existing.Usage, entry.Usage)
			entry.ProviderCost = mergeProviderCost(existing.ProviderCost, entry.ProviderCost)
		}
		byModel[entry.Model] = entry
	}
	result := modelUsageInventoryFromMap(byModel)
	result.Truncated = result.Truncated || value.Truncated
	result.Complete = value.Complete && valid && !result.Truncated && modelUsageEntriesComplete(result.Entries)
	result.Source = EvidenceSourceModelUsage
	return result
}

func modelUsageInventoryFromMap(byModel map[string]ModelUsageEvidence) ModelUsageInventoryEvidence {
	models := make([]string, 0, len(byModel))
	for model := range byModel {
		models = append(models, model)
	}
	sort.Strings(models)
	result := ModelUsageInventoryEvidence{Source: EvidenceSourceModelUsage}
	if len(models) > maxModelUsageEvidenceEntries {
		models = models[:maxModelUsageEvidenceEntries]
		result.Truncated = true
	}
	result.Entries = make([]ModelUsageEvidence, 0, len(models))
	for _, model := range models {
		result.Entries = append(result.Entries, byModel[model])
	}
	return result
}

func modelUsageEntriesComplete(entries []ModelUsageEvidence) bool {
	for _, entry := range entries {
		if !entry.Usage.Complete {
			return false
		}
	}
	return true
}

func validModelEvidenceName(model string) bool {
	if len(model) < 1 || len(model) > 255 || !utf8.ValidString(model) {
		return false
	}
	for _, r := range model {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func normalizeExecutionEvidenceSources(value *ExecutionEvidence) {
	if value.Usage.Source == "" {
		value.Usage.Source = EvidenceSourceMissing
	}
	if value.ProviderModel.Source == "" {
		value.ProviderModel.Source = EvidenceSourceMissing
	}
	if value.ProviderCost.Source == "" {
		value.ProviderCost.Source = EvidenceSourceMissing
	}
	if value.ModelUsage.Source == "" {
		value.ModelUsage.Source = EvidenceSourceMissing
	}
	if value.ProviderCost.Authority == "" {
		value.ProviderCost.Authority = CostAuthorityMissing
	}
	if value.ProviderCost.Basis == "" {
		value.ProviderCost.Basis = CostBasisMissing
	}
}
