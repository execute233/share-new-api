package service

import (
	"math"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
)

// dashboardTokens uses usage facts, never the expression-dependent billing
// variables (which may fold cache or modality counts into other categories).
func dashboardTokens(usage *dto.Usage, anthropic, estimated bool) *model.DashboardTokens {
	if usage == nil {
		return nil
	}
	if canonical, ok := usageFromBillingUsage(usage); ok {
		estimated = estimated || usage.BillingUsage.Estimated
		usage = canonical
		anthropic = usage.UsageSemantic == dto.BillingUsageSemanticAnthropic
	}
	if usage.ClaudeCacheCreation5mTokens < 0 || usage.ClaudeCacheCreation1hTokens < 0 {
		return nil
	}
	input, output := int64(usage.PromptTokens), int64(usage.CompletionTokens)
	read := int64(usage.PromptTokensDetails.CachedTokens)
	write := int64(usage.PromptTokensDetails.CacheCreationTokensTotal())
	if usage.ClaudeCacheCreation5mTokens > 0 || usage.ClaudeCacheCreation1hTokens > 0 {
		write = int64(usage.ClaudeCacheCreation5mTokens) + int64(usage.ClaudeCacheCreation1hTokens)
	}
	if input < 0 || output < 0 || read < 0 || write < 0 || input > math.MaxInt32 || output > math.MaxInt32 || read > math.MaxInt32 || write > math.MaxInt32 {
		return nil
	}
	if !anthropic {
		// Overlapping upstream cache prefixes cannot be represented as disjoint
		// categories reliably. Keep the request, but exclude its unknown tokens.
		if read > input || write > input-read {
			return nil
		}
		input -= read + write
	}
	return &model.DashboardTokens{Input: input, Output: output, CacheRead: read, CacheCreation: write, Incomplete: estimated}
}
