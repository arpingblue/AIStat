package app

import (
	"bytes"
	"encoding/json"
	"runtime"
	"strings"
	"testing"
)

func TestVersionJSON(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run([]string{"version", "--format", "json"}, &out, &errOut); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, errOut.String())
	}
	var parsed map[string]string
	if err := json.Unmarshal(out.Bytes(), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed["version"] == "" {
		t.Fatal("missing version")
	}
}
func TestCheckJSONPortable(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("portable unsupported-state behavior is Windows-specific")
	}
	var out, errOut bytes.Buffer
	code := Run([]string{"check", "--format", "json", "--profile", "general"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, errOut.String())
	}
	var parsed map[string]any
	if err := json.Unmarshal(out.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if parsed["schema_version"] != "0.1" {
		t.Fatalf("bad schema: %v", parsed["schema_version"])
	}
}
func TestUnknownCommand(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run([]string{"wat"}, &out, &errOut); code != 2 || !strings.Contains(errOut.String(), "unknown command") {
		t.Fatalf("code/output: %d %s", code, errOut.String())
	}
}
