package nvidia

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/arpingblue/AIStat/internal/collector"
	"github.com/arpingblue/AIStat/internal/collector/numa"
	"github.com/arpingblue/AIStat/internal/execx"
	"github.com/arpingblue/AIStat/internal/model"
)

type Collector struct{}

func (Collector) ID() collector.ID { return "nvidia" }
func (Collector) Provides() []collector.Capability {
	return []collector.Capability{"gpus", "nvidia", "p2p", "nvidia_topology"}
}
func (Collector) Requires() []collector.Capability { return []collector.Capability{"pci"} }

const fields = "index,uuid,name,pci.bus_id,memory.total,memory.used,utilization.gpu,temperature.gpu,power.draw,power.limit,power.default_limit,persistence_mode,compute_mode,mig.mode.current,ecc.errors.uncorrected.volatile.total,ecc.errors.uncorrected.aggregate.total,pcie.link.width.current,pcie.link.width.max,driver_version"

func (c Collector) Collect(ctx context.Context, env collector.Env) collector.Result {
	if env.Platform != "linux" && !env.Fixture {
		return collector.Result{Collector: c.ID(), State: model.StateUnsupported, Facts: []model.Fact{{Key: "gpus", State: model.StateUnsupported, Confidence: model.ConfidenceHigh}, {Key: "nvidia", State: model.StateUnsupported, Confidence: model.ConfidenceHigh}, {Key: "p2p", State: model.StateUnsupported, Confidence: model.ConfidenceHigh}}}
	}
	res, err := env.Runner.Run(ctx, execx.CommandSpec{Name: "nvidia-smi", Args: []string{"--query-gpu=" + fields, "--format=csv,noheader,nounits"}, Timeout: 3 * time.Second, OutputLimit: 2 << 20})
	if err != nil {
		state := commandState(res, err)
		usable := false
		stack := model.NVIDIAStack{State: state, DriverUsable: &usable, XIDState: model.StateUnknown}
		return collector.Result{Collector: c.ID(), State: state, Facts: []model.Fact{{Key: "gpus", State: state, Confidence: model.ConfidenceMedium}, model.NewFact("nvidia", model.StateAvailable, stack, model.ConfidenceMedium), {Key: "p2p", State: state, Confidence: model.ConfidenceLow}}, Diagnostics: []model.Diagnostic{{Level: "warn", Code: "nvidia_smi", Message: err.Error()}}}
	}
	gpus, driver, err := ParseCSV(res.Stdout)
	if err != nil {
		return collector.Result{Collector: c.ID(), State: model.StateParseError, Facts: []model.Fact{{Key: "gpus", State: model.StateParseError, Confidence: model.ConfidenceLow}, {Key: "nvidia", State: model.StateParseError, Confidence: model.ConfidenceLow}, {Key: "p2p", State: model.StateParseError, Confidence: model.ConfidenceLow}}, Diagnostics: []model.Diagnostic{{Level: "warn", Code: "nvidia_parse", Message: err.Error()}}}
	}
	usable := true
	stack := model.NVIDIAStack{State: model.StateAvailable, DriverUsable: &usable, DriverVersion: driver, XIDState: model.StateUnknown}
	stack.CUDAToolkits = readToolkits(env)
	stack.CUDAToolkit = readToolkitAt(env, "/usr/local/cuda")
	if stack.CUDAToolkit == "" && len(stack.CUDAToolkits) > 0 {
		stack.CUDAToolkit = stack.CUDAToolkits[0]
	}
	stack.CUDACompatPackage = detectCompatPackage(env)
	stack.NCCLVersion = readNCCL(env)
	type commandOutput struct {
		result execx.Result
		err    error
	}
	var summary, logResult, topo, apps, bar1 commandOutput
	var wait sync.WaitGroup
	run := func(target *commandOutput, spec execx.CommandSpec) {
		defer wait.Done()
		target.result, target.err = env.Runner.Run(ctx, spec)
	}
	wait.Add(5)
	go run(&summary, execx.CommandSpec{Name: "nvidia-smi", Timeout: 3 * time.Second, OutputLimit: 512 << 10})
	go run(&logResult, execx.CommandSpec{Name: "dmesg", Args: []string{"--time-format", "iso", "--color=never"}, Timeout: 2 * time.Second, OutputLimit: 2 << 20})
	go run(&topo, execx.CommandSpec{Name: "nvidia-smi", Args: []string{"topo", "-m"}, Timeout: 3 * time.Second, OutputLimit: 1 << 20})
	go run(&apps, execx.CommandSpec{Name: "nvidia-smi", Args: []string{"--query-compute-apps=pid,gpu_uuid", "--format=csv,noheader,nounits"}, Timeout: 2 * time.Second, OutputLimit: 1 << 20})
	go run(&bar1, execx.CommandSpec{Name: "nvidia-smi", Args: []string{"--query-gpu=index,bar1_memory.total,bar1_memory.used", "--format=csv,noheader,nounits"}, Timeout: 2 * time.Second, OutputLimit: 512 << 10})
	wait.Wait()
	if summary.err == nil {
		stack.CUDADriver = matchVersion(summary.result.Stdout, `CUDA Version:\s*([0-9.]+)`)
	}
	if logResult.err == nil {
		stack.XIDState = model.StateAvailable
		stack.XIDEvents = ParseXID(logResult.result.Stdout, env.Clock.Now())
	} else {
		stack.XIDState = commandState(logResult.result, logResult.err)
	}
	if apps.err == nil {
		stack.ComputeProcesses = ParseComputeApps(apps.result.Stdout)
		active := map[string]bool{}
		for _, item := range stack.ComputeProcesses {
			active[item.GPUUUID] = true
		}
		for i := range gpus {
			gpus[i].Active = gpus[i].Active || active[gpus[i].UUID]
		}
	}
	if bar1.err == nil {
		applyBAR1(gpus, bar1.result.Stdout)
	}
	var p2p []model.P2PLink
	var connections []model.TopologyConnection
	if topo.err == nil {
		p2p, connections = ParseTopologyMatrix(topo.result.Stdout, gpus)
	}
	gpuState := model.GPUState{State: model.StateAvailable, Devices: gpus}
	facts := []model.Fact{model.NewFact("gpus", model.StateAvailable, gpuState, model.ConfidenceHigh, model.SourceRef{Collector: "nvidia", Source: "nvidia-smi query-gpu"}), model.NewFact("nvidia", model.StateAvailable, stack, model.ConfidenceHigh, model.SourceRef{Collector: "nvidia", Source: "nvidia-smi"})}
	if topo.err == nil {
		facts = append(facts, model.NewFact("p2p", model.StateAvailable, p2p, model.ConfidenceMedium, model.SourceRef{Collector: "nvidia", Source: "nvidia-smi topo -m"}))
		facts = append(facts, model.NewFact("topology", model.StateAvailable, connections, model.ConfidenceMedium, model.SourceRef{Collector: "nvidia", Source: "nvidia-smi topo -m"}))
	} else {
		facts = append(facts, model.Fact{Key: "p2p", State: commandState(topo.result, topo.err), Confidence: model.ConfidenceLow})
		facts = append(facts, model.Fact{Key: "topology", State: commandState(topo.result, topo.err), Confidence: model.ConfidenceLow})
	}
	return collector.Result{Collector: c.ID(), State: model.StateAvailable, Facts: facts}
}

func ParseComputeApps(raw string) []model.GPUProcess {
	reader := csv.NewReader(strings.NewReader(raw))
	reader.TrimLeadingSpace = true
	result := []model.GPUProcess{}
	for {
		row, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil || len(row) < 2 {
			continue
		}
		pid, parseErr := strconv.Atoi(strings.TrimSpace(row[0]))
		uuid := strings.TrimSpace(row[1])
		if parseErr == nil && pid > 0 && uuid != "" && uuid != "[Not Supported]" {
			result = append(result, model.GPUProcess{PID: pid, GPUUUID: uuid})
		}
	}
	return result
}

func ParseCSV(raw string) ([]model.GPU, string, error) {
	reader := csv.NewReader(strings.NewReader(raw))
	reader.TrimLeadingSpace = true
	gpus := []model.GPU{}
	driver := ""
	for {
		row, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, "", err
		}
		if len(row) < 19 {
			continue
		}
		gpu := model.GPU{}
		gpu.Index, _ = strconv.Atoi(trim(row[0]))
		gpu.UUID = trim(row[1])
		gpu.Name = trim(row[2])
		gpu.PCIAddress = normalizeBDF(trim(row[3]))
		gpu.MemoryTotalBytes = mebibytes(row[4])
		gpu.MemoryUsedBytes = mebibytes(row[5])
		gpu.UtilizationPct, _ = number(row[6])
		gpu.Active = gpu.UtilizationPct > 0
		gpu.TemperatureC, _ = number(row[7])
		gpu.PowerDrawW, _ = number(row[8])
		gpu.PowerLimitW, _ = number(row[9])
		gpu.DefaultPowerLimitW = floatPointer(row[10])
		gpu.PersistenceMode = trim(row[11])
		gpu.ComputeMode = trim(row[12])
		gpu.MIGMode = trim(row[13])
		gpu.ECCCurrentErrors = uintPointer(row[14])
		gpu.ECCAggregateErrors = uintPointer(row[15])
		gpu.PCIELinkWidth = intPointer(row[16])
		gpu.PCIEMaxLinkWidth = intPointer(row[17])
		driver = trim(row[18])
		gpus = append(gpus, gpu)
	}
	return gpus, driver, nil
}
func ParseTopology(raw string, gpus []model.GPU) []model.P2PLink {
	links, _ := ParseTopologyMatrix(raw, gpus)
	return links
}

func ParseTopologyMatrix(raw string, gpus []model.GPU) ([]model.P2PLink, []model.TopologyConnection) {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	if len(lines) < 2 {
		return nil, nil
	}
	header := strings.Fields(lines[0])
	labels := []string{}
	for _, field := range header {
		if strings.EqualFold(field, "CPU") || strings.EqualFold(field, "NUMA") {
			break
		}
		labels = append(labels, field)
	}
	links := []model.P2PLink{}
	connections := []model.TopologyConnection{}
	byIndex := map[int]string{}
	for _, gpu := range gpus {
		id := gpu.UUID
		if id == "" {
			id = strconv.Itoa(gpu.Index)
		}
		byIndex[gpu.Index] = id
	}
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) < len(labels)+1 || !strings.HasPrefix(fields[0], "GPU") {
			continue
		}
		fromIndex, _ := strconv.Atoi(strings.TrimPrefix(fields[0], "GPU"))
		from := strconv.Itoa(fromIndex)
		if id, ok := byIndex[fromIndex]; ok {
			from = id
		}
		for column, label := range labels {
			if column+1 >= len(fields) || label == fields[0] {
				continue
			}
			token := fields[column+1]
			if strings.HasPrefix(label, "GPU") {
				toIndex, parseErr := strconv.Atoi(strings.TrimPrefix(label, "GPU"))
				if parseErr != nil || toIndex <= fromIndex {
					continue
				}
				to := strconv.Itoa(toIndex)
				if id, ok := byIndex[toIndex]; ok {
					to = id
				}
				links = append(links, model.P2PLink{FromGPU: from, ToGPU: to, Kind: token, Status: p2pStatus(token), Distance: p2pRank(token)})
				continue
			}
			kind := topologyLabelKind(label)
			connections = append(connections, model.TopologyConnection{FromKind: "gpu", From: from, ToKind: kind, To: label, Path: token, Status: p2pStatus(token), Distance: p2pRank(token)})
		}
		affinityOffset := len(labels) + 1
		if affinityOffset < len(fields) {
			cpus, parseErr := numa.ParseList(fields[affinityOffset])
			if parseErr == nil {
				for i := range gpus {
					if gpus[i].Index == fromIndex {
						gpus[i].CPUAffinity = cpus
						if affinityOffset+1 < len(fields) {
							if node, nodeErr := strconv.Atoi(fields[affinityOffset+1]); nodeErr == nil && node >= 0 && gpus[i].NUMANode == nil {
								gpus[i].NUMANode = model.Int(node)
							}
						}
						break
					}
				}
			}
		}
	}
	return links, connections
}

func topologyLabelKind(label string) string {
	upper := strings.ToUpper(label)
	if strings.HasPrefix(upper, "NIC") || strings.HasPrefix(upper, "MLX") {
		return "nic"
	}
	if strings.HasPrefix(upper, "NVME") {
		return "storage"
	}
	return "device"
}
func commandState(res execx.Result, err error) model.FactState {
	if res.TimedOut {
		return model.StateTimeout
	}
	message := strings.ToLower(err.Error() + " " + res.Stderr)
	if strings.Contains(message, "permission") || strings.Contains(message, "access is denied") {
		return model.StatePermissionDenied
	}
	if strings.Contains(message, "executable file not found") || strings.Contains(message, "cannot find") || strings.Contains(message, "not found") {
		return model.StateNotDetected
	}
	return model.StateUnknown
}
func trim(value string) string { return strings.TrimSpace(value) }
func number(raw string) (float64, bool) {
	value := trim(raw)
	if value == "" || strings.EqualFold(value, "n/a") {
		return 0, false
	}
	parsed, err := strconv.ParseFloat(value, 64)
	return parsed, err == nil
}
func mebibytes(raw string) uint64 {
	value, ok := number(raw)
	if !ok {
		return 0
	}
	return uint64(value * 1024 * 1024)
}
func uintPointer(raw string) *uint64 {
	value, ok := number(raw)
	if !ok {
		return nil
	}
	result := uint64(value)
	return &result
}
func floatPointer(raw string) *float64 {
	value, ok := number(raw)
	if !ok {
		return nil
	}
	return &value
}
func intPointer(raw string) *int {
	value, ok := number(raw)
	if !ok {
		return nil
	}
	result := int(value)
	return &result
}
func normalizeBDF(value string) string {
	value = strings.ToLower(value)
	parts := strings.Split(value, ":")
	if len(parts) == 3 && len(parts[0]) > 4 {
		parts[0] = parts[0][len(parts[0])-4:]
		value = strings.Join(parts, ":")
	}
	if strings.Count(value, ":") == 1 {
		return "0000:" + value
	}
	return value
}
func p2pStatus(token string) string {
	if strings.EqualFold(token, "X") || strings.EqualFold(token, "N/A") {
		return "unavailable"
	}
	upper := strings.ToUpper(token)
	if strings.HasPrefix(upper, "NV") || upper == "PIX" || upper == "PXB" || upper == "PHB" || upper == "NODE" || upper == "SYS" {
		return "available"
	}
	return "unknown"
}
func p2pRank(token string) int {
	rank := map[string]int{"NV1": 0, "NV2": 0, "NV4": 0, "NV8": 0, "PIX": 1, "PXB": 2, "PHB": 3, "NODE": 4, "SYS": 5, "N/A": 6, "X": 6}
	if value, ok := rank[strings.ToUpper(token)]; ok {
		return value
	}
	if strings.HasPrefix(strings.ToUpper(token), "NV") {
		return 0
	}
	return 6
}

func readToolkits(env collector.Env) []string {
	paths := []string{"/usr/local/cuda"}
	if entries, err := env.FileSystem.ReadDir("/usr/local"); err == nil {
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), "cuda-") {
				paths = append(paths, "/usr/local/"+entry.Name())
			}
		}
	}
	seen := map[string]bool{}
	versions := []string{}
	for _, root := range paths {
		version := readToolkitAt(env, root)
		if version != "" && !seen[version] {
			seen[version] = true
			versions = append(versions, version)
		}
	}
	sort.Strings(versions)
	return versions
}

func readToolkitAt(env collector.Env, root string) string {
	if raw, err := env.FileSystem.ReadFile(root + "/version.json"); err == nil {
		var parsed struct {
			CUDA struct {
				Version string `json:"version"`
			} `json:"cuda"`
		}
		if json.Unmarshal(raw, &parsed) == nil && parsed.CUDA.Version != "" {
			return parsed.CUDA.Version
		}
	}
	if raw, err := env.FileSystem.ReadFile(root + "/version.txt"); err == nil {
		return matchVersion(string(raw), `(?i)CUDA Version\s*([0-9.]+)`)
	}
	return ""
}
func detectCompatPackage(env collector.Env) *bool {
	_, err := env.FileSystem.Stat("/usr/local/cuda/compat/libcuda.so.1")
	if err == nil {
		value := true
		return &value
	}
	if errors.Is(err, fs.ErrNotExist) {
		value := false
		return &value
	}
	return nil
}

func applyBAR1(gpus []model.GPU, raw string) {
	reader := csv.NewReader(strings.NewReader(raw))
	reader.TrimLeadingSpace = true
	for {
		row, err := reader.Read()
		if errors.Is(err, io.EOF) {
			return
		}
		if err != nil || len(row) < 3 {
			continue
		}
		index, parseErr := strconv.Atoi(trim(row[0]))
		if parseErr != nil {
			continue
		}
		for i := range gpus {
			if gpus[i].Index == index {
				gpus[i].BAR1TotalBytes = mebibytePointer(row[1])
				gpus[i].BAR1UsedBytes = mebibytePointer(row[2])
				break
			}
		}
	}
}

func mebibytePointer(raw string) *uint64 {
	value, ok := number(raw)
	if !ok {
		return nil
	}
	bytes := uint64(value * 1024 * 1024)
	return &bytes
}
func readNCCL(env collector.Env) string {
	raw, err := env.FileSystem.ReadFile("/usr/include/nccl.h")
	if err != nil {
		return ""
	}
	major := matchVersion(string(raw), `(?m)^\s*#define\s+NCCL_MAJOR\s+([0-9]+)`)
	minor := matchVersion(string(raw), `(?m)^\s*#define\s+NCCL_MINOR\s+([0-9]+)`)
	patch := matchVersion(string(raw), `(?m)^\s*#define\s+NCCL_PATCH\s+([0-9]+)`)
	if major == "" || minor == "" {
		return ""
	}
	if patch == "" {
		patch = "0"
	}
	return major + "." + minor + "." + patch
}
func matchVersion(raw, pattern string) string {
	match := regexp.MustCompile(pattern).FindStringSubmatch(raw)
	if len(match) < 2 {
		return ""
	}
	return match[1]
}
func ParseXID(raw string, fallback time.Time) []model.XIDEvent {
	pattern := regexp.MustCompile(`(?i)NVRM:\s*Xid.*:\s*([0-9]+),?\s*(.*)$`)
	events := []model.XIDEvent{}
	for _, line := range strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n") {
		match := pattern.FindStringSubmatch(line)
		if len(match) < 3 {
			continue
		}
		code, _ := strconv.Atoi(match[1])
		observed := fallback
		if fields := strings.Fields(line); len(fields) > 0 {
			if parsed, err := time.Parse(time.RFC3339Nano, strings.Trim(fields[0], "[]")); err == nil {
				observed = parsed
			}
		}
		events = append(events, model.XIDEvent{Timestamp: observed, Code: code, Message: strings.TrimSpace(match[2])})
	}
	return events
}
