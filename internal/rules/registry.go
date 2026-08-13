package rules

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/arpingblue/AIStat/internal/compat"
	"github.com/arpingblue/AIStat/internal/model"
	"github.com/arpingblue/AIStat/internal/topology"
)

func Default() Engine {
	refs := map[string][]model.Reference{
		"nvidia": {{Title: "NVIDIA System Management Interface", URL: "https://docs.nvidia.com/deploy/nvidia-smi/"}},
		"cuda":   {{Title: "CUDA Compatibility", URL: "https://docs.nvidia.com/deploy/cuda-compatibility/"}},
		"nccl":   {{Title: "NCCL Troubleshooting", URL: "https://docs.nvidia.com/deeplearning/nccl/user-guide/docs/troubleshooting.html"}},
		"docker": {{Title: "NVIDIA Container Toolkit", URL: "https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/"}},
		"torch":  {{Title: "PyTorch CUDA API", URL: "https://docs.pytorch.org/docs/stable/cuda.html"}},
		"linux":  {{Title: "Linux cgroup v2", URL: "https://docs.kernel.org/admin-guide/cgroup-v2.html"}},
		"vllm":   {{Title: "vLLM documentation", URL: "https://docs.vllm.ai/"}},
		"sglang": {{Title: "SGLang documentation", URL: "https://docs.sglang.ai/"}},
	}
	specs := []rule{
		makeRule("GPU001", "NVIDIA device detected but driver unusable", "nvidia", model.DimensionDeployment, model.PriorityP0, refs["nvidia"], evalGPU001),
		makeRule("GPU002", "GPU compute mode prohibits compute", "nvidia", model.DimensionDeployment, model.PriorityP0, refs["nvidia"], evalGPU002),
		makeRule("GPU003", "Volatile uncorrectable ECC errors detected", "nvidia", model.DimensionDeployment, model.PriorityP0, refs["nvidia"], evalGPU003),
		makeRule("GPU004", "Recent critical NVIDIA Xid detected", "nvidia", model.DimensionDeployment, model.PriorityP0, refs["nvidia"], evalGPU004),
		makeRule("PCIE001", "Active GPU negotiated below max link width", "pcie", model.DimensionPerformance, model.PriorityP1, refs["nvidia"], evalPCIE001),
		makeRule("PCIE002", "ACS may redirect peer traffic", "pcie", model.DimensionPerformance, model.PriorityP1, refs["nccl"], evalPCIE002),
		makeRule("NUMA001", "AI process has no GPU-local CPU", "numa", model.DimensionPerformance, model.PriorityP1, refs["linux"], evalNUMA001),
		makeRule("NUMA002", "Effective memory nodes exclude GPU-local NUMA", "numa", model.DimensionPerformance, model.PriorityP1, refs["linux"], evalNUMA002),
		makeRule("TOPO001", "Selected multi-GPU group is strictly dominated", "topology", model.DimensionPerformance, model.PriorityP1, refs["nccl"], evalTOPO001),
		makeRule("NET001", "Explicitly selected network device unavailable", "network", model.DimensionDeployment, model.PriorityP0, refs["nccl"], evalNET001),
		makeRule("NET002", "GPUDirect path crosses unsupported PCIe root arrangement", "network", model.DimensionPerformance, model.PriorityP1, refs["nccl"], evalNET002),
		makeRule("NET003", "RDMA/NCCL memlock constrained", "network", model.DimensionDeployment, model.PriorityP0, refs["nccl"], evalNET003),
		makeRule("NCCL001", "Explicit NCCL interface filter matches no usable device", "nccl", model.DimensionDeployment, model.PriorityP0, refs["nccl"], evalNCCL001),
		makeRule("CUDA001", "Active CUDA runtime incompatible with driver", "cuda", model.DimensionDeployment, model.PriorityP0, refs["cuda"], evalCUDA001),
		makeRule("CUDA002", "NVIDIA driver branch reached lifecycle EOL", "cuda", model.DimensionDeployment, model.PriorityP2, refs["nvidia"], evalCUDA002),
		makeRule("TORCH001", "GPU runtime expects CUDA but PyTorch reports unavailable", "pytorch", model.DimensionDeployment, model.PriorityP0, refs["torch"], evalTORCH001),
		makeRule("TORCH002", "Effective PyTorch GPU count below runtime requirement", "pytorch", model.DimensionDeployment, model.PriorityP0, refs["torch"], evalTORCH002),
		makeRule("CTR001", "Docker required but daemon unavailable", "container", model.DimensionDeployment, model.PriorityP0, refs["docker"], evalCTR001),
		makeRule("CTR002", "NVIDIA Container Toolkit missing or unconfigured", "container", model.DimensionDeployment, model.PriorityP0, refs["docker"], evalCTR002),
		makeRule("CTR003", "GPU-required container has no effective GPU visibility", "container", model.DimensionDeployment, model.PriorityP0, refs["docker"], evalCTR003),
		makeRule("CTR004", "Multi-process NCCL container has default or tiny shared memory", "container", model.DimensionDeployment, model.PriorityP0, refs["nccl"], evalCTR004),
		makeRule("VLLM001", "vLLM local GPU requirement exceeds visibility", "vllm", model.DimensionDeployment, model.PriorityP0, refs["vllm"], evalVLLM001),
		makeRule("VLLM002", "vLLM GPU selection contains invalid device reference", "vllm", model.DimensionDeployment, model.PriorityP0, refs["vllm"], evalVLLM002),
		makeRule("SGL001", "SGLang local GPU requirement exceeds visibility", "sglang", model.DimensionDeployment, model.PriorityP0, refs["sglang"], evalSGL001),
		makeRule("SGL002", "SGLang disaggregation HCA invalid", "sglang", model.DimensionDeployment, model.PriorityP0, refs["sglang"], evalSGL002),
	}
	items := make([]Rule, len(specs))
	for i := range specs {
		items[i] = specs[i]
	}
	return NewEngine(items...)
}

func makeRule(id, title, domain string, dimension model.Dimension, priority model.Priority, references []model.Reference, evaluate func(RuleContext, rule) []model.Finding) rule {
	return rule{id: RuleID(id), meta: RuleMeta{Title: title, Domain: domain, Dimension: dimension, Priority: priority, Confidence: model.ConfidenceHigh, Description: title + ". The rule evaluates normalized evidence only and never changes host state.", References: references}, evaluate: evaluate}
}

func evalGPU001(ctx RuleContext, r rule) []model.Finding {
	if ctx.Snapshot.PCI.State != model.StateAvailable {
		return unknown(r, "PCI inventory is unavailable.")
	}
	detected := false
	for _, device := range ctx.Snapshot.PCI.Devices {
		if strings.EqualFold(device.VendorID, "0x10de") && strings.HasPrefix(strings.ToLower(device.Class), "0x03") {
			detected = true
			break
		}
	}
	if !detected {
		return skip(r, "No NVIDIA display or compute PCI device was detected.")
	}
	if ctx.Snapshot.NVIDIA.DriverUsable == nil {
		return unknown(r, "An NVIDIA PCI device exists, but driver usability is unknown.")
	}
	if !*ctx.Snapshot.NVIDIA.DriverUsable {
		return []model.Finding{finding(r, model.StatusFail, model.SeverityHigh, model.Subject{Kind: "node", ID: "host"}, "NVIDIA PCI GPU detected but the driver stack is unusable.", "The NVIDIA kernel and userspace driver stack is queryable.", "CUDA workloads cannot initialize reliably.", "Repair the NVIDIA driver, kernel module, and userspace library installation.")}
	}
	return pass(r, "The NVIDIA driver stack is usable.")
}
func evalGPU002(ctx RuleContext, r rule) []model.Finding {
	if ctx.Snapshot.GPUs.State == model.StateNotDetected {
		return skip(r, "No NVIDIA GPU is present.")
	}
	if ctx.Snapshot.GPUs.State != model.StateAvailable {
		return unknown(r, "GPU compute mode could not be read.")
	}
	out := []model.Finding{}
	for _, gpu := range ctx.Snapshot.GPUs.Devices {
		if strings.EqualFold(gpu.ComputeMode, "prohibited") {
			out = append(out, finding(r, model.StatusFail, model.SeverityHigh, gpuSubject(gpu), "GPU compute mode is PROHIBITED.", "Selected GPUs permit compute.", "The selected GPU rejects compute contexts.", "Move the workload to an enabled GPU or have an administrator correct compute mode."))
		}
	}
	if len(out) == 0 {
		return pass(r, "No GPU reports prohibited compute mode.")
	}
	return out
}
func evalGPU003(ctx RuleContext, r rule) []model.Finding {
	if ctx.Snapshot.GPUs.State == model.StateNotDetected {
		return skip(r, "No NVIDIA GPU is present.")
	}
	if ctx.Snapshot.GPUs.State != model.StateAvailable {
		return unknown(r, "GPU ECC counters are unavailable.")
	}
	out := []model.Finding{}
	known := false
	for _, gpu := range ctx.Snapshot.GPUs.Devices {
		if gpu.ECCCurrentErrors == nil {
			continue
		}
		known = true
		if *gpu.ECCCurrentErrors > 0 {
			out = append(out, finding(r, model.StatusFail, model.SeverityHigh, gpuSubject(gpu), fmt.Sprintf("GPU reports %d volatile uncorrectable ECC errors.", *gpu.ECCCurrentErrors), "Volatile uncorrectable ECC count is zero.", "The GPU may return corrupted results or fail workloads.", "Drain the GPU and follow NVIDIA or vendor diagnostics."))
		}
	}
	if len(out) > 0 {
		return out
	}
	if !known {
		return unknown(r, "No reliable volatile uncorrectable ECC counter was collected.")
	}
	return pass(r, "Volatile uncorrectable ECC counters are zero.")
}
func evalGPU004(ctx RuleContext, r rule) []model.Finding {
	if ctx.Snapshot.GPUs.State == model.StateNotDetected {
		return skip(r, "No NVIDIA GPU is present.")
	}
	if ctx.Snapshot.NVIDIA.XIDState != model.StateAvailable {
		return unknown(r, "Recent NVIDIA Xid events could not be inspected.")
	}
	critical := map[int]bool{31: true, 43: true, 48: true, 63: true, 64: true, 79: true, 92: true, 94: true, 95: true}
	out := []model.Finding{}
	for _, event := range ctx.Snapshot.NVIDIA.XIDEvents {
		if critical[event.Code] && ctx.Now.Sub(event.Timestamp) <= 24*time.Hour {
			out = append(out, finding(r, model.StatusFail, model.SeverityHigh, model.Subject{Kind: "gpu", ID: event.GPU}, fmt.Sprintf("Critical NVIDIA Xid %d occurred within 24 hours.", event.Code), "No recent critical Xid event.", "The GPU or driver may be unstable.", "Follow NVIDIA Xid guidance and drain or diagnose the affected GPU.", model.Evidence{Fact: "nvidia.xid", Value: event.Code, Source: "kernel log"}))
		}
	}
	if len(out) == 0 {
		return pass(r, "No critical NVIDIA Xid was observed in the 24-hour window.")
	}
	return out
}
func evalPCIE001(ctx RuleContext, r rule) []model.Finding {
	if ctx.Snapshot.GPUs.State == model.StateNotDetected {
		return skip(r, "No NVIDIA GPU is present.")
	}
	if ctx.Snapshot.GPUs.State != model.StateAvailable {
		return unknown(r, "GPU PCIe link facts are unavailable.")
	}
	out := []model.Finding{}
	known := false
	active := false
	for _, gpu := range ctx.Snapshot.GPUs.Devices {
		if !gpu.Active {
			continue
		}
		active = true
		if gpu.PCIELinkWidth == nil || gpu.PCIEMaxLinkWidth == nil {
			continue
		}
		known = true
		if *gpu.PCIELinkWidth < *gpu.PCIEMaxLinkWidth {
			out = append(out, finding(r, model.StatusWarn, model.SeverityHigh, gpuSubject(gpu), fmt.Sprintf("Active GPU negotiated PCIe x%d while capability is x%d.", *gpu.PCIELinkWidth, *gpu.PCIEMaxLinkWidth), "Active GPU negotiates its expected link width.", "Host-device transfer bandwidth may be constrained.", "Inspect the slot, riser, BIOS, and upstream PCIe path under load."))
		}
	}
	if len(out) > 0 {
		return out
	}
	if !known {
		if active {
			return unknown(r, "An active GPU was detected, but comparable current and maximum PCIe widths are unavailable.")
		}
		return skip(r, "No active GPU with comparable current and maximum link width was found.")
	}
	return pass(r, "Active GPU PCIe widths match their reported capability.")
}
func evalPCIE002(ctx RuleContext, r rule) []model.Finding {
	if !ctx.Profile.GDRRequired && len(ctx.Snapshot.P2P) == 0 {
		return skip(r, "No P2P or GPUDirect workload context is active.")
	}
	if ctx.Snapshot.PCI.State != model.StateAvailable {
		return unknown(r, "PCI bridge ACS state is unavailable.")
	}
	known := false
	for _, device := range ctx.Snapshot.PCI.Devices {
		if device.ACSRedirect != nil {
			known = true
			if *device.ACSRedirect {
				return []model.Finding{finding(r, model.StatusWarn, model.SeverityHigh, model.Subject{Kind: "pci", ID: device.Address}, "ACS redirect is enabled on a relevant PCI bridge.", "Peer traffic is not forced through an avoidable root-complex path.", "P2P or GPUDirect throughput and latency may degrade.", "Validate platform ACS/IOMMU policy with the OEM; AIStat will not disable security features.")}
			}
		}
	}
	if !known {
		return unknown(r, "No reliable ACS bridge state was collected for the peer path.")
	}
	return pass(r, "No ACS redirect flag was detected on inspected PCI bridges.")
}
func evalNUMA001(ctx RuleContext, r rule) []model.Finding { return evalNUMA(ctx, r, false) }
func evalNUMA002(ctx RuleContext, r rule) []model.Finding { return evalNUMA(ctx, r, true) }
func evalNUMA(ctx RuleContext, r rule, memory bool) []model.Finding {
	if len(ctx.Snapshot.Runtimes.Instances) == 0 {
		return skip(r, "No active AI runtime with GPU placement was detected.")
	}
	out := []model.Finding{}
	applicable := false
	for _, runtime := range ctx.Snapshot.Runtimes.Instances {
		if len(runtime.GPUs) == 0 {
			continue
		}
		for _, gpuRef := range runtime.GPUs {
			gpu, ok := findGPU(ctx.Snapshot, gpuRef)
			if !ok || gpu.NUMANode == nil {
				continue
			}
			applicable = true
			if memory {
				if len(runtime.NUMAMems) == 0 {
					return unknown(r, "Effective runtime NUMA memory nodes are unavailable.")
				}
				if !contains(runtime.NUMAMems, *gpu.NUMANode) {
					out = append(out, finding(r, model.StatusWarn, model.SeverityHigh, model.Subject{Kind: "runtime", ID: strconv.Itoa(runtime.PID), Name: runtime.Kind}, fmt.Sprintf("Effective memory nodes exclude GPU-local NUMA node %d.", *gpu.NUMANode), "GPU-local NUMA memory is allowed.", "Pinned memory and host-device transfers may cross sockets.", "Include the GPU-local memory node in the effective cpuset memory policy."))
				}
			} else {
				local := numaCPUs(ctx.Snapshot, *gpu.NUMANode)
				if len(local) == 0 || len(runtime.CPUSet) == 0 {
					return unknown(r, "Runtime CPU affinity or GPU-local CPU inventory is unavailable.")
				}
				if !intersects(local, runtime.CPUSet) {
					out = append(out, finding(r, model.StatusWarn, model.SeverityHigh, model.Subject{Kind: "runtime", ID: strconv.Itoa(runtime.PID), Name: runtime.Kind}, "AI runtime CPU affinity does not overlap the selected GPU-local CPUs.", "Runtime CPU affinity overlaps GPU-local CPUs.", "Cross-socket traffic may increase latency and reduce throughput.", "Test pinning workers to CPUs local to the selected GPU."))
				}
			}
		}
	}
	if len(out) > 0 {
		return out
	}
	if !applicable {
		return skip(r, "No runtime-to-GPU NUMA mapping was available.")
	}
	return pass(r, "Runtime CPU and memory placement satisfies the evaluated GPU-local NUMA condition.")
}
func evalTOPO001(ctx RuleContext, r rule) []model.Finding {
	selected := selectedGPUGroup(ctx.Snapshot)
	if len(selected) < 2 {
		return skip(r, "No selected multi-GPU group was detected.")
	}
	all := gpuIDs(ctx.Snapshot)
	if len(all) <= len(selected) {
		return pass(r, "No same-size alternative GPU group exists.")
	}
	selectedScore, ok := pairScores(ctx.Snapshot, selected)
	if !ok {
		return unknown(r, "Selected GPU pair topology is incomplete.")
	}
	candidate, dominated := dominatingGroup(ctx.Snapshot, all, len(selected), selectedScore)
	if !dominated {
		return pass(r, "No strictly dominating visible GPU group was found.")
	}
	return []model.Finding{finding(r, model.StatusWarn, model.SeverityHigh, model.Subject{Kind: "gpu_group", ID: strings.Join(selected, ",")}, "The selected GPU set is strictly dominated by an available set.", "No visible same-size group is strictly better on all known GPU pairs.", "Collective communication may take a slower topology path.", "Benchmark the alternative group; AIStat will not reorder devices automatically.", model.Evidence{Fact: "topology.alternative_group", Value: candidate, Source: "normalized P2P topology"})}
}
func evalNET001(ctx RuleContext, r rule) []model.Finding {
	selectedNICs, selectedHCAs := selectedNetwork(ctx.Snapshot)
	if len(selectedNICs)+len(selectedHCAs) == 0 {
		return skip(r, "No runtime explicitly selected a NIC or HCA.")
	}
	if ctx.Snapshot.Network.State != model.StateAvailable || ctx.Snapshot.RDMA.State == model.StatePermissionDenied {
		return unknown(r, "Selected network devices cannot be verified from inventory.")
	}
	bad := invalidNetwork(ctx.Snapshot, selectedNICs, selectedHCAs)
	if len(bad) > 0 {
		return []model.Finding{finding(r, model.StatusFail, model.SeverityHigh, model.Subject{Kind: "network", ID: strings.Join(bad, ",")}, "An explicitly selected NIC or HCA is absent or down.", "Every explicitly selected network device is present and usable.", "Distributed runtime initialization or traffic can fail.", "Correct the selection or restore the selected devices.")}
	}
	return pass(r, "Explicitly selected network devices are present and usable.")
}
func evalNET002(ctx RuleContext, r rule) []model.Finding {
	if !ctx.Profile.GDRRequired {
		return skip(r, "GPUDirect RDMA is not required by the active profile.")
	}
	gpus := selectedGPUGroup(ctx.Snapshot)
	nics, _ := selectedNetwork(ctx.Snapshot)
	if len(gpus) == 0 || len(nics) == 0 {
		return unknown(r, "Selected GPU and NIC pairs are unavailable.")
	}
	for _, gpuRef := range gpus {
		gpu, ok := findGPU(ctx.Snapshot, gpuRef)
		if !ok {
			continue
		}
		for _, nicName := range nics {
			nic, ok := findNIC(ctx.Snapshot, nicName)
			if !ok {
				continue
			}
			if topology.RootForPCI(ctx.Snapshot, gpu.PCIAddress) != topology.RootForPCI(ctx.Snapshot, nic.PCIAddress) {
				return []model.Finding{finding(r, model.StatusWarn, model.SeverityHigh, model.Subject{Kind: "gpu_nic_pair", ID: gpuRef + ":" + nicName}, "Selected GPU and NIC do not share an upstream PCIe root.", "The confirmed GDR pair shares a supported upstream root arrangement.", "GPUDirect RDMA may be unavailable or slower.", "Select a topologically closer NIC/GPU pair and validate with a GDR or NCCL bandwidth test.")}
			}
		}
	}
	return pass(r, "Selected GPUDirect GPU/NIC pairs share an upstream PCIe root.")
}
func evalNET003(ctx RuleContext, r rule) []model.Finding {
	if !ctx.Profile.RDMARequired && len(ctx.Snapshot.RDMA.Devices) == 0 {
		return skip(r, "No RDMA/NCCL InfiniBand context is active.")
	}
	known := false
	for _, container := range ctx.Snapshot.Containers.Devices {
		if container.MemlockSoft != nil {
			known = true
			if *container.MemlockSoft < 1<<30 {
				return []model.Finding{finding(r, model.StatusFail, model.SeverityHigh, model.Subject{Kind: "container", ID: container.ID}, fmt.Sprintf("Container memlock soft limit is %d bytes.", *container.MemlockSoft), "RDMA pinned-memory workflows have an adequate or unlimited memlock limit.", "NCCL or RDMA memory registration may fail.", "Raise memlock according to deployment policy and retest NCCL initialization.")}
			}
		}
	}
	if !known {
		return unknown(r, "Effective memlock could not be established for the RDMA context.")
	}
	return pass(r, "Effective memlock is adequate for the checked RDMA context.")
}
func evalNCCL001(ctx RuleContext, r rule) []model.Finding {
	nics, hcas := selectedNetwork(ctx.Snapshot)
	explicit := false
	for _, runtime := range ctx.Snapshot.Runtimes.Instances {
		if runtime.Details["NCCL_SOCKET_IFNAME"] != "" || runtime.Details["NCCL_IB_HCA"] != "" {
			explicit = true
		}
	}
	if !explicit {
		return skip(r, "No explicit NCCL interface or HCA filter was detected.")
	}
	if len(nics) > 0 && ctx.Snapshot.Network.State != model.StateAvailable {
		return unknown(r, "NCCL selected a network interface, but network inventory is unavailable.")
	}
	if len(hcas) > 0 && ctx.Snapshot.RDMA.State != model.StateAvailable {
		return unknown(r, "NCCL selected an HCA, but RDMA inventory is unavailable.")
	}
	if bad := invalidNetwork(ctx.Snapshot, nics, hcas); len(bad) > 0 || len(nics)+len(hcas) == 0 {
		return []model.Finding{finding(r, model.StatusFail, model.SeverityHigh, model.Subject{Kind: "nccl", ID: "network_filter"}, "The explicit NCCL interface or HCA filter matches no usable device.", "Every explicit NCCL filter resolves to at least one usable device.", "NCCL initialization or inter-node communication can fail.", "Correct or remove the explicit NCCL filter.")}
	}
	return pass(r, "Explicit NCCL filters resolve to usable devices.")
}
func evalCUDA001(ctx RuleContext, r rule) []model.Finding {
	runtimeCUDA := ""
	for _, runtime := range ctx.Snapshot.Runtimes.Instances {
		if runtime.CUDAVersion != "" {
			runtimeCUDA = runtime.CUDAVersion
			break
		}
	}
	if runtimeCUDA == "" {
		return skip(r, "No active runtime CUDA build was identified.")
	}
	if ctx.Snapshot.NVIDIA.DriverVersion == "" {
		return unknown(r, "The active CUDA build is known, but the NVIDIA driver version is unavailable.")
	}
	compatPackage := ctx.Snapshot.NVIDIA.CUDACompatPackage != nil && *ctx.Snapshot.NVIDIA.CUDACompatPackage
	decision, reason := compat.CUDAWithCompat(ctx.Snapshot.NVIDIA.DriverVersion, runtimeCUDA, compatPackage)
	if decision == compat.Unknown {
		return unknown(r, reason)
	}
	if decision == compat.Incompatible {
		return []model.Finding{finding(r, model.StatusFail, model.SeverityHigh, model.Subject{Kind: "cuda", ID: runtimeCUDA}, reason, "The active CUDA runtime has a supported driver compatibility path.", "CUDA initialization or execution can fail.", "Upgrade the driver or use a compatible runtime image or build.")}
	}
	return pass(r, reason)
}
func evalCUDA002(ctx RuleContext, r rule) []model.Finding {
	if ctx.Snapshot.NVIDIA.DriverVersion == "" {
		if ctx.Snapshot.GPUs.State == model.StateNotDetected {
			return skip(r, "No NVIDIA driver is required.")
		}
		return unknown(r, "NVIDIA driver lifecycle cannot be checked without a version.")
	}
	eol, date, known := compat.DriverEOL(ctx.Snapshot.NVIDIA.DriverVersion, ctx.Now)
	if !known {
		return unknown(r, "The driver branch is outside the embedded lifecycle dataset.")
	}
	if eol {
		return []model.Finding{finding(r, model.StatusWarn, model.SeverityMedium, model.Subject{Kind: "driver", ID: ctx.Snapshot.NVIDIA.DriverVersion}, fmt.Sprintf("NVIDIA driver branch reached lifecycle EOL on %s.", date.Format("2006-01-02")), "The installed driver branch is within its support lifecycle.", "Security and compatibility fixes may no longer be delivered.", "Plan migration to a supported data-center driver branch.")}
	}
	return pass(r, "The installed NVIDIA driver branch is within the embedded support lifecycle.")
}
func evalTORCH001(ctx RuleContext, r rule) []model.Finding {
	found := false
	active := false
	for _, runtime := range ctx.Snapshot.Runtimes.Instances {
		if runtime.Kind == "pytorch" || runtime.Kind == "vllm" || runtime.Kind == "sglang" || runtime.CUDAVersion != "" {
			active = true
		}
		if runtime.CUDAAvailable == nil {
			continue
		}
		found = true
		if !*runtime.CUDAAvailable {
			return []model.Finding{finding(r, model.StatusFail, model.SeverityHigh, model.Subject{Kind: "runtime", ID: strconv.Itoa(runtime.PID), Name: runtime.Kind}, "PyTorch reports CUDA unavailable in the active runtime interpreter.", "PyTorch reports CUDA available in the active GPU runtime.", "The workload cannot use its expected GPUs.", "Check the PyTorch build, driver compatibility, and container GPU exposure.")}
		}
	}
	if !found {
		if active {
			return unknown(r, "An active GPU runtime was detected, but its PyTorch CUDA probe is unavailable.")
		}
		return skip(r, "No active runtime PyTorch CUDA probe was available.")
	}
	return pass(r, "PyTorch reports CUDA available in the active runtime.")
}
func evalTORCH002(ctx RuleContext, r rule) []model.Finding { return worldSizeRule(ctx, r, "", true) }
func evalCTR001(ctx RuleContext, r rule) []model.Finding {
	needed := ctx.Profile.DockerRequired || len(ctx.Snapshot.Containers.Devices) > 0
	if !needed {
		return skip(r, "Docker is not required by the active context.")
	}
	switch ctx.Snapshot.Containers.DaemonState {
	case model.StateAvailable:
		return pass(r, "Docker daemon is reachable.")
	case model.StatePermissionDenied:
		return unknown(r, "Docker daemon access was denied.")
	case model.StateNotDetected, model.StateTimeout:
		return []model.Finding{finding(r, model.StatusFail, model.SeverityHigh, model.Subject{Kind: "container_engine", ID: "docker"}, "Docker is required but the daemon is unreachable.", "The required Docker daemon is reachable.", "Container deployment cannot start or be inspected.", "Start Docker or repair its configured context and socket.")}
	default:
		return unknown(r, "Docker daemon state is not known.")
	}
}
func evalCTR002(ctx RuleContext, r rule) []model.Finding {
	needed := ctx.Profile.DockerRequired && ctx.Profile.GPURequired
	for _, container := range ctx.Snapshot.Containers.Devices {
		needed = needed || container.GPURequired
	}
	if !needed {
		return skip(r, "No Docker GPU deployment context is active.")
	}
	if ctx.Snapshot.Containers.State != model.StateAvailable {
		return unknown(r, "Container runtime configuration is unavailable.")
	}
	if !ctx.Snapshot.Containers.NVIDIARuntime || (ctx.Snapshot.Containers.ToolkitDetected != nil && !*ctx.Snapshot.Containers.ToolkitDetected) {
		return []model.Finding{finding(r, model.StatusFail, model.SeverityHigh, model.Subject{Kind: "container_engine", ID: "docker"}, "NVIDIA Container Toolkit is missing or Docker is not configured for GPU access.", "Docker GPU deployment has a configured NVIDIA runtime/toolkit path.", "GPU containers cannot receive NVIDIA devices.", "Install and configure NVIDIA Container Toolkit, then validate with a minimal GPU container.")}
	}
	return pass(r, "NVIDIA Container Toolkit is configured for Docker GPU use.")
}
func evalCTR003(ctx RuleContext, r rule) []model.Finding {
	if ctx.Snapshot.Containers.State != model.StateAvailable {
		if ctx.Profile.DockerRequired && ctx.Profile.GPURequired {
			return unknown(r, "GPU container visibility cannot be evaluated because container inventory is unavailable.")
		}
		return skip(r, "No inspectable container inventory is available.")
	}
	out := []model.Finding{}
	applicable := false
	for _, container := range ctx.Snapshot.Containers.Devices {
		if !container.GPURequired {
			continue
		}
		applicable = true
		if !container.GPUAccess || len(container.GPUUUIDs) == 0 {
			out = append(out, finding(r, model.StatusFail, model.SeverityHigh, model.Subject{Kind: "container", ID: container.ID}, "GPU-required container has no effective GPU visibility.", "The container resolves at least one requested GPU.", "The workload cannot initialize CUDA.", "Correct Docker GPU device requests and effective visibility."))
		}
	}
	if len(out) > 0 {
		return out
	}
	if !applicable {
		return skip(r, "No GPU-required running container was detected.")
	}
	return pass(r, "GPU-required containers have effective GPU visibility.")
}
func evalCTR004(ctx RuleContext, r rule) []model.Finding {
	if !ctx.Profile.MultiProcess {
		return skip(r, "No multi-process NCCL container context is active.")
	}
	known := false
	for _, container := range ctx.Snapshot.Containers.Devices {
		if container.SHMSize == nil {
			continue
		}
		known = true
		if *container.SHMSize <= 64<<20 {
			return []model.Finding{finding(r, model.StatusFail, model.SeverityHigh, model.Subject{Kind: "container", ID: container.ID}, fmt.Sprintf("Container shared memory is %d bytes.", *container.SHMSize), "Shared memory exceeds the Docker default for the multi-process workload.", "NCCL or framework shared-memory initialization can fail.", "Increase shared memory or use the framework-recommended IPC strategy.")}
		}
	}
	if !known {
		return unknown(r, "Container shared-memory size is unavailable.")
	}
	return pass(r, "Container shared memory exceeds the default tiny allocation.")
}
func evalVLLM001(ctx RuleContext, r rule) []model.Finding {
	return worldSizeRule(ctx, r, "vllm", false)
}
func evalVLLM002(ctx RuleContext, r rule) []model.Finding {
	return visibleSelectionRule(ctx, r, "vllm")
}
func evalSGL001(ctx RuleContext, r rule) []model.Finding {
	return worldSizeRule(ctx, r, "sglang", false)
}
func evalSGL002(ctx RuleContext, r rule) []model.Finding {
	found := false
	for _, runtime := range ctx.Snapshot.Runtimes.Instances {
		if !strings.EqualFold(runtime.Kind, "sglang") || !runtime.Disaggregation {
			continue
		}
		found = true
		if len(runtime.SelectedHCAs) == 0 {
			return unknown(r, "SGLang disaggregation is enabled but its HCA mapping was not resolved.")
		}
		if ctx.Snapshot.RDMA.State != model.StateAvailable {
			return unknown(r, "SGLang disaggregation HCA mapping exists, but RDMA inventory is unavailable.")
		}
		bad := invalidNetwork(ctx.Snapshot, nil, runtime.SelectedHCAs)
		if len(bad) > 0 {
			return []model.Finding{finding(r, model.StatusFail, model.SeverityHigh, model.Subject{Kind: "runtime", ID: strconv.Itoa(runtime.PID), Name: runtime.Kind}, "SGLang disaggregation references a missing or unavailable HCA.", "Every disaggregation HCA reference resolves to an active RDMA device.", "Prefill/decode disaggregation initialization can fail.", "Correct the SGLang IB device mapping or restore the RDMA device.")}
		}
	}
	if !found {
		return skip(r, "No SGLang disaggregation context was detected.")
	}
	return pass(r, "SGLang disaggregation HCA references are usable.")
}

func worldSizeRule(ctx RuleContext, r rule, kind string, torch bool) []model.Finding {
	found := false
	active := false
	for _, runtime := range ctx.Snapshot.Runtimes.Instances {
		if kind != "" && !strings.EqualFold(runtime.Kind, kind) {
			continue
		}
		active = true
		if runtime.LocalWorldSize == nil {
			continue
		}
		found = true
		count := runtime.GPUCount
		if !torch && count == nil {
			visible := len(runtime.GPUs)
			count = &visible
		}
		if count == nil {
			return unknown(r, "Runtime GPU visibility is unavailable.")
		}
		if *runtime.LocalWorldSize > *count {
			return []model.Finding{finding(r, model.StatusFail, model.SeverityHigh, model.Subject{Kind: "runtime", ID: strconv.Itoa(runtime.PID), Name: runtime.Kind}, fmt.Sprintf("Runtime local world size %d exceeds effective GPU count %d.", *runtime.LocalWorldSize, *count), "Local GPU requirements do not exceed effective visibility.", "The runtime cannot create all configured workers.", "Reduce local parallelism or restore the intended GPU visibility.")}
		}
	}
	if !found {
		if active {
			return unknown(r, "An applicable runtime was detected, but local world size is unavailable.")
		}
		return skip(r, "No applicable runtime local-world-size context was detected.")
	}
	return pass(r, "Runtime local GPU requirements fit effective visibility.")
}
func visibleSelectionRule(ctx RuleContext, r rule, kind string) []model.Finding {
	found := false
	valid := map[string]bool{}
	for _, gpu := range ctx.Snapshot.GPUs.Devices {
		valid[strconv.Itoa(gpu.Index)] = true
		if gpu.UUID != "" {
			valid[gpu.UUID] = true
		}
	}
	for _, runtime := range ctx.Snapshot.Runtimes.Instances {
		if !strings.EqualFold(runtime.Kind, kind) {
			continue
		}
		raw := strings.Join(runtime.GPUDeviceRefs, ",")
		if raw == "" {
			raw = runtime.Details["CUDA_VISIBLE_DEVICES"]
		}
		if raw == "" {
			raw = runtime.Details["NVIDIA_VISIBLE_DEVICES"]
		}
		if raw == "" {
			continue
		}
		found = true
		normalized := strings.ToLower(strings.TrimSpace(raw))
		if normalized == "all" || normalized == "none" || normalized == "void" || normalized == "-1" {
			continue
		}
		if ctx.Snapshot.GPUs.State != model.StateAvailable {
			return unknown(r, "Runtime GPU selection is explicit, but GPU inventory is unavailable.")
		}
		seen := map[string]bool{}
		for _, ref := range splitList(raw) {
			if seen[ref] || !valid[ref] {
				return []model.Finding{finding(r, model.StatusFail, model.SeverityHigh, model.Subject{Kind: "runtime", ID: strconv.Itoa(runtime.PID), Name: runtime.Kind}, "GPU selection contains a duplicate or nonexistent device reference.", "Every selected GPU reference is unique and resolves to inventory.", "The runtime can expose the wrong number or identity of GPUs.", "Correct the explicit runtime or environment GPU selection.")}
			}
			seen[ref] = true
		}
	}
	if !found {
		return skip(r, "No explicit runtime GPU selection was detected.")
	}
	return pass(r, "Runtime GPU selection references are unique and valid.")
}
func gpuSubject(gpu model.GPU) model.Subject {
	id := gpu.UUID
	if id == "" {
		id = strconv.Itoa(gpu.Index)
	}
	return model.Subject{Kind: "gpu", ID: id, Name: gpu.Name}
}
func findGPU(s *model.Snapshot, ref string) (model.GPU, bool) {
	for _, gpu := range s.GPUs.Devices {
		if gpu.UUID == ref || strconv.Itoa(gpu.Index) == ref {
			return gpu, true
		}
	}
	return model.GPU{}, false
}
func findNIC(s *model.Snapshot, name string) (model.NIC, bool) {
	for _, nic := range s.Network.NICs {
		if nic.Name == name {
			return nic, true
		}
	}
	return model.NIC{}, false
}
func contains(values []int, want int) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
func intersects(a, b []int) bool {
	set := map[int]bool{}
	for _, value := range a {
		set[value] = true
	}
	for _, value := range b {
		if set[value] {
			return true
		}
	}
	return false
}
func numaCPUs(s *model.Snapshot, node int) []int {
	for _, item := range s.NUMA.Nodes {
		if item.ID == node {
			return item.CPUList
		}
	}
	return nil
}
func selectedGPUGroup(s *model.Snapshot) []string {
	for _, runtime := range s.Runtimes.Instances {
		if len(runtime.GPUs) > 1 {
			return append([]string(nil), runtime.GPUs...)
		}
	}
	return nil
}
func gpuIDs(s *model.Snapshot) []string {
	result := []string{}
	for _, gpu := range s.GPUs.Devices {
		id := gpu.UUID
		if id == "" {
			id = strconv.Itoa(gpu.Index)
		}
		result = append(result, id)
	}
	sort.Strings(result)
	return result
}
func pairScores(s *model.Snapshot, group []string) ([]int, bool) {
	scores := []int{}
	for i := 0; i < len(group); i++ {
		for j := i + 1; j < len(group); j++ {
			score, ok := pairScore(s, group[i], group[j])
			if !ok {
				return nil, false
			}
			scores = append(scores, score)
		}
	}
	sort.Ints(scores)
	return scores, true
}
func pairScore(s *model.Snapshot, a, b string) (int, bool) {
	rank := map[string]int{"NV1": 0, "NV2": 0, "NV4": 0, "NV8": 0, "PIX": 1, "PXB": 2, "PHB": 3, "NODE": 4, "SYS": 5, "N/A": 6}
	for _, link := range s.P2P {
		if (link.FromGPU == a && link.ToGPU == b) || (link.FromGPU == b && link.ToGPU == a) {
			if value, ok := rank[strings.ToUpper(link.Kind)]; ok {
				return value, true
			}
			if strings.EqualFold(link.Status, "available") {
				return link.Distance, true
			}
			return 6, true
		}
	}
	return 0, false
}
func dominatingGroup(s *model.Snapshot, all []string, size int, selected []int) ([]string, bool) {
	var result []string
	var walk func(int, []string)
	walk = func(start int, current []string) {
		if result != nil {
			return
		}
		if len(current) == size {
			scores, ok := pairScores(s, current)
			if !ok || len(scores) != len(selected) {
				return
			}
			better := false
			for i := range scores {
				if scores[i] > selected[i] {
					return
				}
				if scores[i] < selected[i] {
					better = true
				}
			}
			if better {
				result = append([]string(nil), current...)
			}
			return
		}
		for i := start; i < len(all); i++ {
			walk(i+1, append(current, all[i]))
		}
	}
	walk(0, nil)
	return result, result != nil
}
func selectedNetwork(s *model.Snapshot) ([]string, []string) {
	nics, hcas := []string{}, []string{}
	for _, runtime := range s.Runtimes.Instances {
		nics = append(nics, runtime.SelectedNICs...)
		hcas = append(hcas, runtime.SelectedHCAs...)
		if raw := runtime.Details["NCCL_SOCKET_IFNAME"]; raw != "" {
			nics = append(nics, splitList(raw)...)
		}
		if raw := runtime.Details["NCCL_IB_HCA"]; raw != "" {
			hcas = append(hcas, splitList(raw)...)
		}
	}
	return unique(nics), unique(hcas)
}
func splitList(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == ';' })
	out := []string{}
	for _, part := range parts {
		part = strings.TrimSpace(strings.TrimLeft(part, "=^"))
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
func unique(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}
func invalidNetwork(s *model.Snapshot, nics, hcas []string) []string {
	bad := []string{}
	for _, want := range nics {
		found := false
		for _, nic := range s.Network.NICs {
			if nic.Name == want && (nic.OperState == "up" || nic.OperState == "unknown") {
				found = true
			}
		}
		if !found {
			bad = append(bad, want)
		}
	}
	for _, want := range hcas {
		found := false
		for _, device := range s.RDMA.Devices {
			if device.Name == want && (device.State == "" || strings.EqualFold(device.State, "active") || strings.EqualFold(device.State, "up")) {
				found = true
			}
		}
		if !found {
			bad = append(bad, want)
		}
	}
	return bad
}
