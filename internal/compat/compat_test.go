package compat

import (
	"testing"
	"time"
)

func TestCUDA(t *testing.T) {
	tests := []struct {
		driver, cuda string
		want         Decision
	}{{"580.65.06", "13.0", Compatible}, {"525.60.12", "12.8", Incompatible}, {"bad", "12.8", Unknown}}
	for _, test := range tests {
		if got, _ := CUDA(test.driver, test.cuda); got != test.want {
			t.Errorf("CUDA(%q,%q)=%s want %s", test.driver, test.cuda, got, test.want)
		}
	}
}
func TestCUDACompatibilityPackage(t *testing.T) {
	got, reason := CUDAWithCompat("525.60.12", "12.8", true)
	if got != CompatibleWithPackage || reason == "" {
		t.Fatalf("unexpected decision: %s %q", got, reason)
	}
}
func TestEmbeddedDatasetMetadata(t *testing.T) {
	if DatasetVersion != "2026-08-13" || len(Sources) != 2 || len(minimumDriver) == 0 || len(lifecycles) == 0 {
		t.Fatalf("embedded datasets not loaded: version=%q sources=%v", DatasetVersion, Sources)
	}
}
func TestDriverEOL(t *testing.T) {
	eol, _, known := DriverEOL("535.183.01", time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))
	if !known || !eol {
		t.Fatalf("known=%v eol=%v", known, eol)
	}
}
