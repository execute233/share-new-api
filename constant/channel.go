package constant

const (
	ChannelTypeUnknown        = 0
	ChannelTypeOpenAI         = 1
	ChannelTypeCustom         = 8
	ChannelTypeAnthropic      = 14
	ChannelTypeGemini         = 24
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
	"",                                    // 15
	"",                                    // 16
	"",                                    // 17
	"",                                    // 18
	"",                                    // 19
	"",                                    // 20
	"",                                    // 21
	"",                                    // 22
	"",                                    // 23
	"https://generativelanguage.googleapis.com", // 24
	"",                                          // 25
	"",                                          // 26
	"",                                          // 27
	"",                                          // 28
	"",                                          // 29
	"",                                          // 30
	"",                                          // 31
	"",                                          // 32
	"",                                          // 33
	"",                                          // 34
	"",                                          // 35
	"",                                          // 36
	"",                                          // 37
	"",                                          // 38
	"",                                          // 39
	"",                                          // 40
	"",                                          // 41
	"",                                          // 42
	"",                                          // 43
	"",                                          // 44
	"",                                          // 45
	"",                                          // 46
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
	ChannelTypeCustom:         "Custom",
	ChannelTypeAnthropic:      "Anthropic",
	ChannelTypeGemini:         "Gemini",
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
