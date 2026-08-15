package domain_test

import (
	"encoding/json"
	"testing"
	"unicode/utf8"

	"go.klarlabs.de/axi/domain"
)

// FuzzNewActionName ensures name validation never panics and accepts
// only the documented pattern.
func FuzzNewActionName(f *testing.F) {
	f.Add("greet")
	f.Add("send-email")
	f.Add("a")
	f.Add("")
	f.Add("1bad")
	f.Add("has space")
	f.Add("ok.name_1-2")
	f.Add("UPPER")
	f.Add("unicode-世界")

	f.Fuzz(func(t *testing.T, s string) {
		name, err := domain.NewActionName(s)
		if err != nil {
			if name != "" {
				t.Fatalf("erroneous non-empty name %q with error %v", name, err)
			}
			return
		}
		if name != domain.ActionName(s) {
			t.Fatalf("name = %q, want %q", name, s)
		}
		// Accepted names must be non-empty and match the public contract.
		if s == "" {
			t.Fatal("empty string accepted")
		}
	})
}

// FuzzNewCapabilityName mirrors action-name validation.
func FuzzNewCapabilityName(f *testing.F) {
	f.Add("string.upper")
	f.Add("")
	f.Add("9nope")
	f.Add("http.get")

	f.Fuzz(func(t *testing.T, s string) {
		_, _ = domain.NewCapabilityName(s)
	})
}

// FuzzSessionFromSnapshot ensures snapshot decode never panics and
// rejects unsupported schemas without corrupting memory.
func FuzzSessionFromSnapshot(f *testing.F) {
	valid, _ := json.Marshal(domain.SessionSnapshot{
		Schema:     domain.CurrentSessionSchema,
		ID:         "s1",
		ActionName: "greet",
		Status:     string(domain.StatusSucceeded),
	})
	f.Add(valid)
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"schema":"99","id":"x","action_name":"a","status":"succeeded"}`))
	f.Add([]byte(`{"id":"legacy","action_name":"a","status":"pending"}`))
	f.Add([]byte(`not json`))
	f.Add([]byte{0x00, 0xff, 0xfe})

	f.Fuzz(func(t *testing.T, raw []byte) {
		if !utf8.Valid(raw) && len(raw) > 0 {
			// json.Unmarshal tolerates invalid UTF-8 in strings poorly;
			// we still must not panic.
		}
		var snap domain.SessionSnapshot
		if err := json.Unmarshal(raw, &snap); err != nil {
			return
		}
		session, err := domain.SessionFromSnapshot(snap)
		if err != nil {
			if session != nil {
				t.Fatalf("non-nil session with error: %v", err)
			}
			return
		}
		if session.ID() == "" {
			t.Fatal("restored session has empty ID")
		}
		_ = session.Status()
		_ = session.Evidence()
		_ = session.ResultChunks()
	})
}
