package eval

import "testing"

func TestAPIDAppendRemove(t *testing.T) {
	env := NewEnv(nil)
	r := &Runner{Env: env}
	r.addAPID(123)
	r.addAPID(456)
	vals := env.Get("apid")
	if len(vals) != 2 || vals[0] != "123" || vals[1] != "456" {
		t.Fatalf("unexpected apid: %v", vals)
	}
	r.removeAPID(123)
	vals = env.Get("apid")
	if len(vals) != 1 || vals[0] != "456" {
		t.Fatalf("unexpected apid after remove: %v", vals)
	}
	r.removeAPID(456)
	vals = env.Get("apid")
	if len(vals) != 0 {
		t.Fatalf("expected apid unset, got %v", vals)
	}
}
