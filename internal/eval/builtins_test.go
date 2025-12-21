package eval

import (
	"bytes"
	"strings"
	"testing"
)

func TestBuiltinAPID(t *testing.T) {
	env := NewEnv(nil)
	r := &Runner{Env: env}
	r.addAPID(123)
	r.addAPID(456)

	var out bytes.Buffer
	status := builtinAPID(nil, &out, &out, nil, r)
	if status != 0 {
		t.Fatalf("unexpected status: %d", status)
	}
	got := strings.TrimSpace(out.String())
	if got != "123 456" {
		t.Fatalf("unexpected output: %q", got)
	}
}
