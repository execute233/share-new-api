package common

import "github.com/QuantumNous/new-api/constant"

func ChannelType2APIType(channelType int) (int, bool) {
	apiType := -1
	switch channelType {
	case constant.ChannelTypeOpenAI:
		apiType = constant.APITypeOpenAI
	case constant.ChannelTypeAnthropic:
		apiType = constant.APITypeAnthropic
	case constant.ChannelTypeGemini:
		apiType = constant.APITypeGemini
	case constant.ChannelTypeCodex:
		apiType = constant.APITypeCodex
	case constant.ChannelTypeAdvancedCustom:
		apiType = constant.APITypeAdvancedCustom
	case constant.ChannelTypeSub2API:
		apiType = constant.APITypeSub2API
	case constant.ChannelTypeNewAPI:
		apiType = constant.APITypeNewAPI
	case constant.ChannelTypeCodexDisguise:
		apiType = constant.APITypeCodexDisguise
	}
	if apiType == -1 {
		return constant.APITypeOpenAI, false
	}
	return apiType, true
}

func SupportsResponsesCompact(channelType, apiType int) bool {
	switch apiType {
	case constant.APITypeOpenAI,
		constant.APITypeCodex,
		constant.APITypeAdvancedCustom,
		constant.APITypeSub2API,
		constant.APITypeNewAPI,
		constant.APITypeCodexDisguise:
		return true
	default:
		return false
	}
}
