package constant

const (
	ChannelTypeUnknown        = 0
	ChannelTypeOpenAI         = 1
	ChannelTypeAzure          = 3
	ChannelTypeCustom         = 8
	ChannelTypeAnthropic      = 14
	ChannelTypeBaidu          = 15
	ChannelTypeZhipu          = 16
	ChannelTypeAli            = 17
	ChannelTypeXunfei         = 18
	ChannelType360            = 19
	ChannelTypeTencent        = 23
	ChannelTypeGemini         = 24
	ChannelTypeMoonshot       = 25
	ChannelTypeZhipu_v4       = 26
	ChannelTypeLingYiWanWu    = 31
	ChannelTypeMiniMax        = 35
	ChannelTypeSiliconFlow    = 40
	ChannelTypeDeepSeek       = 43
	ChannelTypeMokaAI         = 44
	ChannelTypeVolcEngine     = 45
	ChannelTypeBaiduV2        = 46
	ChannelTypeCodex          = 57
	ChannelTypeAdvancedCustom = 58
	ChannelTypeSub2API        = 59
	ChannelTypeNewAPI         = 60
	ChannelTypeDummy          = 61 // this one is only for count, do not add any channel after this

)

var ChannelBaseURLs = []string{
	"",                                    // 0
	"https://api.openai.com",              // 1
	"",                                    // 2
	"",                                    // 3
	"",                                    // 4
	"",                                    // 5
	"",                                    // 6
	"",                                    // 7
	"",                                    // 8
	"",                                    // 9
	"",                                    // 10
	"",                                    // 11
	"",                                    // 12
	"",                                    // 13
	"https://api.anthropic.com",           // 14
	"https://aip.baidubce.com",            // 15
	"https://open.bigmodel.cn",            // 16
	"https://dashscope.aliyuncs.com",      // 17
	"",                                    // 18
	"https://api.360.cn",                  // 19
	"",                                    // 20
	"",                                    // 21
	"",                                    // 22
	"https://hunyuan.tencentcloudapi.com", // 23
	"https://generativelanguage.googleapis.com", // 24
	"https://api.moonshot.cn",                   // 25
	"https://open.bigmodel.cn",                  // 26
	"",                                          // 27
	"",                                          // 28
	"",                                          // 29
	"",                                          // 30
	"https://api.lingyiwanwu.com",               // 31
	"",                                          // 32
	"",                                          // 33
	"",                                          // 34
	"https://api.minimax.chat",                  // 35
	"",                                          // 36
	"",                                          // 37
	"",                                          // 38
	"",                                          // 39
	"https://api.siliconflow.cn",                // 40
	"",                                          // 41
	"",                                          // 42
	"https://api.deepseek.com",                  // 43
	"https://api.moka.ai",                       // 44
	"https://ark.cn-beijing.volces.com",         // 45
	"https://qianfan.baidubce.com",              // 46
	"",                                          // 47
	"",                                          // 48
	"",                                          // 49
	"",                                          // 50
	"",                                          // 51
	"",                                          // 52
	"",                                          // 53
	"",                                          // 54
	"",                                          // 55
	"",                                          // 56
	"https://chatgpt.com",                       // 57
	"",                                          // 58
	"",                                          // 59
	"",                                          // 60
}

var ChannelTypeNames = map[int]string{
	ChannelTypeUnknown:        "Unknown",
	ChannelTypeOpenAI:         "OpenAI",
	ChannelTypeAzure:          "Azure",
	ChannelTypeCustom:         "Custom",
	ChannelTypeAnthropic:      "Anthropic",
	ChannelTypeBaidu:          "Baidu",
	ChannelTypeZhipu:          "Zhipu",
	ChannelTypeAli:            "Ali",
	ChannelTypeXunfei:         "Xunfei",
	ChannelType360:            "360",
	ChannelTypeTencent:        "Tencent",
	ChannelTypeGemini:         "Gemini",
	ChannelTypeMoonshot:       "Moonshot",
	ChannelTypeZhipu_v4:       "ZhipuV4",
	ChannelTypeLingYiWanWu:    "LingYiWanWu",
	ChannelTypeMiniMax:        "MiniMax",
	ChannelTypeSiliconFlow:    "SiliconFlow",
	ChannelTypeDeepSeek:       "DeepSeek",
	ChannelTypeMokaAI:         "MokaAI",
	ChannelTypeVolcEngine:     "VolcEngine",
	ChannelTypeBaiduV2:        "BaiduV2",
	ChannelTypeCodex:          "ChatGPT Subscription (Codex)",
	ChannelTypeAdvancedCustom: "Advanced Custom",
	ChannelTypeSub2API:        "Sub2API",
	ChannelTypeNewAPI:         "New API",
}

func GetChannelTypeName(channelType int) string {
	if name, ok := ChannelTypeNames[channelType]; ok {
		return name
	}
	return "Unknown"
}

type ChannelSpecialBase struct {
	ClaudeBaseURL string
	OpenAIBaseURL string
}

var ChannelSpecialBases = map[string]ChannelSpecialBase{
	"glm-coding-plan": {
		ClaudeBaseURL: "https://open.bigmodel.cn/api/anthropic",
		OpenAIBaseURL: "https://open.bigmodel.cn/api/coding/paas/v4",
	},
	"glm-coding-plan-international": {
		ClaudeBaseURL: "https://api.z.ai/api/anthropic",
		OpenAIBaseURL: "https://api.z.ai/api/coding/paas/v4",
	},
	"kimi-coding-plan": {
		ClaudeBaseURL: "https://api.kimi.com/coding",
		OpenAIBaseURL: "https://api.kimi.com/coding/v1",
	},
	"doubao-coding-plan": {
		ClaudeBaseURL: "https://ark.cn-beijing.volces.com/api/coding",
		OpenAIBaseURL: "https://ark.cn-beijing.volces.com/api/coding/v3",
	},
}
