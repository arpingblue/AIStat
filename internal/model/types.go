package model

import (
	"encoding/json"
	"time"
)

type FactState string

const (
	StateAvailable        FactState = "available"
	StateNotDetected      FactState = "not_detected"
	StateUnsupported      FactState = "unsupported"
	StatePermissionDenied FactState = "permission_denied"
	StateTimeout          FactState = "timeout"
	StateParseError       FactState = "parse_error"
	StateUnknown          FactState = "unknown"
)

type Confidence string

const (
	ConfidenceHigh   Confidence = "high"
	ConfidenceMedium Confidence = "medium"
	ConfidenceLow    Confidence = "low"
)

type Status string

const (
	StatusPass    Status = "pass"
	StatusWarn    Status = "warn"
	StatusFail    Status = "fail"
	StatusInfo    Status = "info"
	StatusUnknown Status = "unknown"
	StatusSkip    Status = "skip"
)

type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

type Priority string

const (
	PriorityP0 Priority = "P0"
	PriorityP1 Priority = "P1"
	PriorityP2 Priority = "P2"
	PriorityP3 Priority = "P3"
)

type Dimension string

const (
	DimensionDeployment  Dimension = "deployment"
	DimensionPerformance Dimension = "performance"
)

type SourceRef struct {
	Collector string `json:"collector"`
	Source    string `json:"source"`
	Observed  string `json:"observed,omitempty"`
}

type Fact struct {
	Key        string          `json:"key"`
	State      FactState       `json:"state"`
	Value      json.RawMessage `json:"value,omitempty"`
	Unit       string          `json:"unit,omitempty"`
	Confidence Confidence      `json:"confidence"`
	Sources    []SourceRef     `json:"sources,omitempty"`
}

func NewFact[T any](key string, state FactState, value T, confidence Confidence, sources ...SourceRef) Fact {
	f := Fact{Key: key, State: state, Confidence: confidence, Sources: sources}
	if state == StateAvailable {
		raw, err := json.Marshal(value)
		if err != nil {
			f.State = StateParseError
			f.Confidence = ConfidenceLow
		} else {
			f.Value = raw
		}
	}
	return f
}

func Int(value int) *int { return &value }

type Diagnostic struct {
	Level     string `json:"level"`
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable,omitempty"`
}

type Meta struct {
	SchemaVersion string    `json:"schema_version"`
	ToolVersion   string    `json:"tool_version"`
	CollectedAt   time.Time `json:"collected_at"`
	Hostname      string    `json:"hostname,omitempty"`
	OS            string    `json:"os"`
	Arch          string    `json:"arch"`
	DurationMS    int64     `json:"duration_ms"`
	Profile       string    `json:"profile,omitempty"`
}

type Host struct {
	State          FactState `json:"state"`
	Kernel         string    `json:"kernel,omitempty"`
	Distro         string    `json:"distro,omitempty"`
	Machine        string    `json:"machine,omitempty"`
	UptimeSeconds  float64   `json:"uptime_seconds,omitempty"`
	BIOSVendor     string    `json:"bios_vendor,omitempty"`
	BIOSVersion    string    `json:"bios_version,omitempty"`
	Virtualization string    `json:"virtualization,omitempty"`
}

type LogicalCPU struct {
	ID        int  `json:"id"`
	PackageID int  `json:"package_id"`
	CoreID    int  `json:"core_id"`
	NUMANode  *int `json:"numa_node,omitempty"`
}

type CPU struct {
	State          FactState         `json:"state"`
	Model          string            `json:"model,omitempty"`
	Vendor         string            `json:"vendor,omitempty"`
	Sockets        int               `json:"sockets,omitempty"`
	PhysicalCores  int               `json:"physical_cores,omitempty"`
	LogicalCores   int               `json:"logical_cores,omitempty"`
	ThreadsPerCore int               `json:"threads_per_core,omitempty"`
	Logical        []LogicalCPU      `json:"logical,omitempty"`
	SMT            *bool             `json:"smt,omitempty"`
	Governor       string            `json:"governor,omitempty"`
	FrequencyKHz   uint64            `json:"frequency_khz,omitempty"`
	CacheBytes     map[string]uint64 `json:"cache_bytes,omitempty"`
}

type Memory struct {
	State                FactState `json:"state"`
	TotalBytes           uint64    `json:"total_bytes,omitempty"`
	FreeBytes            uint64    `json:"free_bytes,omitempty"`
	SwapTotalBytes       uint64    `json:"swap_total_bytes,omitempty"`
	SwapFreeBytes        uint64    `json:"swap_free_bytes,omitempty"`
	HugePagesTotal       uint64    `json:"hugepages_total,omitempty"`
	HugePagesFree        uint64    `json:"hugepages_free,omitempty"`
	HugePageSizeBytes    uint64    `json:"hugepage_size_bytes,omitempty"`
	THPMode              string    `json:"thp_mode,omitempty"`
	NUMABalancing        *bool     `json:"numa_balancing,omitempty"`
	MemlockSoftBytes     *uint64   `json:"memlock_soft_bytes,omitempty"`
	MemlockHardBytes     *uint64   `json:"memlock_hard_bytes,omitempty"`
	MemlockSoftUnlimited bool      `json:"memlock_soft_unlimited,omitempty"`
	MemlockHardUnlimited bool      `json:"memlock_hard_unlimited,omitempty"`
}

type NUMANode struct {
	ID          int    `json:"id"`
	CPUList     []int  `json:"cpu_list,omitempty"`
	MemoryBytes uint64 `json:"memory_bytes,omitempty"`
	Distances   []int  `json:"distances,omitempty"`
}

type NUMAState struct {
	State FactState  `json:"state"`
	Nodes []NUMANode `json:"nodes,omitempty"`
}

type PCIDevice struct {
	Address     string  `json:"address"`
	Class       string  `json:"class,omitempty"`
	VendorID    string  `json:"vendor_id,omitempty"`
	DeviceID    string  `json:"device_id,omitempty"`
	NUMANode    *int    `json:"numa_node,omitempty"`
	IOMMUGroup  string  `json:"iommu_group,omitempty"`
	Parent      string  `json:"parent,omitempty"`
	Driver      string  `json:"driver,omitempty"`
	LinkSpeedGT float64 `json:"link_speed_gt,omitempty"`
	LinkWidth   int     `json:"link_width,omitempty"`
	MaxSpeedGT  float64 `json:"max_speed_gt,omitempty"`
	MaxWidth    int     `json:"max_width,omitempty"`
	ACSRedirect *bool   `json:"acs_redirect,omitempty"`
}

type PCIState struct {
	State   FactState   `json:"state"`
	Devices []PCIDevice `json:"devices,omitempty"`
}

type GPU struct {
	Index              int      `json:"index"`
	UUID               string   `json:"uuid,omitempty"`
	Name               string   `json:"name,omitempty"`
	PCIAddress         string   `json:"pci_address,omitempty"`
	NUMANode           *int     `json:"numa_node,omitempty"`
	MemoryTotalBytes   uint64   `json:"memory_total_bytes,omitempty"`
	MemoryUsedBytes    uint64   `json:"memory_used_bytes,omitempty"`
	UtilizationPct     float64  `json:"utilization_pct,omitempty"`
	TemperatureC       float64  `json:"temperature_c,omitempty"`
	PowerDrawW         float64  `json:"power_draw_w,omitempty"`
	PowerLimitW        float64  `json:"power_limit_w,omitempty"`
	DefaultPowerLimitW *float64 `json:"default_power_limit_w,omitempty"`
	PersistenceMode    string   `json:"persistence_mode,omitempty"`
	ComputeMode        string   `json:"compute_mode,omitempty"`
	MIGMode            string   `json:"mig_mode,omitempty"`
	BAR1TotalBytes     *uint64  `json:"bar1_total_bytes,omitempty"`
	BAR1UsedBytes      *uint64  `json:"bar1_used_bytes,omitempty"`
	ECCCurrentErrors   *uint64  `json:"ecc_current_errors,omitempty"`
	ECCAggregateErrors *uint64  `json:"ecc_aggregate_errors,omitempty"`
	Active             bool     `json:"active,omitempty"`
	PCIELinkWidth      *int     `json:"pcie_link_width,omitempty"`
	PCIEMaxLinkWidth   *int     `json:"pcie_max_link_width,omitempty"`
	CPUAffinity        []int    `json:"cpu_affinity,omitempty"`
}

type GPUState struct {
	State   FactState `json:"state"`
	Devices []GPU     `json:"devices,omitempty"`
}

type P2PLink struct {
	FromGPU  string `json:"from_gpu"`
	ToGPU    string `json:"to_gpu"`
	Kind     string `json:"kind,omitempty"`
	Status   string `json:"status,omitempty"`
	Distance int    `json:"distance,omitempty"`
}

type TopologyConnection struct {
	FromKind string `json:"from_kind"`
	From     string `json:"from"`
	ToKind   string `json:"to_kind"`
	To       string `json:"to"`
	Path     string `json:"path"`
	Status   string `json:"status"`
	Distance int    `json:"distance"`
}

type XIDEvent struct {
	Timestamp time.Time `json:"timestamp,omitempty"`
	GPU       string    `json:"gpu,omitempty"`
	Code      int       `json:"code"`
	Message   string    `json:"message,omitempty"`
}

type NIC struct {
	Name       string `json:"name"`
	PCIAddress string `json:"pci_address,omitempty"`
	NUMANode   *int   `json:"numa_node,omitempty"`
	OperState  string `json:"oper_state,omitempty"`
	SpeedMbps  int64  `json:"speed_mbps,omitempty"`
	MTU        int    `json:"mtu,omitempty"`
	Driver     string `json:"driver,omitempty"`
}

type NetworkState struct {
	State FactState `json:"state"`
	NICs  []NIC     `json:"nics,omitempty"`
}

type RDMADevice struct {
	Name       string `json:"name"`
	NetDevice  string `json:"net_device,omitempty"`
	PCIAddress string `json:"pci_address,omitempty"`
	NUMANode   *int   `json:"numa_node,omitempty"`
	LinkLayer  string `json:"link_layer,omitempty"`
	State      string `json:"link_state,omitempty"`
}

type RDMAState struct {
	State   FactState    `json:"state"`
	Devices []RDMADevice `json:"devices,omitempty"`
}

type Mount struct {
	Source     string `json:"source,omitempty"`
	Target     string `json:"target"`
	FSType     string `json:"fs_type,omitempty"`
	TotalBytes uint64 `json:"total_bytes,omitempty"`
	FreeBytes  uint64 `json:"free_bytes,omitempty"`
}

type StorageState struct {
	State   FactState     `json:"state"`
	Mounts  []Mount       `json:"mounts,omitempty"`
	Devices []BlockDevice `json:"devices,omitempty"`
}

type BlockDevice struct {
	Name       string `json:"name"`
	Model      string `json:"model,omitempty"`
	SizeBytes  uint64 `json:"size_bytes,omitempty"`
	PCIAddress string `json:"pci_address,omitempty"`
	NUMANode   *int   `json:"numa_node,omitempty"`
	Rotational *bool  `json:"rotational,omitempty"`
}

type NVIDIAStack struct {
	State             FactState    `json:"state"`
	DriverUsable      *bool        `json:"driver_usable,omitempty"`
	DriverVersion     string       `json:"driver_version,omitempty"`
	CUDADriver        string       `json:"cuda_driver,omitempty"`
	CUDAToolkit       string       `json:"cuda_toolkit,omitempty"`
	CUDAToolkits      []string     `json:"cuda_toolkits,omitempty"`
	CUDACompatPackage *bool        `json:"cuda_compat_package,omitempty"`
	NCCLVersion       string       `json:"nccl_version,omitempty"`
	NVLinkState       string       `json:"nvlink_state,omitempty"`
	XIDEvents         []XIDEvent   `json:"xid_events,omitempty"`
	XIDState          FactState    `json:"xid_state"`
	ComputeProcesses  []GPUProcess `json:"compute_processes,omitempty"`
}

type GPUProcess struct {
	PID     int    `json:"pid"`
	GPUUUID string `json:"gpu_uuid"`
}

type Container struct {
	ID            string   `json:"id"`
	InitPID       int      `json:"init_pid,omitempty"`
	Name          string   `json:"name,omitempty"`
	Image         string   `json:"image,omitempty"`
	Runtime       string   `json:"runtime,omitempty"`
	GPURequired   bool     `json:"gpu_required,omitempty"`
	GPUAccess     bool     `json:"gpu_access,omitempty"`
	GPUUUIDs      []string `json:"gpu_uuids,omitempty"`
	EffectiveCPUs []int    `json:"effective_cpus,omitempty"`
	EffectiveMems []int    `json:"effective_mems,omitempty"`
	MemoryLimit   *uint64  `json:"memory_limit_bytes,omitempty"`
	SHMSize       *uint64  `json:"shm_size_bytes,omitempty"`
	MemlockSoft   *uint64  `json:"memlock_soft_bytes,omitempty"`
	MemlockHard   *uint64  `json:"memlock_hard_bytes,omitempty"`
}

type ContainerState struct {
	State               FactState   `json:"state"`
	ClientState         FactState   `json:"client_state,omitempty"`
	DaemonState         FactState   `json:"daemon_state"`
	Engine              string      `json:"engine,omitempty"`
	ClientVersion       string      `json:"client_version,omitempty"`
	EngineVersion       string      `json:"engine_version,omitempty"`
	DefaultRuntime      string      `json:"default_runtime,omitempty"`
	NVIDIARuntime       bool        `json:"nvidia_runtime,omitempty"`
	NVIDIARuntimeState  FactState   `json:"nvidia_runtime_state,omitempty"`
	ToolkitDetected     *bool       `json:"toolkit_detected,omitempty"`
	ToolkitState        FactState   `json:"toolkit_state,omitempty"`
	ToolkitVersion      string      `json:"toolkit_version,omitempty"`
	ToolkitPackageState FactState   `json:"toolkit_package_state,omitempty"`
	ToolkitPackages     []string    `json:"toolkit_packages,omitempty"`
	ToolkitCLIState     FactState   `json:"toolkit_cli_state,omitempty"`
	CDIState            FactState   `json:"cdi_state,omitempty"`
	CDISpecs            []string    `json:"cdi_specs,omitempty"`
	GPUContainerState   FactState   `json:"gpu_container_state,omitempty"`
	GPUContainerModes   []string    `json:"gpu_container_modes,omitempty"`
	ToolkitEvidence     []string    `json:"toolkit_evidence,omitempty"`
	CgroupVersion       string      `json:"cgroup_version,omitempty"`
	Rootless            *bool       `json:"rootless,omitempty"`
	Devices             []Container `json:"containers,omitempty"`
}

type Process struct {
	PID         int               `json:"pid"`
	RuntimeKind string            `json:"runtime_kind,omitempty"`
	Executable  string            `json:"executable,omitempty"`
	Command     string            `json:"command,omitempty"`
	User        string            `json:"user,omitempty"`
	GPUUUIDs    []string          `json:"gpu_uuids,omitempty"`
	ContainerID string            `json:"container_id,omitempty"`
	CPUSet      []int             `json:"cpu_set,omitempty"`
	NUMAMems    []int             `json:"numa_mems,omitempty"`
	AllowedArgs map[string]string `json:"args,omitempty"`
	AllowedEnv  map[string]string `json:"env,omitempty"`
}

type ProcessState struct {
	State     FactState `json:"state"`
	Processes []Process `json:"processes,omitempty"`
}

type RuntimeInstance struct {
	Kind               string            `json:"kind"`
	Version            string            `json:"version,omitempty"`
	PyTorchVersion     string            `json:"pytorch_version,omitempty"`
	PythonVersion      string            `json:"python_version,omitempty"`
	CUDAAvailable      *bool             `json:"cuda_available,omitempty"`
	CUDAVersion        string            `json:"cuda_version,omitempty"`
	GPUCount           *int              `json:"gpu_count,omitempty"`
	DistributedReady   *bool             `json:"distributed_ready,omitempty"`
	PID                int               `json:"pid,omitempty"`
	ContainerID        string            `json:"container_id,omitempty"`
	Executable         string            `json:"executable,omitempty"`
	PythonEnvironment  string            `json:"python_environment,omitempty"`
	GPUs               []string          `json:"gpus,omitempty"`
	CPUSet             []int             `json:"cpu_set,omitempty"`
	NUMAMems           []int             `json:"numa_mems,omitempty"`
	TensorParallel     *int              `json:"tensor_parallel,omitempty"`
	PipelineParallel   *int              `json:"pipeline_parallel,omitempty"`
	DataParallel       *int              `json:"data_parallel,omitempty"`
	LocalWorldSize     *int              `json:"local_world_size,omitempty"`
	NNodes             *int              `json:"nnodes,omitempty"`
	NodeRank           *int              `json:"node_rank,omitempty"`
	DistributedBackend string            `json:"distributed_backend,omitempty"`
	GPUDeviceRefs      []string          `json:"gpu_device_refs,omitempty"`
	NUMABindCPUs       []int             `json:"numa_bind_cpus,omitempty"`
	ModelPath          string            `json:"model_path,omitempty"`
	SelectedNICs       []string          `json:"selected_nics,omitempty"`
	SelectedHCAs       []string          `json:"selected_hcas,omitempty"`
	Disaggregation     bool              `json:"disaggregation,omitempty"`
	Details            map[string]string `json:"details,omitempty"`
}

type RuntimeInstallation struct {
	Product           string     `json:"product"`
	Version           string     `json:"version,omitempty"`
	Path              string     `json:"path"`
	PythonEnvironment string     `json:"python_environment,omitempty"`
	Scope             string     `json:"scope"`
	ContainerID       string     `json:"container_id,omitempty"`
	Source            string     `json:"source"`
	Confidence        Confidence `json:"confidence"`
}

type RuntimeProduct struct {
	Name               string                `json:"name"`
	InstallationState  FactState             `json:"installation_state"`
	InstallationReason string                `json:"installation_reason,omitempty"`
	ExecutionState     FactState             `json:"execution_state"`
	ExecutionReason    string                `json:"execution_reason,omitempty"`
	HostState          FactState             `json:"host_state"`
	ContainerState     FactState             `json:"container_state"`
	InstanceCount      int                   `json:"instance_count"`
	Installations      []RuntimeInstallation `json:"installations,omitempty"`
}

type RuntimeState struct {
	State     FactState         `json:"state"`
	Instances []RuntimeInstance `json:"instances,omitempty"`
	Products  []RuntimeProduct  `json:"products,omitempty"`
}

type CollectorStatus struct {
	ID         string    `json:"id"`
	State      FactState `json:"state"`
	DurationMS int64     `json:"duration_ms"`
	Error      string    `json:"error,omitempty"`
}

type Snapshot struct {
	Meta       Meta                 `json:"meta"`
	Host       Host                 `json:"host"`
	CPU        CPU                  `json:"cpu"`
	Memory     Memory               `json:"memory"`
	NUMA       NUMAState            `json:"numa"`
	PCI        PCIState             `json:"pci"`
	GPUs       GPUState             `json:"gpus"`
	P2P        []P2PLink            `json:"p2p,omitempty"`
	Topology   []TopologyConnection `json:"topology,omitempty"`
	Network    NetworkState         `json:"network"`
	RDMA       RDMAState            `json:"rdma"`
	Storage    StorageState         `json:"storage"`
	NVIDIA     NVIDIAStack          `json:"nvidia"`
	Containers ContainerState       `json:"containers"`
	Processes  ProcessState         `json:"processes"`
	Runtimes   RuntimeState         `json:"runtimes"`
	Collectors []CollectorStatus    `json:"collectors,omitempty"`
}

type Subject struct {
	Kind string `json:"kind"`
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type Evidence struct {
	Fact   string `json:"fact"`
	Value  any    `json:"value,omitempty"`
	Source string `json:"source,omitempty"`
}

type Reference struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

type Finding struct {
	RuleID         string      `json:"rule_id"`
	Title          string      `json:"title"`
	Domain         string      `json:"domain"`
	Status         Status      `json:"status"`
	Severity       Severity    `json:"severity"`
	Dimension      Dimension   `json:"dimension"`
	Priority       Priority    `json:"priority"`
	Subject        Subject     `json:"subject"`
	CurrentState   string      `json:"current_state,omitempty"`
	ExpectedState  string      `json:"expected_state,omitempty"`
	Impact         string      `json:"impact,omitempty"`
	Why            string      `json:"why"`
	Recommendation string      `json:"recommendation,omitempty"`
	Verification   []string    `json:"verification,omitempty"`
	Confidence     Confidence  `json:"confidence"`
	References     []Reference `json:"references,omitempty"`
	Evidence       []Evidence  `json:"evidence,omitempty"`
}

type Readiness struct {
	Deployment  string `json:"deployment"`
	Performance string `json:"performance"`
}

type Summary struct {
	Pass    int `json:"pass"`
	Warn    int `json:"warn"`
	Fail    int `json:"fail"`
	Info    int `json:"info"`
	Unknown int `json:"unknown"`
	Skip    int `json:"skip"`
}

type Profile struct {
	Name           string `json:"name"`
	GPURequired    bool   `json:"gpu_required"`
	DockerRequired bool   `json:"docker_required"`
	GDRRequired    bool   `json:"gdr_required"`
	RDMARequired   bool   `json:"rdma_required"`
	MultiProcess   bool   `json:"multi_process"`
}

type Report struct {
	SchemaVersion        string    `json:"schema_version"`
	AIStatVersion        string    `json:"aistat_version"`
	CollectedAt          time.Time `json:"collected_at"`
	Profile              string    `json:"profile"`
	CompatibilityVersion string    `json:"compatibility_version"`
	Readiness            Readiness `json:"readiness"`
	Summary              Summary   `json:"summary"`
	Node                 *Snapshot `json:"node"`
	Findings             []Finding `json:"findings"`
}
