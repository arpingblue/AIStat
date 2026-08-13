package cpu

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/arpingblue/AIStat/internal/collector"
	"github.com/arpingblue/AIStat/internal/fsx"
	"github.com/arpingblue/AIStat/internal/model"
)

func TestParseFixture(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "fixtures", "linux", "ubuntu2404-single-numa", "proc", "cpuinfo"))
	if err != nil {
		t.Fatal(err)
	}
	cpu, err := Parse(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	if cpu.LogicalCores != 4 || cpu.PhysicalCores != 2 || cpu.Sockets != 1 || cpu.ThreadsPerCore != 2 || len(cpu.Logical) != 4 {
		t.Fatalf("unexpected CPU: %#v", cpu)
	}
}

func TestParseCPUList(t *testing.T) {
	got, err := ParseCPUList("0-2,4,2")
	if err != nil || len(got) != 4 || got[3] != 4 {
		t.Fatalf("got=%v err=%v", got, err)
	}
	if _, err := ParseCPUList("4-1"); err == nil {
		t.Fatal("reverse range must fail")
	}
	if _, err := ParseCPUList("0-2147483647"); err == nil {
		t.Fatal("unbounded range must fail")
	}
}

func FuzzParseCPUList(f *testing.F) {
	for _, seed := range []string{"0", "0-3", "0-2,4,6-7", "", "4-1", "0-2147483647", "x"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		values, err := ParseCPUList(raw)
		if err != nil {
			return
		}
		seen := map[int]bool{}
		for _, value := range values {
			if value < 0 || value > maxCPUIndex {
				t.Fatalf("CPU index outside parser bounds: %d", value)
			}
			if seen[value] {
				t.Fatalf("duplicate CPU index: %d", value)
			}
			seen[value] = true
		}
	})
}

func TestCollectCacheFixture(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "fixtures", "linux", "ubuntu2404-single-numa")
	result := Collector{}.Collect(context.Background(), collector.Env{FileSystem: fsx.Rooted{Root: root}, Platform: "linux", Fixture: true})
	if result.State != model.StateAvailable || len(result.Facts) != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
	var got model.CPU
	if err := json.Unmarshal(result.Facts[0].Value, &got); err != nil {
		t.Fatal(err)
	}
	if got.CacheBytes["L1d"] != 32<<10 || got.CacheBytes["L3"] != 8<<20 {
		t.Fatalf("unexpected caches: %v", got.CacheBytes)
	}
}

func TestParseCacheSize(t *testing.T) {
	for raw, want := range map[string]uint64{"32K": 32 << 10, "2M": 2 << 20, "1024": 1024} {
		got, err := parseCacheSize(raw)
		if err != nil || got != want {
			t.Errorf("parseCacheSize(%q)=(%d,%v), want %d", raw, got, err, want)
		}
	}
	if _, err := parseCacheSize("invalid"); err == nil {
		t.Fatal("invalid cache size must fail")
	}
}
