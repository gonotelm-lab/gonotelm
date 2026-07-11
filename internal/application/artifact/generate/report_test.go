package generate

import (
	"testing"

	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
)

func TestReportGenerator_ImplementsGenerator(t *testing.T) {
	var _ Generator = &ReportGenerator{}
}

func TestNewGenerator_ReportKind(t *testing.T) {
	g, err := newGenerator(artifactentity.KindReport, &ServiceDeps{})
	if err != nil {
		t.Fatalf("expected success for KindReport, got err=%v", err)
	}
	if _, ok := g.(*ReportGenerator); !ok {
		t.Fatalf("expected *ReportGenerator, got %T", g)
	}
}
