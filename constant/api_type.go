package constant

const (
	APITypeOpenAI = iota
	APITypeAnthropic
	APITypeBaidu
	APITypeZhipu
	APITypeAli
	APITypeXunfei
	APITypeTencent
	APITypeGemini
	APITypeZhipuV4
	APITypeSiliconFlow
	APITypeDeepSeek
	APITypeMokaAI
	APITypeVolcEngine
	APITypeBaiduV2
	_ // APITypeJimeng (removed, numbering preserved)
	APITypeMoonshot
	APITypeMiniMax
	APITypeCodex
	APITypeAdvancedCustom
	APITypeSub2API
	APITypeNewAPI
	APITypeDummy // this one is only for count, do not add any channel after this
)
