package collector_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/arpingblue/AIStat/internal/clock"
	"github.com/arpingblue/AIStat/internal/collector"
	"github.com/arpingblue/AIStat/internal/collector/cpu"
	"github.com/arpingblue/AIStat/internal/collector/memory"
	"github.com/arpingblue/AIStat/internal/collector/network"
	"github.com/arpingblue/AIStat/internal/collector/numa"
	"github.com/arpingblue/AIStat/internal/collector/pci"
	"github.com/arpingblue/AIStat/internal/collector/storage"
	systemcollector "github.com/arpingblue/AIStat/internal/collector/system"
	"github.com/arpingblue/AIStat/internal/fsx"
	"github.com/arpingblue/AIStat/internal/normalize"
)

func TestLinuxFixturePipelineRunsOnWindows(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "fixtures", "linux", "ubuntu2404-single-numa")
	registry, err := collector.NewRegistry(systemcollector.Collector{}, cpu.Collector{}, memory.Collector{}, numa.Collector{}, pci.Collector{}, network.Collector{}, storage.Collector{})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1, 0).UTC()
	results, statuses, err := registry.Run(context.Background(), collector.Env{FileSystem: fsx.Rooted{Root: root}, Clock: clock.Fixed{Time: now}, Platform: "windows", Fixture: true})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := normalize.Results(normalize.Empty(now, "test", "general"), results, statuses)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Host.Distro != "Ubuntu 24.04.1 LTS" || snapshot.CPU.LogicalCores != 4 || len(snapshot.NUMA.Nodes) != 1 || len(snapshot.PCI.Devices) != 1 || snapshot.PCI.Devices[0].Address != "0000:3b:00.0" || len(snapshot.Network.NICs) != 1 || len(snapshot.Storage.Mounts) != 2 {
		t.Fatalf("unexpected normalized fixture snapshot: %#v", snapshot)
	}
}
