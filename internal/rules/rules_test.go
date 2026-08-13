package rules

import (
	"testing"
	"time"

	"github.com/arpingblue/AIStat/internal/model"
	"github.com/arpingblue/AIStat/internal/topology"
)

var testNow = time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)

func baseContext() RuleContext {
	s := &model.Snapshot{NUMA: model.NUMAState{State: model.StateAvailable, Nodes: []model.NUMANode{{ID: 0, CPUList: []int{0, 1}}, {ID: 1, CPUList: []int{2, 3}}}}, PCI: model.PCIState{State: model.StateAvailable}, GPUs: model.GPUState{State: model.StateAvailable}, Network: model.NetworkState{State: model.StateAvailable, NICs: []model.NIC{{Name: "eth0", OperState: "up", PCIAddress: "0000:03:00.0", NUMANode: model.Int(0)}}}, RDMA: model.RDMAState{State: model.StateAvailable, Devices: []model.RDMADevice{{Name: "mlx5_0", State: "ACTIVE", PCIAddress: "0000:03:00.0", NUMANode: model.Int(0)}}}, NVIDIA: model.NVIDIAStack{State: model.StateAvailable, DriverVersion: "580.65.06", XIDState: model.StateAvailable}, Containers: model.ContainerState{State: model.StateAvailable, DaemonState: model.StateAvailable, NVIDIARuntime: true}, Processes: model.ProcessState{State: model.StateAvailable}, Runtimes: model.RuntimeState{State: model.StateAvailable}}
	return RuleContext{Snapshot: s, Graph: topology.BuildBase(s), Profile: model.Profile{Name: "general"}, Now: testNow}
}
func boolp(value bool) *bool     { return &value }
func intp(value int) *int        { return &value }
func uintp(value uint64) *uint64 { return &value }
func gpu(index int, id string, node int) model.GPU {
	return model.GPU{Index: index, UUID: id, Name: "GPU", NUMANode: model.Int(node), PCIAddress: "0000:0" + string(rune('1'+index)) + ":00.0", ComputeMode: "Default", ECCCurrentErrors: uintp(0)}
}
func withRuntime(ctx RuleContext, runtime model.RuntimeInstance) RuleContext {
	ctx.Snapshot.Runtimes.Instances = []model.RuntimeInstance{runtime}
	return ctx
}
func statusOf(t *testing.T, item Rule, ctx RuleContext) model.Status {
	t.Helper()
	ctx.Graph = topology.BuildBase(ctx.Snapshot)
	findings := item.Evaluate(ctx)
	if len(findings) == 0 {
		t.Fatalf("%s returned no finding", item.ID())
	}
	return findings[0].Status
}

type ruleContexts struct {
	trigger, pass, missing RuleContext
	triggerStatus          model.Status
}

func contextsFor(id string) (ruleContexts, bool) {
	a, b, c := baseContext(), baseContext(), baseContext()
	status := model.StatusFail
	switch id {
	case "GPU001":
		a.Snapshot.PCI.Devices = []model.PCIDevice{{VendorID: "0x10de", Class: "0x030200"}}
		a.Snapshot.NVIDIA.DriverUsable = boolp(false)
		b.Snapshot.PCI.Devices = []model.PCIDevice{{VendorID: "0x10de", Class: "0x030200"}}
		b.Snapshot.NVIDIA.DriverUsable = boolp(true)
		c.Snapshot.PCI.State = model.StatePermissionDenied
	case "GPU002":
		a.Snapshot.GPUs.Devices = []model.GPU{{Index: 0, ComputeMode: "PROHIBITED"}}
		b.Snapshot.GPUs.Devices = []model.GPU{{Index: 0, ComputeMode: "Default"}}
		c.Snapshot.GPUs.State = model.StateUnknown
	case "GPU003":
		a.Snapshot.GPUs.Devices = []model.GPU{{Index: 0, ECCCurrentErrors: uintp(1)}}
		b.Snapshot.GPUs.Devices = []model.GPU{{Index: 0, ECCCurrentErrors: uintp(0)}}
		c.Snapshot.GPUs.Devices = []model.GPU{{Index: 0}}
	case "GPU004":
		a.Snapshot.GPUs.Devices = []model.GPU{{Index: 0}}
		a.Snapshot.NVIDIA.XIDEvents = []model.XIDEvent{{Timestamp: testNow.Add(-time.Hour), Code: 79, GPU: "GPU-0"}}
		b.Snapshot.GPUs.Devices = []model.GPU{{Index: 0}}
		c.Snapshot.GPUs.Devices = []model.GPU{{Index: 0}}
		c.Snapshot.NVIDIA.XIDState = model.StatePermissionDenied
	case "PCIE001":
		status = model.StatusWarn
		a.Snapshot.GPUs.Devices = []model.GPU{{Index: 0, Active: true, PCIELinkWidth: intp(8), PCIEMaxLinkWidth: intp(16)}}
		b.Snapshot.GPUs.Devices = []model.GPU{{Index: 0, Active: true, PCIELinkWidth: intp(16), PCIEMaxLinkWidth: intp(16)}}
		c.Snapshot.GPUs.Devices = []model.GPU{{Index: 0, Active: true}}
	case "PCIE002":
		status = model.StatusWarn
		a.Profile.GDRRequired = true
		a.Snapshot.PCI.Devices = []model.PCIDevice{{Address: "0000:00:01.0", ACSRedirect: boolp(true)}}
		b.Profile.GDRRequired = true
		b.Snapshot.PCI.Devices = []model.PCIDevice{{Address: "0000:00:01.0", ACSRedirect: boolp(false)}}
		c.Profile.GDRRequired = true
		c.Snapshot.PCI.Devices = []model.PCIDevice{{Address: "0000:00:01.0"}}
	case "NUMA001":
		status = model.StatusWarn
		a.Snapshot.GPUs.Devices = []model.GPU{gpu(0, "g0", 0)}
		a = withRuntime(a, model.RuntimeInstance{Kind: "vllm", GPUs: []string{"g0"}, CPUSet: []int{2}})
		b.Snapshot.GPUs.Devices = []model.GPU{gpu(0, "g0", 0)}
		b = withRuntime(b, model.RuntimeInstance{Kind: "vllm", GPUs: []string{"g0"}, CPUSet: []int{0}})
		c.Snapshot.GPUs.Devices = []model.GPU{gpu(0, "g0", 0)}
		c = withRuntime(c, model.RuntimeInstance{Kind: "vllm", GPUs: []string{"g0"}})
	case "NUMA002":
		status = model.StatusWarn
		a.Snapshot.GPUs.Devices = []model.GPU{gpu(0, "g0", 0)}
		a = withRuntime(a, model.RuntimeInstance{Kind: "vllm", GPUs: []string{"g0"}, NUMAMems: []int{1}})
		b.Snapshot.GPUs.Devices = []model.GPU{gpu(0, "g0", 0)}
		b = withRuntime(b, model.RuntimeInstance{Kind: "vllm", GPUs: []string{"g0"}, NUMAMems: []int{0}})
		c.Snapshot.GPUs.Devices = []model.GPU{gpu(0, "g0", 0)}
		c = withRuntime(c, model.RuntimeInstance{Kind: "vllm", GPUs: []string{"g0"}})
	case "TOPO001":
		status = model.StatusWarn
		a.Snapshot.GPUs.Devices = []model.GPU{gpu(0, "0", 0), gpu(1, "1", 0), gpu(2, "2", 1)}
		a.Snapshot.P2P = []model.P2PLink{{FromGPU: "0", ToGPU: "1", Kind: "PIX"}, {FromGPU: "0", ToGPU: "2", Kind: "SYS"}, {FromGPU: "1", ToGPU: "2", Kind: "SYS"}}
		a = withRuntime(a, model.RuntimeInstance{Kind: "vllm", GPUs: []string{"0", "2"}})
		b.Snapshot.GPUs.Devices = []model.GPU{gpu(0, "0", 0), gpu(1, "1", 0)}
		b = withRuntime(b, model.RuntimeInstance{Kind: "vllm", GPUs: []string{"0", "1"}})
		c.Snapshot.GPUs.Devices = []model.GPU{gpu(0, "0", 0), gpu(1, "1", 0), gpu(2, "2", 1)}
		c = withRuntime(c, model.RuntimeInstance{Kind: "vllm", GPUs: []string{"0", "2"}})
	case "NET001":
		a = withRuntime(a, model.RuntimeInstance{SelectedNICs: []string{"eth9"}})
		b = withRuntime(b, model.RuntimeInstance{SelectedNICs: []string{"eth0"}})
		c.Snapshot.Network.State = model.StateUnknown
		c = withRuntime(c, model.RuntimeInstance{SelectedNICs: []string{"eth0"}})
	case "NET002":
		status = model.StatusWarn
		a = networkPathContext(false)
		b = networkPathContext(true)
		c.Profile.GDRRequired = true
	case "NET003":
		a.Profile.RDMARequired = true
		a.Snapshot.Containers.Devices = []model.Container{{ID: "c", MemlockSoft: uintp(1024)}}
		b.Profile.RDMARequired = true
		b.Snapshot.Containers.Devices = []model.Container{{ID: "c", MemlockSoft: uintp(2 << 30)}}
		c.Profile.RDMARequired = true
		c.Snapshot.Containers.Devices = []model.Container{{ID: "c"}}
	case "NCCL001":
		a.Snapshot.Network.NICs = nil
		a = withRuntime(a, model.RuntimeInstance{Details: map[string]string{"NCCL_SOCKET_IFNAME": "eth9"}})
		b = withRuntime(b, model.RuntimeInstance{Details: map[string]string{"NCCL_SOCKET_IFNAME": "eth0"}})
		c.Snapshot.Network.State = model.StateUnknown
		c = withRuntime(c, model.RuntimeInstance{Details: map[string]string{"NCCL_SOCKET_IFNAME": "eth0"}})
	case "CUDA001":
		a.Snapshot.NVIDIA.DriverVersion = "525.60.12"
		a = withRuntime(a, model.RuntimeInstance{CUDAVersion: "12.8"})
		b = withRuntime(b, model.RuntimeInstance{CUDAVersion: "12.8"})
		c.Snapshot.NVIDIA.DriverVersion = ""
		c = withRuntime(c, model.RuntimeInstance{CUDAVersion: "12.8"})
	case "CUDA002":
		status = model.StatusWarn
		a.Snapshot.NVIDIA.DriverVersion = "535.183.01"
		b.Snapshot.NVIDIA.DriverVersion = "535.183.01"
		b.Now = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		c.Snapshot.NVIDIA.DriverVersion = ""
		c.Snapshot.GPUs.Devices = []model.GPU{{Index: 0}}
	case "TORCH001":
		a = withRuntime(a, model.RuntimeInstance{Kind: "pytorch", CUDAAvailable: boolp(false)})
		b = withRuntime(b, model.RuntimeInstance{Kind: "pytorch", CUDAAvailable: boolp(true)})
		c = withRuntime(c, model.RuntimeInstance{Kind: "pytorch", CUDAVersion: "12.8"})
	case "TORCH002":
		a = withRuntime(a, model.RuntimeInstance{Kind: "pytorch", LocalWorldSize: intp(4), GPUCount: intp(2)})
		b = withRuntime(b, model.RuntimeInstance{Kind: "pytorch", LocalWorldSize: intp(2), GPUCount: intp(2)})
		c = withRuntime(c, model.RuntimeInstance{Kind: "pytorch"})
	case "CTR001":
		a.Profile.DockerRequired = true
		a.Snapshot.Containers.DaemonState = model.StateNotDetected
		b.Profile.DockerRequired = true
		c.Profile.DockerRequired = true
		c.Snapshot.Containers.DaemonState = model.StateUnknown
	case "CTR002":
		a.Profile.DockerRequired = true
		a.Profile.GPURequired = true
		a.Snapshot.Containers.NVIDIARuntime = false
		b.Profile.DockerRequired = true
		b.Profile.GPURequired = true
		c.Profile.DockerRequired = true
		c.Profile.GPURequired = true
		c.Snapshot.Containers.State = model.StateUnknown
	case "CTR003":
		a.Snapshot.Containers.Devices = []model.Container{{ID: "c", GPURequired: true}}
		b.Snapshot.Containers.Devices = []model.Container{{ID: "c", GPURequired: true, GPUAccess: true, GPUUUIDs: []string{"g0"}}}
		c.Profile.DockerRequired = true
		c.Profile.GPURequired = true
		c.Snapshot.Containers.State = model.StateUnknown
	case "CTR004":
		a.Profile.MultiProcess = true
		a.Snapshot.Containers.Devices = []model.Container{{ID: "c", SHMSize: uintp(64 << 20)}}
		b.Profile.MultiProcess = true
		b.Snapshot.Containers.Devices = []model.Container{{ID: "c", SHMSize: uintp(1 << 30)}}
		c.Profile.MultiProcess = true
		c.Snapshot.Containers.Devices = []model.Container{{ID: "c"}}
	case "VLLM001":
		a = withRuntime(a, model.RuntimeInstance{Kind: "vllm", LocalWorldSize: intp(4), GPUs: []string{"g0", "g1"}})
		b = withRuntime(b, model.RuntimeInstance{Kind: "vllm", LocalWorldSize: intp(2), GPUs: []string{"g0", "g1"}})
		c = withRuntime(c, model.RuntimeInstance{Kind: "vllm"})
	case "VLLM002":
		a.Snapshot.GPUs.Devices = []model.GPU{gpu(0, "g0", 0)}
		a = withRuntime(a, model.RuntimeInstance{Kind: "vllm", Details: map[string]string{"CUDA_VISIBLE_DEVICES": "0,0"}})
		b.Snapshot.GPUs.Devices = []model.GPU{gpu(0, "g0", 0)}
		b = withRuntime(b, model.RuntimeInstance{Kind: "vllm", Details: map[string]string{"CUDA_VISIBLE_DEVICES": "0"}})
		c.Snapshot.GPUs.State = model.StateUnknown
		c = withRuntime(c, model.RuntimeInstance{Kind: "vllm", Details: map[string]string{"CUDA_VISIBLE_DEVICES": "0"}})
	case "SGL001":
		a = withRuntime(a, model.RuntimeInstance{Kind: "sglang", LocalWorldSize: intp(4), GPUs: []string{"g0"}})
		b = withRuntime(b, model.RuntimeInstance{Kind: "sglang", LocalWorldSize: intp(1), GPUs: []string{"g0"}})
		c = withRuntime(c, model.RuntimeInstance{Kind: "sglang"})
	case "SGL002":
		a.Snapshot.RDMA.Devices = nil
		a = withRuntime(a, model.RuntimeInstance{Kind: "sglang", Disaggregation: true, SelectedHCAs: []string{"mlx5_9"}})
		b = withRuntime(b, model.RuntimeInstance{Kind: "sglang", Disaggregation: true, SelectedHCAs: []string{"mlx5_0"}})
		c.Snapshot.RDMA.State = model.StateUnknown
		c = withRuntime(c, model.RuntimeInstance{Kind: "sglang", Disaggregation: true, SelectedHCAs: []string{"mlx5_0"}})
	default:
		return ruleContexts{}, false
	}
	return ruleContexts{a, b, c, status}, true
}

func networkPathContext(shared bool) RuleContext {
	ctx := baseContext()
	ctx.Profile.GDRRequired = true
	ctx.Snapshot.PCI.Devices = []model.PCIDevice{{Address: "0000:00:01.0"}, {Address: "0000:00:02.0"}, {Address: "0000:01:00.0", Parent: "0000:00:01.0"}, {Address: "0000:02:00.0", Parent: "0000:00:01.0"}, {Address: "0000:03:00.0", Parent: "0000:00:02.0"}}
	if shared {
		ctx.Snapshot.Network.NICs[0].PCIAddress = "0000:01:00.0"
	}
	ctx.Snapshot.GPUs.Devices = []model.GPU{{Index: 0, UUID: "g0", PCIAddress: "0000:01:00.0"}, {Index: 1, UUID: "g1", PCIAddress: "0000:02:00.0"}}
	ctx = withRuntime(ctx, model.RuntimeInstance{Kind: "vllm", GPUs: []string{"g0", "g1"}, SelectedNICs: []string{"eth0"}})
	return ctx
}

func TestFrozenRuleSetHasTriggerPassAndMissingEvidenceCases(t *testing.T) {
	engine := Default()
	if len(engine.Rules()) != 25 {
		t.Fatalf("got %d rules, want 25", len(engine.Rules()))
	}
	seen := map[RuleID]bool{}
	for _, item := range engine.Rules() {
		if seen[item.ID()] {
			t.Fatalf("duplicate rule %s", item.ID())
		}
		seen[item.ID()] = true
		contexts, ok := contextsFor(string(item.ID()))
		if !ok {
			t.Fatalf("no tests for %s", item.ID())
		}
		t.Run(string(item.ID())+"_trigger", func(t *testing.T) {
			if got := statusOf(t, item, contexts.trigger); got != contexts.triggerStatus {
				t.Fatalf("got %s want %s", got, contexts.triggerStatus)
			}
		})
		t.Run(string(item.ID())+"_pass", func(t *testing.T) {
			if got := statusOf(t, item, contexts.pass); got != model.StatusPass {
				t.Fatalf("got %s want pass", got)
			}
		})
		t.Run(string(item.ID())+"_missing", func(t *testing.T) {
			if got := statusOf(t, item, contexts.missing); got != model.StatusUnknown {
				t.Fatalf("got %s want unknown", got)
			}
		})
	}
}

func skipContextFor(id string) RuleContext {
	ctx := baseContext()
	switch id {
	case "GPU001":
		ctx.Snapshot.PCI.Devices = nil
	case "GPU002", "GPU003", "GPU004", "PCIE001":
		ctx.Snapshot.GPUs.State = model.StateNotDetected
	case "PCIE002":
		ctx.Profile.GDRRequired = false
		ctx.Snapshot.P2P = nil
	case "NUMA001", "NUMA002", "TOPO001", "TORCH001", "TORCH002", "VLLM001", "VLLM002", "SGL001", "SGL002":
		ctx.Snapshot.Runtimes.Instances = nil
	case "NET001", "NCCL001":
		ctx.Snapshot.Runtimes.Instances = nil
	case "NET002":
		ctx.Profile.GDRRequired = false
	case "NET003":
		ctx.Profile.RDMARequired = false
		ctx.Snapshot.RDMA.Devices = nil
		ctx.Snapshot.RDMA.State = model.StateNotDetected
	case "CUDA001":
		ctx.Snapshot.Runtimes.Instances = nil
	case "CUDA002":
		ctx.Snapshot.NVIDIA.DriverVersion = ""
		ctx.Snapshot.GPUs.State = model.StateNotDetected
	case "CTR001":
		ctx.Profile.DockerRequired = false
		ctx.Snapshot.Containers.Devices = nil
	case "CTR002":
		ctx.Profile.DockerRequired = false
		ctx.Profile.GPURequired = false
		ctx.Snapshot.Containers.Devices = nil
	case "CTR003":
		ctx.Snapshot.Containers.Devices = nil
	case "CTR004":
		ctx.Profile.MultiProcess = false
	}
	return ctx
}
func TestFrozenRuleSetSkipCases(t *testing.T) {
	for _, item := range Default().Rules() {
		t.Run(string(item.ID()), func(t *testing.T) {
			if got := statusOf(t, item, skipContextFor(string(item.ID()))); got != model.StatusSkip {
				t.Fatalf("got %s want skip", got)
			}
		})
	}
}

func TestFalsePositiveRegressions(t *testing.T) {
	rulesByID := map[string]Rule{}
	for _, item := range Default().Rules() {
		rulesByID[string(item.ID())] = item
	}
	ctx := baseContext()
	ctx.Snapshot.GPUs.Devices = []model.GPU{{Index: 0, Active: false, PCIELinkWidth: intp(1), PCIEMaxLinkWidth: intp(16)}}
	if got := statusOf(t, rulesByID["PCIE001"], ctx); got != model.StatusSkip {
		t.Fatalf("idle GPU link must not warn: %s", got)
	}
	ctx = baseContext()
	ctx.Profile.GDRRequired = false
	ctx.Snapshot.PCI.Devices = []model.PCIDevice{{Address: "0000:00:01.0", ACSRedirect: boolp(true)}}
	if got := statusOf(t, rulesByID["PCIE002"], ctx); got != model.StatusSkip {
		t.Fatalf("ACS without P2P/GDR context must skip: %s", got)
	}
}

func TestReadinessAggregation(t *testing.T) {
	engine := NewEngine(rule{id: "D", meta: RuleMeta{Dimension: model.DimensionDeployment}, evaluate: func(RuleContext, rule) []model.Finding {
		return []model.Finding{{RuleID: "D", Dimension: model.DimensionDeployment, Status: model.StatusFail}}
	}}, rule{id: "P", meta: RuleMeta{Dimension: model.DimensionPerformance}, evaluate: func(RuleContext, rule) []model.Finding {
		return []model.Finding{{RuleID: "P", Dimension: model.DimensionPerformance, Status: model.StatusWarn}}
	}})
	_, readiness, summary := engine.Evaluate(baseContext())
	if readiness.Deployment != "not_ready" || readiness.Performance != "warn" || summary.Fail != 1 || summary.Warn != 1 {
		t.Fatalf("unexpected aggregation: %#v %#v", readiness, summary)
	}
}
