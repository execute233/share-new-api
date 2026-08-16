package common

import "github.com/QuantumNous/new-api/constant"

func ChannelType2APIType(channelType int) (int, bool) {
	apiType := -1
	switch channelType {
	case constant.ChannelTypeOpenAI:
		apiType = constant.APITypeOpenAI
	case constant.ChannelTypeAnthropic:
		apiType = constant.APITypeAnthropic
	case constant.ChannelTypeBaidu:
		apiType = constant.APITypeBaidu
	case constant.ChannelTypeZhipu:
		apiType = constant.APITypeZhipu
	case constant.ChannelTypeAli:
		apiType = constant.APITypeAli
	case constant.ChannelTypeXunfei:
		apiType = constant.APITypeXunfei
	case constant.ChannelTypeTencent:
		apiType = constant.APITypeTencent
	case constant.ChannelTypeGemini:
		apiType = constant.APITypeGemini
	case constant.ChannelTypeZhipu_v4:
		apiType = constant.APITypeZhipuV4
	case constant.ChannelTypeSiliconFlow:
		apiType = constant.APITypeSiliconFlow
	case constant.ChannelTypeDeepSeek:
		apiType = constant.APITypeDeepSeek
	case constant.ChannelTypeMokaAI:
		apiType = constant.APITypeMokaAI
	case constant.ChannelTypeVolcEngine:
		apiType = constant.APITypeVolcEngine
	case constant.ChannelTypeBaiduV2:
		apiType = constant.APITypeBaiduV2
	case constant.ChannelTypeMoonshot:
		apiType = constant.APITypeMoonshot
	case constant.ChannelTypeMiniMax:
		apiType = constant.APITypeMiniMax
	case constant.ChannelTypeCodex:
		apiType = constant.APITypeCodex
	case constant.ChannelTypeAdvancedCustom:
		apiType = constant.APITypeAdvancedCustom
	case constant.ChannelTypeSub2API:
		apiType = constant.APITypeSub2API
	case constant.ChannelTypeNewAPI:
		apiType = constant.APITypeNewAPI
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
		constant.APITypeNewAPI:
		return true
	default:
		return false
	}
}
