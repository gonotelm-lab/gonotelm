package generate

import (
	"context"
	"testing"

	generatetypes "github.com/gonotelm-lab/gonotelm/internal/application/artifact/generate/types"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
)

func TestNewGenerator_SupportedKinds(t *testing.T) {
	tests := []struct {
		kind artifactentity.Kind
		ok   bool
	}{
		{artifactentity.KindMindmap, true},
		{artifactentity.KindReport, true},
		{artifactentity.KindInfoGraphic, true},
		{artifactentity.KindAudioOverview, true},
	}

	for _, tt := range tests {
		_, err := newGenerator(tt.kind, &generatetypes.ServiceDeps{})
		if tt.ok && err != nil {
			t.Errorf("expected success for kind %s, got err=%v", tt.kind, err)
		}
		if !tt.ok && err == nil {
			t.Errorf("expected error for kind %s, got nil", tt.kind)
		}
	}
}

func TestRun_UnknownKind(t *testing.T) {
	req := &generatetypes.Request{
		Kind: artifactentity.Kind("unknown_kind"),
	}

	_, err := Run(context.Background(), nil, req)
	if err == nil {
		t.Fatal("expected error for unsupported kind")
	}
}

func TestNewGenerator_ReportKind(t *testing.T) {
	g, err := newGenerator(artifactentity.KindReport, &generatetypes.ServiceDeps{})
	if err != nil {
		t.Fatalf("expected success for KindReport, got err=%v", err)
	}
	if g == nil {
		t.Fatal("expected non-nil generator")
	}
}

func TestNewGenerator_InfoGraphicKind(t *testing.T) {
	g, err := newGenerator(artifactentity.KindInfoGraphic, &generatetypes.ServiceDeps{})
	if err != nil {
		t.Fatalf("expected success for KindInfoGraphic, got err=%v", err)
	}
	if g == nil {
		t.Fatal("expected non-nil generator")
	}
}
