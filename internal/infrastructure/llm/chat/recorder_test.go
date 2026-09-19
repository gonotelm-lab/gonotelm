package chat

import (
	"context"
	"testing"
	"time"

	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
)

func TestBuildRecordSubagentMetadata(t *testing.T) {
	ctx := context.Background()
	endTime := time.Now()

	rec := buildRecord(ctx, endTime)
	if _, ok := rec.Metadatas[RecordMetaSubagent]; ok {
		t.Fatalf("non-subagent record should not carry metadata %q", RecordMetaSubagent)
	}

	rec = buildRecord(pkgcontext.WithSubagent(ctx), endTime)
	if rec.Metadatas[RecordMetaSubagent] != true {
		t.Fatalf("subagent record metadata %q = %v, want true",
			RecordMetaSubagent, rec.Metadatas[RecordMetaSubagent])
	}
}
