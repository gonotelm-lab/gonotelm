package videooverview

import (
	"github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

// videoNarratorSpeakerKey 视频旁白固定使用单一解说音色。
const videoNarratorSpeakerKey = "apollo"

func (s *audioSynthizer) resolveNarratorVoice(lang entity.Language) (string, error) {
	sp, ok := entity.BuiltinSpeakers[videoNarratorSpeakerKey]
	if !ok {
		return "", errors.ErrInner.Msgf("video narrator speaker %q not found", videoNarratorSpeakerKey)
	}

	langMap, ok := sp.Voices[s.provider.String()]
	if !ok {
		return "", errors.ErrInner.Msgf(
			"video narrator speaker %q has no voice mapping for provider %q",
			videoNarratorSpeakerKey, s.provider,
		)
	}

	voice := langMap[string(lang)]
	if voice == "" {
		return "", errors.ErrInner.Msgf(
			"video narrator speaker %q has no voice for language %q in provider %q",
			videoNarratorSpeakerKey, lang, s.provider,
		)
	}

	return voice, nil
}
