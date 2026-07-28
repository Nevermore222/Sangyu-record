package workflow

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestDeterministicWriteBookPreservesEvidenceReference(t *testing.T) {
	processor := DeterministicProcessor{Node: NodeWriteBook}
	output, err := processor.Process(context.Background(), NodePayload{ProjectID: uuid.New(), Node: NodeWriteBook})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(output), "audio-fixture#12-20") {
		t.Fatalf("output = %s", output)
	}
}
