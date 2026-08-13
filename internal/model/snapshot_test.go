package model

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSnapshotFixtures(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "..", "testdata", "snapshots", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 6 {
		t.Fatalf("got %d snapshot fixtures, want 6", len(files))
	}
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			raw, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			var snapshot Snapshot
			if err := json.Unmarshal(raw, &snapshot); err != nil {
				t.Fatal(err)
			}
			if snapshot.Meta.SchemaVersion != "0.1" || snapshot.Meta.OS != "linux" {
				t.Fatalf("invalid fixture metadata: %#v", snapshot.Meta)
			}
			states := []FactState{snapshot.Host.State, snapshot.CPU.State, snapshot.Memory.State, snapshot.NUMA.State, snapshot.PCI.State, snapshot.GPUs.State, snapshot.Network.State, snapshot.RDMA.State, snapshot.Storage.State, snapshot.NVIDIA.State, snapshot.Containers.State, snapshot.Processes.State, snapshot.Runtimes.State}
			for _, state := range states {
				if state == "" {
					t.Fatal("fixture section has empty state")
				}
			}
		})
	}
}

func TestNewFactDoesNotClaimAvailableOnMarshalFailure(t *testing.T) {
	fact := NewFact("bad", StateAvailable, make(chan int), ConfidenceHigh)
	if fact.State != StateParseError || len(fact.Value) != 0 {
		t.Fatalf("unexpected fact: %#v", fact)
	}
}
