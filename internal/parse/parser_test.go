package parse

import (
	"strings"
	"testing"
)

func TestParseReadsInput(t *testing.T) {
	input := "hello"
	file, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if file.Text != input {
		t.Fatalf("expected %q, got %q", input, file.Text)
	}
}
