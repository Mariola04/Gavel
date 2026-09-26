package engine

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEqualJSON(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{`1`, `1`, true},
		{`1`, `1.0`, true},
		{`1`, `2`, false},
		{`"a"`, `"a"`, true},
		{`"1"`, `1`, false},
		{`true`, `false`, false},
		{`[1,2]`, `[1,2]`, true},
		{`[1,2]`, `[2,1]`, false},
		{`null`, `[]`, true},
		{`null`, `{}`, true},
		{`[[]]`, `[null]`, true},
		{`null`, `0`, false},
		{`{"a":1,"b":[1]}`, `{"b":[1],"a":1}`, true},
		{`9223372036854775808`, `9223372036854775808`, true},
		{`not json`, `1`, false},
	}
	for _, tt := range tests {
		if got := equalJSON(json.RawMessage(tt.a), json.RawMessage(tt.b)); got != tt.want {
			t.Errorf("equalJSON(%s, %s) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestParseResultsRequiresNonce(t *testing.T) {
	out := strings.Join([]string{
		`hello from the student`,
		resultPrefix + `{"nonce":"forged","index":0,"got":99}`,
		resultPrefix + `{"index":1,"got":99}`,
		resultPrefix + `{"nonce":"n1","index":0,"got":1,"duration_ms":3}`,
		`@@RESULT@@{"nonce":"n1","index":2}`,
		resultPrefix + `broken`,
	}, "\n")
	got := parseResults(out, "n1")
	if len(got) != 1 {
		t.Fatalf("got %d results, want 1: %+v", len(got), got)
	}
	if r := got[0]; string(r.Got) != "1" || r.DurationMS != 3 {
		t.Fatalf("result = %+v", r)
	}
}

func TestLimitedBuffer(t *testing.T) {
	b := &limitedBuffer{limit: 5}
	for _, s := range []string{"abc", "defg", "h"} {
		if n, err := b.Write([]byte(s)); n != len(s) || err != nil {
			t.Fatalf("Write(%q) = %d, %v", s, n, err)
		}
	}
	if b.String() != "abcde" || !b.truncated {
		t.Fatalf("buffer = %q, truncated = %v", b.String(), b.truncated)
	}
}

func TestParseSandboxMode(t *testing.T) {
	for _, s := range []string{"auto", "firejail", "none"} {
		if _, err := ParseSandboxMode(s); err != nil {
			t.Errorf("ParseSandboxMode(%q): %v", s, err)
		}
	}
	if _, err := ParseSandboxMode("docker"); err == nil {
		t.Error("expected error for unknown mode")
	}
}
