package audiotranscriber

import (
	"context"
	"io"
)

type DummyTranscriber struct{}

func NewDummyTranscriber() *DummyTranscriber {
	return &DummyTranscriber{}
}

// TranscribeAudio converts audio file to text
func (t *DummyTranscriber) TranscribeAudio(ctx context.Context, audioData io.Reader) (string, error) {
	return "", nil // No actual transcription, just a placeholder
}

// IsSupported checks if the audio format is supported
func (t *DummyTranscriber) IsSupported(ctx context.Context, format string) bool {
	return false
}
