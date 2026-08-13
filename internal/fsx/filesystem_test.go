package fsx

import (
	"path/filepath"
	"testing"
)

func TestRootedReadsLinuxPathOnWindows(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "fixtures", "linux", "ubuntu2404-single-numa")
	filesystem := Rooted{Root: root}
	raw, err := filesystem.ReadFile("/proc/meminfo")
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) == 0 {
		t.Fatal("empty fixture")
	}
	entries, err := filesystem.ReadDir("/sys/bus/pci/devices")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "0000:3b:00.0" {
		t.Fatalf("unexpected virtual BDF entries: %#v", entries)
	}
}

func TestRootedRejectsEscape(t *testing.T) {
	filesystem := Rooted{Root: t.TempDir()}
	if _, err := filesystem.ReadFile("../../outside"); err == nil {
		t.Fatal("expected path escape error")
	}
}
