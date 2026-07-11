package generate

import (
	"context"
	"testing"

	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
)

func TestNewGenerator_SupportedKinds(t *testing.T) {
	tests := []struct {
		kind artifactentity.Kind
		ok   bool
	}{
		{artifactentity.KindMindmap, true},
		{artifactentity.KindReport, false},
		{artifactentity.KindInfoGraphic, false},
		{artifactentity.KindAudioOverview, false},
	}

	for _, tt := range tests {
		_, err := newGenerator(tt.kind, &ServiceDeps{})
		if tt.ok && err != nil {
			t.Errorf("expected success for kind %s, got err=%v", tt.kind, err)
		}
		if !tt.ok && err == nil {
			t.Errorf("expected error for kind %s, got nil", tt.kind)
		}
	}
}

func TestRun_UnsupportedKind(t *testing.T) {
	req := &Request{
		Kind: artifactentity.KindReport,
	}

	_, err := Run(context.Background(), nil, req)
	if err == nil {
		t.Fatal("expected error for unsupported kind")
	}
}
