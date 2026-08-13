package nvidia

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/arpingblue/AIStat/internal/clock"
	"github.com/arpingblue/AIStat/internal/collector"
	"github.com/arpingblue/AIStat/internal/execx"
	"github.com/arpingblue/AIStat/internal/fsx"
	"github.com/arpingblue/AIStat/internal/model"
)

type fakeRunner struct{ query, topology string }

func (r fakeRunner) Run(_ context.Context, spec execx.CommandSpec) (execx.Result, error) {
	joined := strings.Join(spec.Args, " ")
	switch {
	case strings.Contains(joined, "--query-gpu="):
		return execx.Result{Stdout: r.query}, nil
	case strings.Contains(joined, "--query-compute-apps="):
		return execx.Result{Stdout: "42, GPU-fixture-0\n"}, nil
	case joined == "":
		return execx.Result{Stdout: "NVIDIA-SMI 580.65.06 Driver Version: 580.65.06 CUDA Version: 13.0"}, nil
	case strings.HasPrefix(joined, "topo "):
		return execx.Result{Stdout: r.topology}, nil
	case strings.HasPrefix(joined, "--time-format"):
		return execx.Result{Stdout: "2026-08-13T10:00:00+00:00 kernel: NVRM: Xid (PCI:0000:3b:00): 79, fallen off bus\n"}, nil
	}
	return execx.Result{}, nil
}

func TestParseCSVFixture(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "fixtures", "nvidia", "single-l4", "query-gpu.csv"))
	if err != nil {
		t.Fatal(err)
	}
	gpus, driver, err := ParseCSV(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(gpus) != 1 || gpus[0].UUID != "GPU-fixture-0" || gpus[0].PCIAddress != "0000:3b:00.0" || driver != "580.65.06" {
		t.Fatalf("unexpected parse: %#v driver=%s", gpus, driver)
	}
	if gpus[0].ECCCurrentErrors == nil || *gpus[0].ECCCurrentErrors != 0 {
		t.Fatal("zero ECC must retain presence")
	}
}

func TestParseXID(t *testing.T) {
	fallback := time.Unix(1, 0).UTC()
	events := ParseXID("2026-08-13T10:00:00+00:00 kernel: NVRM: Xid (PCI:0000:3b:00): 79, GPU has fallen off the bus\n", fallback)
	if len(events) != 1 || events[0].Code != 79 || events[0].Timestamp.Equal(fallback) {
		t.Fatalf("unexpected events: %#v", events)
	}
}

func FuzzParseNvidiaCSV(f *testing.F) {
	f.Add("0, GPU-0, NVIDIA L4, 00000000:3B:00.0, 23034, 0, 0, 40, 30, 72, 72, Enabled, Default, Disabled, 0, 0, 16, 16, 580.65.06\n")
	f.Add("")
	f.Fuzz(func(t *testing.T, raw string) {
		gpus, _, err := ParseCSV(raw)
		if err != nil {
			return
		}
		for _, gpu := range gpus {
			if gpu.Index < 0 {
				t.Fatalf("negative GPU index: %#v", gpu)
			}
		}
	})
}

func FuzzParseNvidiaTopology(f *testing.F) {
	f.Add("\tGPU0\tGPU1\tNIC0\tCPU Affinity\tNUMA Affinity\nGPU0\tX\tNV4\tPIX\t0-3\t0\nGPU1\tNV4\tX\tSYS\t4-7\t1\n")
	f.Add("")
	f.Add("\tGPU0\tGPU1\nGPU0\tX\tFUTURE_TOKEN\n")
	f.Fuzz(func(t *testing.T, raw string) {
		gpus := []model.GPU{{Index: 0, UUID: "GPU-0"}, {Index: 1, UUID: "GPU-1"}}
		links, connections := ParseTopologyMatrix(raw, gpus)
		for _, link := range links {
			if link.FromGPU == "" || link.ToGPU == "" || link.Distance < 0 {
				t.Fatalf("invalid topology link: %#v", link)
			}
		}
		for _, connection := range connections {
			if connection.From == "" || connection.To == "" || connection.Distance < 0 {
				t.Fatalf("invalid topology connection: %#v", connection)
			}
		}
	})
}

func TestCollectFixtureCommands(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "fixtures")
	query, err := os.ReadFile(filepath.Join(root, "nvidia", "single-l4", "query-gpu.csv"))
	if err != nil {
		t.Fatal(err)
	}
	topo, err := os.ReadFile(filepath.Join(root, "nvidia", "single-l4", "topo.txt"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	result := Collector{}.Collect(context.Background(), collector.Env{Runner: fakeRunner{string(query), string(topo)}, FileSystem: fsx.Rooted{Root: root}, Clock: clock.Fixed{Time: now}, Platform: "linux"})
	if result.State != model.StateAvailable || len(result.Facts) != 4 {
		t.Fatalf("unexpected result: %#v", result)
	}
	stack, ok := collector.DecodeFact[model.NVIDIAStack](collector.Env{Facts: map[string]model.Fact{"nvidia": result.Facts[1]}}, "nvidia")
	if !ok || stack.DriverVersion != "580.65.06" || stack.CUDADriver != "13.0" || stack.CUDAToolkit != "13.0" || len(stack.CUDAToolkits) != 2 || stack.CUDACompatPackage == nil || !*stack.CUDACompatPackage || stack.XIDState != model.StateAvailable || len(stack.XIDEvents) != 1 || len(stack.ComputeProcesses) != 1 {
		t.Fatalf("unexpected stack: %#v", stack)
	}
}

func TestParseComputeApps(t *testing.T) {
	got := ParseComputeApps("42, GPU-0\ninvalid, GPU-1\n")
	if len(got) != 1 || got[0].PID != 42 || got[0].GPUUUID != "GPU-0" {
		t.Fatalf("unexpected compute apps: %#v", got)
	}
}
func TestApplyBAR1(t *testing.T) {
	gpus := []model.GPU{{Index: 0}, {Index: 1}}
	applyBAR1(gpus, "0, 1024, 64\n1, [Not Supported], N/A\n")
	if gpus[0].BAR1TotalBytes == nil || *gpus[0].BAR1TotalBytes != 1024<<20 || gpus[0].BAR1UsedBytes == nil || *gpus[0].BAR1UsedBytes != 64<<20 || gpus[1].BAR1TotalBytes != nil {
		t.Fatalf("unexpected BAR1 data: %#v", gpus)
	}
}
func TestParseTopologyUnknownTokenDegrades(t *testing.T) {
	links := ParseTopology("\tGPU0\tGPU1\nGPU0\tX\tFOO\nGPU1\tFOO\tX\n", nil)
	if len(links) != 1 || links[0].Distance != 6 || links[0].Status != "unknown" {
		t.Fatalf("unexpected links: %#v", links)
	}
}

func TestParseTopologyConnectionsAndAffinity(t *testing.T) {
	gpus := []model.GPU{{Index: 0, UUID: "GPU-0"}, {Index: 1, UUID: "GPU-1"}}
	raw := "\tGPU0\tGPU1\tNIC0\tCPU Affinity\tNUMA Affinity\nGPU0\tX\tNV4\tPIX\t0-3\t0\nGPU1\tNV4\tX\tSYS\t4-7\t1\nNIC0\tPIX\tSYS\tX\n"
	links, connections := ParseTopologyMatrix(raw, gpus)
	if len(links) != 1 || links[0].Kind != "NV4" || len(connections) != 2 || connections[0].ToKind != "nic" {
		t.Fatalf("unexpected topology: links=%#v connections=%#v", links, connections)
	}
	if strings.Join(intStrings(gpus[0].CPUAffinity), ",") != "0,1,2,3" || gpus[0].NUMANode == nil || *gpus[0].NUMANode != 0 {
		t.Fatalf("unexpected GPU affinity: %#v", gpus)
	}
}

func TestParseTopologyStripsANSIHeader(t *testing.T) {
	gpus := []model.GPU{{Index: 0, UUID: "GPU-0"}, {Index: 1, UUID: "GPU-1"}, {Index: 2, UUID: "GPU-2"}, {Index: 3, UUID: "GPU-3"}}
	raw := "\t\x1b[4mGPU0\tGPU1\tGPU2\tGPU3\tCPU Affinity\tNUMA Affinity\x1b[0m\n" +
		"GPU0\tX\tNODE\tSYS\tSYS\t0,2,4,6\t0\n" +
		"GPU1\tNODE\tX\tSYS\tSYS\t0,2,4,6\t0\n" +
		"GPU2\tSYS\tSYS\tX\tNODE\t1,3,5,7\t1\n" +
		"GPU3\tSYS\tSYS\tNODE\tX\t1,3,5,7\t1\n"
	links, connections := ParseTopologyMatrix(raw, gpus)
	if len(links) != 6 {
		t.Fatalf("expected six GPU pairs, got %#v", links)
	}
	for _, connection := range connections {
		if strings.Contains(connection.To, "\x1b") || connection.To == "GPU0" {
			t.Fatalf("ANSI header leaked into connection: %#v", connection)
		}
	}
}

func TestCommandStateOperationNotPermitted(t *testing.T) {
	state := commandState(execx.Result{Stderr: "dmesg: read kernel buffer failed: Operation not permitted"}, os.ErrPermission)
	if state != model.StatePermissionDenied {
		t.Fatalf("state=%s", state)
	}
}

func intStrings(values []int) []string {
	result := make([]string, len(values))
	for i, value := range values {
		result[i] = strconv.Itoa(value)
	}
	return result
}
