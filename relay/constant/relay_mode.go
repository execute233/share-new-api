package constant

import (
	"strings"
)

const (
	RelayModeUnknown = iota
	RelayModeChatCompletions
	RelayModeCompletions
	RelayModeEmbeddings
	RelayModeModerations
	RelayModeImagesGenerations
	RelayModeImagesEdits
	RelayModeEdits

	_ // RelayModeMidjourneyImagine (removed, numbering preserved)
	_ // RelayModeMidjourneyDescribe
	_ // RelayModeMidjourneyBlend
	_ // RelayModeMidjourneyChange
	_ // RelayModeMidjourneySimpleChange
	_ // RelayModeMidjourneyNotify
	_ // RelayModeMidjourneyTaskFetch
	_ // RelayModeMidjourneyTaskImageSeed
	_ // RelayModeMidjourneyTaskFetchByCondition
	_ // RelayModeMidjourneyAction
	_ // RelayModeMidjourneyModal
	_ // RelayModeMidjourneyShorten
	_ // RelayModeSwapFace
	_ // RelayModeMidjourneyUpload
	_ // RelayModeMidjourneyVideo
	_ // RelayModeMidjourneyEdits

	_ // RelayModeSunoFetch (removed, numbering preserved)
	_ // RelayModeSunoFetchByID
	_ // RelayModeSunoSubmit

	_ // RelayModeVideoFetchByID
	_ // RelayModeVideoSubmit

	RelayModeResponses

	RelayModeGemini

	RelayModeResponsesCompact

	RelayModeAlphaSearch
)

func Path2RelayMode(path string) int {
	relayMode := RelayModeUnknown
	if strings.HasPrefix(path, "/v1/chat/completions") || strings.HasPrefix(path, "/pg/chat/completions") {
		relayMode = RelayModeChatCompletions
	} else if strings.HasPrefix(path, "/v1/completions") {
		relayMode = RelayModeCompletions
	} else if strings.HasPrefix(path, "/v1/embeddings") {
		relayMode = RelayModeEmbeddings
	} else if strings.HasSuffix(path, "embeddings") {
		relayMode = RelayModeEmbeddings
	} else if strings.HasPrefix(path, "/v1/moderations") {
		relayMode = RelayModeModerations
	} else if strings.HasPrefix(path, "/v1/images/generations") {
		relayMode = RelayModeImagesGenerations
	} else if strings.HasPrefix(path, "/v1/images/edits") {
		relayMode = RelayModeImagesEdits
	} else if strings.HasPrefix(path, "/v1/edits") {
		relayMode = RelayModeEdits
	} else if strings.HasPrefix(path, "/v1/responses/compact") {
		relayMode = RelayModeResponsesCompact
	} else if strings.HasPrefix(path, "/v1/responses") {
		relayMode = RelayModeResponses
	} else if strings.HasPrefix(path, "/v1/alpha/search") {
		relayMode = RelayModeAlphaSearch
	} else if strings.HasPrefix(path, "/v1beta/models") || strings.HasPrefix(path, "/v1/models") {
		relayMode = RelayModeGemini
	}
	return relayMode
}
