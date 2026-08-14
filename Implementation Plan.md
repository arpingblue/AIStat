# AIStat v0.1 Implementation Plan / Engineering Playbook

## Executive Summary

**AIStat** 是一个面向 AI Infra 工程师的轻量级 NVIDIA AI Compute Node 全栈检查工具：**Go 单二进制、Linux + NVIDIA、默认只读、CLI-first、无需 daemon、无需联网**。

v0.1 的工程目标不是做“更多命令的包装器”，而是建立一条稳定的数据管线：**Collect Facts → Normalize → Build Topology → Detect Runtime Context → Evaluate Rules → Produce Explainable Findings**。Linux 原生 `/proc`、`/sys`、cgroup 应作为首选数据源；`nvidia-smi`、Docker、PyTorch、vLLM、SGLang 等官方接口用于补足领域信息。Linux cgroup v2 官方文档定义了 `cpuset.cpus`、`cpuset.mems` 等资源约束；NVIDIA `nvidia-smi topo` 已能提供 GPU、NIC、NVMe、CPU/MEM affinity 和 NUMA 等拓扑信息，因此这些都可以成为 AIStat 的事实来源，而不是自行猜测。citeturn3search1turn6view2

v0.1 的真正产品资产是 **Normalized Node Model + Topology Graph + Ruleset**。长期产品闭环固定为 **Check → Diagnose → Monitor → Benchmark → Optimize → Verify**，所有阶段都复用同一数据模型，避免未来推倒重来；当前v0.1只实现Check与Diagnose的只读基础。

**冻结的产品定义：**

> **AIStat is a lightweight, read-only AI infrastructure inspection and performance-readiness tool for Linux NVIDIA compute nodes.**

> **Know the node before you tune it.**

### 文档权威关系与执行门

本文件是《AIStat项目计划书》的工程落地版本。两份文档发生冲突时，按以下优先级处理：

```text
AIStat项目计划书（产品范围、规则语义、验收目标）
        ↓
Implementation Plan（接口、任务拆分、测试与交付顺序）
        ↓
AGENTS.md（编码代理不可破坏约束）
        ↓
代码与测试
```

实施前必须满足以下执行门；未满足时只允许继续修订文档，不开始编码：

```text
[ ] 项目所有者明确说“执行”
[ ] 确认 Git module path / GitHub organization and repository
[ ] 确认 License（建议 Apache-2.0，但不替项目所有者决定）
[ ] 冻结 JSON schema 0.1 的命名、状态值和兼容策略
[ ] 冻结本文定义的 25 条 Rule ID 与语义
[ ] 明确首轮真实 Linux/NVIDIA 验证资源与负责人
```

本计划中的 `github.com/ORG/aistat` 只能作为文档占位符。创建 `go.mod` 前必须替换，禁止把 `ORG` 占位符提交为正式模块路径。

本文现存的 `cite…` 仅是调研过程留下的引用标记，不是可发布链接。M0 建立 `docs/REFERENCES.md`，为每个 `REF-*` 记录官方 URL、标题、访问日期和适用 Rule；README、Finding 和正式文档不得输出内部引用标记。

## 工程边界、里程碑与执行原则

### 冻结范围

v0.1 只支持：

```text
Linux
+
NVIDIA GPU
+
NVIDIA Driver / CUDA / NCCL
+
Docker / NVIDIA Container Toolkit
+
PyTorch
+
vLLM
+
SGLang
+
Node-level CPU / NUMA / Memory / PCIe / NIC / RDMA / Storage
```

v0.1 明确不实现：

```text
daemon
long-running monitoring
Prometheus exporter
Grafana
eBPF profiler
GPU kernel profiling
automatic sysctl modification
automatic CPU pinning
automatic GPU clock/power tuning
Docker config mutation
vLLM/SGLang config mutation
NCCL benchmark
nvbandwidth execution
cluster-level inventory
Kubernetes
Slurm
AMD / Intel GPU / Ascend
```

这不是永久拒绝，而是 **v0.1 scope fence**。

NVIDIA 官方 `nvidia-smi` 已公开拓扑、P2P、CPU/MEM affinity、NUMA、GPU/NIC/NVMe 等查询能力，因此 v0.1 不需要自行重写 NVIDIA topology discovery；NVIDIA Container Toolkit 则是 GPU 容器运行链的重要官方组件。citeturn6view2turn0search8turn6view3

### 总体数据流

```text
/proc /sys /cgroup
      +
nvidia-smi
      +
docker
      +
Python best-effort probe
      +
runtime processes
        │
        ▼
┌───────────────────┐
│    Collectors     │
└────────┬──────────┘
         ▼
┌───────────────────┐
│    Raw Facts      │
└────────┬──────────┘
         ▼
┌───────────────────┐
│   Normalizer      │
└────────┬──────────┘
         ▼
┌───────────────────┐
│ Normalized Model  │
└────────┬──────────┘
         ▼
┌───────────────────┐
│ Base Hardware Graph│
└────────┬──────────┘
         ▼
┌───────────────────┐
│ Runtime Resolver  │
└────────┬──────────┘
         ▼
┌───────────────────┐
│ Final Snapshot +  │
│ Enriched Graph    │
└────────┬──────────┘
         ▼
┌───────────────────┐
│   Rule Engine     │
└────────┬──────────┘
         ▼
┌───────────────────┐
│    Findings       │
└────────┬──────────┘
         ▼
 text / json report
```

这是两阶段建图，不是循环依赖：第一阶段只从 host/PCI/NUMA/GPU/NIC facts 构建 hardware graph；Runtime Resolver 使用已采集并脱敏的 process/container facts 加上 hardware graph 完成 PID→Container→CPU/NUMA/GPU 映射；第二阶段只添加 runtime/container edges，得到供 Rules 使用的最终 Snapshot + Graph。两次建图均不执行 I/O。

核心约束：

```yaml
project:
  name: AIStat
  version_target: v0.1.0
  module_path: 未指定
  github_org: 未指定
  language: Go
  minimum_go: "1.26"
  ci_toolchain: "Go 1.26.5"
  platforms:
    - linux/amd64
    - linux/arm64

runtime:
  daemon: false
  network_required: false
  root_required: false
  mutation_allowed: false
  cgo_required: false

product:
  primary_profile: llm-inference
  secondary_profile: general
  default_output: human
  machine_output: json

architecture_rules:
  - collectors_do_not_judge
  - rules_do_not_collect
  - reporters_do_not_recalculate
  - raw_secrets_are_never_serialized
  - no_shell_interpolation
  - every_external_command_has_timeout
  - missing_optional_tool_is_not_internal_error
  - unknown_is_never_implicitly_pass
  - json_contract_is_lowercase_and_versioned
  - linux_sources_are_injected_for_fixture_tests
```

截至 2026 年 8 月，Go 官方当前版本线已到 Go 1.26；GoReleaser 支持通过 `GOOS`/`GOARCH` 生成构建矩阵，并可以设置 `CGO_ENABLED=0`，适合 AIStat 的 Linux amd64/arm64 单二进制策略。citeturn4search23turn6view6

### 里程碑

| Milestone | 优先级 | 核心成果 | Exit Criteria |
|---|---|---|---|
| M0 Foundation | P0 | 仓库、CLI、模型、exec wrapper、CI | `aistat version`、`aistat check --format json` 可运行 |
| M1 Host Inventory | P0 | system/cpu/memory/numa/pci/network/storage | CPU-only Linux 主机可生成完整 Snapshot |
| M2 NVIDIA | P0 | GPU inventory + GPU topology | CPU↔NUMA↔PCIe↔GPU 建图 |
| M3 AI Software Stack | P0 | Driver/CUDA/NCCL/Docker/NCT/PyTorch | `aistat stack` 可输出版本链 |
| M4 Runtime Context | P0 | process/Docker/vLLM/SGLang | 能回答“哪个 runtime 使用哪些 GPU/CPU” |
| M5 Graph & Normalization | P0 | 稳定 Graph API | Rules 不读取 `/proc` `/sys` |
| M6 Rule Engine | P0 | 首批约 25 Rules | `aistat check` 产生可解释 findings |
| M7 Release Hardening | P0 | fixtures/golden/security/release | tag `v0.1.0` 可自动产生 release |

**依赖关系：**

```text
M0
 │
 ├── M1 ─┐
 ├── M2 ─┼── M5 ── M6 ── M7
 ├── M3 ─┤
 └── M4 ─┘
```

### 各里程碑详细任务

**M0 — Foundation**

输入：空仓库。

输出：

```text
Go module
CLI dispatcher
version metadata
model package
collector interface
safe command runner
JSON output
test skeleton
CI skeleton
AGENTS.md
```

必须完成：

```bash
go test ./...
go vet ./...
go build ./cmd/aistat
./aistat version
./aistat check --format json
```

前 3 项可在 Windows 本地执行；`./aistat ...` 是 Linux CI smoke gate。Windows 使用本机测试 binary 验证 CLI dispatcher，同时只 cross-build、不运行正式 Linux artifact。

其中外部命令执行统一使用 Go `os/exec.CommandContext`；Go 官方文档明确说明 `CommandContext` 会在 context 结束时中断子进程，而 `os/exec` 默认不会像 shell 那样解释 glob、pipeline、redirection，因此适合建立无 shell interpolation 的安全 command runner。citeturn3search2turn3search30

**M1 — Host Inventory**

实现：

```text
system
cpu
memory
numa
pci
network
storage
```

输入：

```text
/proc/*
/sys/*
/etc/os-release
cgroup filesystem
optional: lspci/ip/ethtool/rdma
```

输出：

```text
[]Fact
→ Normalized Snapshot
```

验收：

```bash
aistat info
aistat info --format json
```

在没有 NVIDIA GPU、没有 Docker 的普通 Linux VM 上必须正常结束，不 panic。

**M2 — NVIDIA**

实现：

```text
NVIDIA PCI discovery
driver availability
GPU inventory
GPU UUID/BDF mapping
NUMA affinity
GPU-GPU topology
GPU P2P matrix
GPU-NIC topology
NVLink/NVSwitch-related topology
PCIe current/max width/speed
basic ECC/compute mode/power snapshot
recent normalized Xid events best effort
```

优先来源：

```text
/sys/bus/pci
nvidia-smi --query-gpu ...
nvidia-smi topo -gpu
nvidia-smi topo -nic
nvidia-smi topo -cpu
nvidia-smi topo -all
nvidia-smi topo -p2p (or supported equivalent)
journal/dmesg best effort for normalized Xid events
```

NVIDIA 官方当前手册定义了 `SYS/NODE/PHB/PXB/PIX/NV#` 的连接语义，并支持 GPU-GPU、GPU-NIC、GPU-NVMe、CPU/MEM affinity 和 NUMA 查询。citeturn6view2

验收：

```bash
aistat topology
```

至少能够生成：

```text
NUMA0
├── CPU 0-31
├── GPU0
├── GPU1
└── mlx5_0

NUMA1
├── CPU 32-63
├── GPU2
├── GPU3
└── mlx5_1
```

**M3 — AI Software Stack**

实现：

```text
NVIDIA driver
driver-supported CUDA capability
installed CUDA toolkit(s)
active CUDA runtime/framework build
versioned embedded compatibility data
NCCL presence/version best effort
Docker
NVIDIA Container Toolkit
PyTorch
optional tools:
  DCGM
  nccl-tests
  nvbandwidth
```

必须严格区分：

```text
Driver Version
Driver-supported CUDA Version
Installed CUDA Toolkit Version
PyTorch CUDA Build Version
```

不能把 `nvidia-smi` 顶部显示的 CUDA version 直接当作“已安装 CUDA Toolkit”。CUDA 官方兼容文档明确区分 driver/runtime/toolkit compatibility，并定义 minor-version compatibility 与 forward compatibility。citeturn0search1turn0search4

验收：

```bash
aistat stack
```

**M4 — Runtime Context**

实现：

```text
/proc PID discovery
CPU affinity
cgroup identity
container identity
allowlisted environment
vLLM detection
SGLang detection
recognized launch flags
selected GPU IDs
effective CPU and memory-node sets
TP / PP / DP
```

vLLM 当前 CLI 已包含 TP、PP、DP、device IDs 和 NUMA CPU binding 等基础设施相关参数，所以这些字段值得成为 AIStat runtime model 的正式字段，而不是仅保存完整命令字符串。citeturn6view1

SGLang 官方 server arguments 同样将 parallelism、memory management 和 deployment performance 参数作为正式服务器配置。citeturn6view0

**M5 — Normalization & Topology**

目标：

> M5 之后 Rules 禁止直接访问 filesystem 或执行命令。

Rules 只能得到：

```go
RuleContext{
    Snapshot,
    Graph,
    Profile,
    Now,
}
```

这是最重要的架构边界之一。

**M6 — Rules**

实现：

```text
rule registry
rule metadata
evaluation
deployment/performance dimension
priority
severity
confidence
references
recommendation
verification
finding serialization
deterministic readiness aggregation
```

输出：

```text
PASS
WARN
FAIL
INFO
UNKNOWN
SKIP
```

但默认 human report 只展开：

```text
FAIL
WARN
important INFO
```

**M7 — Release**

完成：

```text
fixture suite
golden suite
redaction tests
timeout tests
CPU-only tests
cross compile
GoReleaser
install.sh
checksums
README
SECURITY.md
README contribution section
AGENTS.md
v0.1.0 release
```

GitHub 官方建议 Go 项目使用 `setup-go` 保持 CI 工具链一致；GoReleaser 当前官方 GitHub Actions 示例使用 GoReleaser Action v7，并要求发布时 checkout 完整 Git history。citeturn6view7turn9search0

## 接口、数据模型与项目骨架

### 仓库结构

```text
aistat/
├── cmd/
│   └── aistat/
│       └── main.go
│
├── internal/
│   ├── app/
│   │   ├── app.go
│   │   ├── check.go
│   │   ├── info.go
│   │   ├── topology.go
│   │   ├── stack.go
│   │   ├── runtime.go
│   │   └── version.go
│   │
│   ├── collector/
│   │   ├── collector.go
│   │   ├── registry.go
│   │   ├── system/
│   │   ├── cpu/
│   │   ├── memory/
│   │   ├── numa/
│   │   ├── pci/
│   │   ├── nvidia/
│   │   ├── docker/
│   │   ├── pytorch/
│   │   ├── vllm/
│   │   ├── sglang/
│   │   ├── storage/
│   │   └── network/
│   │
│   ├── runtime/
│   │   ├── process.go
│   │   ├── environ.go
│   │   └── affinity.go
│   │
│   ├── model/
│   │   ├── types.go
│   │   ├── fact.go
│   │   ├── snapshot.go
│   │   ├── finding.go
│   │   └── profile.go
│   │
│   ├── normalize/
│   │   └── normalize.go
│   │
│   ├── topology/
│   │   ├── graph.go
│   │   ├── distance.go
│   │   └── builder.go
│   │
│   ├── rules/
│   │   ├── rule.go
│   │   ├── engine.go
│   │   ├── registry.go
│   │   ├── gpu/
│   │   ├── pcie/
│   │   ├── numa/
│   │   ├── network/
│   │   ├── container/
│   │   ├── cuda/
│   │   ├── nccl/
│   │   ├── pytorch/
│   │   ├── vllm/
│   │   └── sglang/
│   │
│   ├── report/
│   │   ├── report.go
│   │   ├── human.go
│   │   └── json.go
│   │
│   ├── execx/
│   │   ├── runner.go
│   │   └── resolver.go
│   │
│   ├── fsx/
│   │   ├── filesystem.go
│   │   └── rooted.go
│   │
│   ├── clock/
│   │   └── clock.go
│   │
│   ├── compat/
│   │   ├── cuda.go
│   │   └── lifecycle.go
│   │
│   ├── redact/
│   │   └── redact.go
│   │
│   └── version/
│       └── version.go
│
├── testdata/
│   ├── fixtures/
│   │   ├── linux/
│   │   ├── nvidia/
│   │   ├── docker/
│   │   ├── pytorch/
│   │   ├── vllm/
│   │   └── sglang/
│   ├── snapshots/
│   └── golden/
│
├── data/
│   └── compatibility/
│       ├── cuda.json
│       └── nvidia-driver-lifecycle.json
│
├── docs/
│   ├── architecture.md
│   ├── data-model.md
│   ├── collectors.md
│   ├── topology.md
│   ├── rules.md
│   ├── security.md
│   ├── contributing-rules.md
│   ├── REFERENCES.md
│   └── schema/
│       └── report-v0.1.schema.json
│
├── scripts/
│   ├── install.sh
│   ├── sanitize-fixture.sh
│   └── dev.ps1
│
├── .github/
│   └── workflows/
│       ├── ci.yml
│       └── release.yml
│
├── AGENTS.md
├── README.md (includes contribution guide)
├── SECURITY.md
├── LICENSE
├── Makefile
├── go.mod
├── go.sum
├── .goreleaser.yaml
└── README.md
```

### Collector Contract

Collector 必须通过注入环境访问文件、时间与外部命令，不能把真实 `/proc`、`/sys` 或本机时钟写死在解析逻辑中。该约束既用于 Linux fixture test，也使 Windows 开发机能够测试绝大多数业务逻辑。

冻结接口：

```go
package collector

import (
	"context"

	"github.com/ORG/aistat/internal/clock"
	"github.com/ORG/aistat/internal/execx"
	"github.com/ORG/aistat/internal/fsx"
	"github.com/ORG/aistat/internal/model"
)

type ID string
type Capability string

type Collector interface {
	ID() ID
	Provides() []Capability
	Requires() []Capability
	Collect(ctx context.Context, env Env) Result
}

type Env struct {
	Runner     execx.Runner
	FileSystem fsx.FileSystem
	Clock      clock.Clock
}

type Result struct {
	Collector   ID                 `json:"collector"`
	Facts       []model.Fact       `json:"facts"`
	Diagnostics []model.Diagnostic `json:"diagnostics,omitempty"`
}
```

约束：

```text
Collector:
  YES read facts
  YES parse facts
  YES report collection errors
  NO emit WARN/FAIL
  NO make recommendation
  NO modify host
  NO depend on the developer host filesystem in tests
```

`RunAll` 执行 capability DAG：无依赖 Collector 可以有界并发；有依赖的 runtime/normalization 阶段等待所需 capability。输出必须按 Collector ID 稳定排序。单个 Collector 失败或超时只影响自身 Result，不取消其他独立 Collector；只有全局 context 取消才停止整次采集。

### Fact Model

```go
package model

import (
	"encoding/json"
	"time"
)

type Confidence string

const (
	ConfidenceHigh   Confidence = "high"
	ConfidenceMedium Confidence = "medium"
	ConfidenceLow    Confidence = "low"
)

type FactState string

const (
	FactAvailable        FactState = "available"
	FactNotDetected      FactState = "not_detected"
	FactUnsupported      FactState = "unsupported"
	FactPermissionDenied FactState = "permission_denied"
	FactTimeout          FactState = "timeout"
	FactParseError       FactState = "parse_error"
	FactUnknown          FactState = "unknown"
)

type Subject struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

type SourceRef struct {
	Kind    string `json:"kind"`              // sysfs, procfs, command, cgroup...
	Locator string `json:"locator,omitempty"` // path or command name
	Field   string `json:"field,omitempty"`
}

type Fact struct {
	Key        string          `json:"key"`
	Domain     string          `json:"domain"`
	Subject    Subject         `json:"subject"`
	Field      string          `json:"field"`
	Value      json.RawMessage `json:"value,omitempty"`
	Unit       string          `json:"unit,omitempty"`
	State      FactState       `json:"state"`
	Confidence Confidence      `json:"confidence"`
	Source     SourceRef       `json:"source"`
	CollectedAt time.Time      `json:"collected_at"`
	Reason      string         `json:"reason,omitempty"`
}

type Diagnostic struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable,omitempty"`
}
```

状态语义冻结如下：

```text
available         成功取得值
not_detected      已成功检查，目标不存在
unsupported       当前平台、内核或工具明确不支持
permission_denied 目标可能存在，但权限不足
timeout           有界采集超时
parse_error       已得到输入，但无法可靠解析
unknown           其他证据不足情形
```

禁止把 `permission_denied`、`timeout`、`parse_error` 折叠为普通 `error`；Rule 必须据此输出 UNKNOWN，而不是误判 PASS 或 FAIL。

示例 Collector 输出：

```json
{
  "collector": "nvidia",
  "facts": [
    {
      "key": "gpu.GPU-abc.pci_bdf",
      "domain": "gpu",
      "subject": {
        "kind": "gpu",
        "id": "GPU-abc"
      },
      "field": "pci_bdf",
      "value": "0000:31:00.0",
      "state": "available",
      "confidence": "high",
      "source": {
        "kind": "nvidia-smi",
        "locator": "nvidia-smi"
      }
    }
  ]
}
```

### Normalized Snapshot

Raw Fact 是 Collector 与 Normalizer 的协议；Rule 不直接消费 Raw Fact。

```go
type Snapshot struct {
	Meta          Meta   `json:"-"` // serialized by the Report envelope
	Host          Host   `json:"host"`
	CPU           CPU    `json:"cpu"`
	Memory        Memory `json:"memory"`

	NUMA    NUMAState    `json:"numa"`
	PCI     PCIState     `json:"pci"`
	GPUs    []GPU        `json:"gpus"`
	Network NetworkState `json:"network"`
	RDMA    RDMAState    `json:"rdma"`
	Storage StorageState `json:"storage"`

	NVIDIA NVIDIAStack `json:"nvidia"`
	Docker DockerState `json:"docker"`

	Containers []Container       `json:"containers"`
	Processes  []Process         `json:"processes"`
	Runtimes   []RuntimeInstance `json:"runtimes"`

	Collection []CollectorStatus `json:"collection"`
}

type Meta struct {
	SchemaVersion        string    `json:"schema_version"`
	AIStatVersion        string    `json:"aistat_version"`
	CollectedAt          time.Time `json:"collected_at"`
	Profile              string    `json:"profile"`
	CompatibilityVersion string    `json:"compatibility_version"`
}
```

关键类型：

```go
type GPU struct {
	UUID              string `json:"uuid"`
	Index             int    `json:"index"`
	Model             string `json:"model"`
	PCIBDF            string `json:"pci_bdf"`
	NUMANode          *int   `json:"numa_node,omitempty"`
	MemoryBytes       uint64 `json:"memory_bytes"`
	ComputeMode       string `json:"compute_mode,omitempty"`
	PowerLimitWatts   *int   `json:"power_limit_watts,omitempty"`
	DefaultPowerWatts *int   `json:"default_power_watts,omitempty"`
	ECCUncorrectable  *uint64 `json:"ecc_uncorrectable,omitempty"`
	PersistenceMode   *bool   `json:"persistence_mode,omitempty"`
	MIGMode           string  `json:"mig_mode,omitempty"`
	BAR1TotalBytes    *uint64 `json:"bar1_total_bytes,omitempty"`
	BAR1UsedBytes     *uint64 `json:"bar1_used_bytes,omitempty"`
}

type NUMANode struct {
	ID          int      `json:"id"`
	CPUSet      []int    `json:"cpus"`
	MemoryBytes uint64   `json:"memory_bytes"`
	Distance    []uint32 `json:"distance,omitempty"`
}

type RuntimeInstance struct {
	Kind        string            `json:"kind"` // vllm, sglang
	Version     string            `json:"version,omitempty"`
	PID         int               `json:"pid"`
	ContainerID string            `json:"container_id,omitempty"`
	Executable  string            `json:"executable"`
	GPUs        []string          `json:"gpus,omitempty"` // UUIDs after normalization
	CPUSet      []int             `json:"cpu_set,omitempty"`
	NUMAMems    []int             `json:"numa_mems,omitempty"`
	TP          *int              `json:"tensor_parallel,omitempty"`
	PP          *int              `json:"pipeline_parallel,omitempty"`
	DP          *int              `json:"data_parallel,omitempty"`
	Args        map[string]string `json:"args,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
}

type Container struct {
	ID             string   `json:"id"`
	EffectiveCPUs  []int    `json:"effective_cpus,omitempty"`
	EffectiveMems  []int    `json:"effective_mems,omitempty"`
	MemoryLimit    *uint64  `json:"memory_limit_bytes,omitempty"`
	SHMSize        *uint64  `json:"shm_size_bytes,omitempty"`
	MemlockSoft    *uint64  `json:"memlock_soft_bytes,omitempty"`
	MemlockHard    *uint64  `json:"memlock_hard_bytes,omitempty"`
	GPUDeviceRefs  []string `json:"gpu_device_refs,omitempty"`
}

type Process struct {
	PID         int               `json:"pid"`
	Executable string            `json:"executable"`
	ContainerID string           `json:"container_id,omitempty"`
	CPUSet      []int             `json:"cpu_set,omitempty"`
	NUMAMems    []int             `json:"numa_mems,omitempty"`
	AllowedArgs map[string]string `json:"args,omitempty"`
	AllowedEnv  map[string]string `json:"env,omitempty"`
}
```

所有 optional scalar 必须用指针、显式状态包装或等价 presence 机制区分“真实零值”和“未取得”。Meta、必采字段及每个 Collector 的采集状态必须始终出现在 JSON 中；条件字段可以省略，但不能让缺失被解释为正常值。

### Topology Graph

```go
package topology

type NodeKind string

const (
	NodeHost       NodeKind = "host"
	NodeNUMA       NodeKind = "numa"
	NodeCPUPackage NodeKind = "cpu_package"
	NodeCPU        NodeKind = "cpu"
	NodePCIRoot    NodeKind = "pci_root"
	NodePCIBridge  NodeKind = "pci_bridge"
	NodePCI        NodeKind = "pci"
	NodeGPU        NodeKind = "gpu"
	NodeNIC        NodeKind = "nic"
	NodeRDMA       NodeKind = "rdma"
	NodeStorage    NodeKind = "block_device"
	NodeContainer  NodeKind = "container"
	NodeProcess    NodeKind = "process"
	NodeSoftware   NodeKind = "software"
)

type EdgeKind string

const (
	EdgeContains    EdgeKind = "contains"
	EdgeAttachedTo  EdgeKind = "attached_to"
	EdgeLocalTo     EdgeKind = "local_to"
	EdgeConnectedTo EdgeKind = "connected_to"
	EdgePeerToPeer  EdgeKind = "peer_to_peer"
	EdgeRunsIn      EdgeKind = "runs_in"
	EdgeUses        EdgeKind = "uses"
	EdgeVisibleTo   EdgeKind = "visible_to"
	EdgeBoundTo     EdgeKind = "bound_to"
)

type Node struct {
	ID   string
	Kind NodeKind
	Meta map[string]string
}

type Edge struct {
	From       string
	To         string
	Kind       EdgeKind
	Distance   int
	Attributes map[string]string
}

type Graph struct {
	Nodes map[string]Node
	Edges []Edge
}

func Build(snapshot *model.Snapshot) (*Graph, error)
func (g *Graph) Neighbors(id string, kind EdgeKind) []Node
func (g *Graph) Distance(from, to string) (int, bool)
func (g *Graph) LocalNUMA(id string) (*Node, bool)
func (g *Graph) ClosestNIC(gpuID string) (*Node, bool)
func (g *Graph) StrictlyBetterGPUSet(candidate, selected []string) (bool, error)
```

内部 topology distance 可以定义为 AIStat 自身的**相对成本模型**，不能冒充真实 latency：

```text
NVLink/NVSwitch     low
PIX                 low
PXB                 medium
PHB                 higher
NODE                higher
SYS                 highest
```

NVIDIA 对这些 path labels 的官方含义已经有正式定义；AIStat 可以基于含义建立内部排序，但该排序属于 AIStat heuristic，因此 Finding 应显示 confidence，而不能声称是硬件实测 latency。citeturn6view2

Graph 必须保留 PCI root/bridge 层级和 P2P 支持状态；只记录 `GPU → NUMA` 无法支撑 ACS、GPUDirect、GPU↔NIC root locality 与严格支配 GPU-set 规则。Graph builder 必须 deterministic、无 I/O，并只从 Snapshot 构建。

### Rule Contract

输入：

```go
RuleInput{
    Snapshot,
    Graph,
    Profile,
}
```

输出：

```go
RuleResult
```

接口：

```go
package rules

type RuleID string

type Rule interface {
	ID() RuleID
	Meta() RuleMeta
	Evaluate(ctx RuleContext) []model.Finding
}

type RuleContext struct {
	Snapshot *model.Snapshot
	Graph    *topology.Graph
	Profile  model.Profile
	Now      time.Time
}

type RuleMeta struct {
	Title       string
	Domain      string
	Dimension   model.Dimension
	Priority    model.Priority
	Confidence  model.Confidence
	Description string
	References  []model.Reference
}
```

返回 slice 是为了允许同一 Rule 对多个 GPU、进程或容器分别产生 Finding。Rule 不执行 I/O；`Now` 由 orchestrator 注入，确保生命周期/Xid 观察窗规则可重复测试。

Finding：

```go
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
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
	SeverityInfo     Severity = "info"
)

type Evidence struct {
	Label  string    `json:"label"`
	Value  any       `json:"value"`
	Source SourceRef `json:"source"`
}

type Finding struct {
	RuleID         RuleID       `json:"rule_id"`
	Title          string       `json:"title"`
	Domain         string       `json:"domain"`
	Dimension      Dimension    `json:"dimension"`
	Priority       Priority     `json:"priority"`
	Status         Status       `json:"status"`
	Severity       Severity     `json:"severity"`
	Confidence     Confidence   `json:"confidence"`
	CurrentState   string       `json:"current_state"`
	ExpectedState  string       `json:"expected_state,omitempty"`
	Evidence       []Evidence   `json:"evidence"`
	Impact         string       `json:"impact"`
	Recommendation string       `json:"recommendation"`
	Verification   []string     `json:"verification"`
	References     []Reference  `json:"references,omitempty"`
}
```

JSON enum 永远使用小写；Human Reporter 展示为大写。`UNKNOWN` 表示规则适用但证据不足，`SKIP` 表示当前上下文本来不适用。任何缺失、权限不足、超时或解析失败证据都不得静默转换为 PASS。

Readiness 聚合规则冻结如下：

```text
Deployment Readiness:
  NOT_READY  if any applicable deployment finding is FAIL
  UNKNOWN    else if any applicable deployment rule is UNKNOWN
  READY      otherwise

Performance Readiness:
  WARN       if any applicable performance finding is WARN or FAIL
  UNKNOWN    else if any applicable performance rule is UNKNOWN
  READY      otherwise

INFO and SKIP do not lower readiness.
Priority/Severity affect sorting and presentation, not the aggregation formula.
```

同一 Rule 对多个 subject 产生结果时，Readiness 使用该 Rule 中最差的 applicable status；最终 Findings 按 dimension、status、priority、rule ID、subject ID 稳定排序。

### 示例 Finding

```json
{
  "rule_id": "NUMA001",
  "title": "AI runtime CPU affinity is remote from selected GPU",
  "domain": "numa",
  "status": "warn",
  "dimension": "performance",
  "priority": "P1",
  "severity": "medium",
  "confidence": "high",
  "current_state": "vLLM PID 18231 has no CPU affinity overlap with GPU0-local CPUs.",
  "evidence": [
    {
      "label": "gpu_numa",
      "value": 0,
      "source": {
        "kind": "sysfs",
        "locator": "/sys/bus/pci/devices/0000:31:00.0/numa_node"
      }
    },
    {
      "label": "process_cpuset",
      "value": "32-63",
      "source": {
        "kind": "procfs",
        "locator": "/proc/18231/status"
      }
    }
  ],
  "impact": "The runtime is scheduled away from CPUs local to the selected GPU.",
  "recommendation": "Evaluate binding host-side runtime work to CPUs local to the GPU NUMA domain.",
  "verification": [
    "Repeat the same inference benchmark after changing CPU affinity.",
    "Run aistat check again and confirm the process has GPU-local CPU access."
  ]
}
```

Linux NUMA memory policy 与 cpuset 是不同机制，并且 cpuset 约束会限制任务可使用的 CPU/memory nodes；因此 runtime/process/container locality 应分别建模，不能只看 GPU NUMA ID。citeturn3search33turn3search5

### Profile

```go
type Profile struct {
	Name string `json:"name"`

	Workload struct {
		Kind        string `json:"kind"`
		LatencyBias bool   `json:"latency_bias"`
		MultiGPU    bool   `json:"multi_gpu"`
	} `json:"workload"`
}
```

v0.1：

```text
general
llm-inference
```

默认：

```bash
aistat
```

等价：

```bash
aistat status --profile llm-inference
```

### CLI

```text
aistat
aistat status
aistat check
aistat info
aistat topology
aistat stack
aistat runtime
aistat explain RULE_ID
aistat version
```

公共参数：

```text
--format human|json
--profile general|llm-inference
--timeout 10s
--no-color
--fail-on fail|warn
```

`aistat` 与 `aistat status` 提供节点运维总览；`check` 输出完整 Finding、证据与建议。`--format human` 是公开名称；内部实现可以继续使用 `FormatHuman`。JSON 状态使用小写，Human 输出使用大写。`--fail-on warn` 使 WARN 也返回 exit code 1，便于严格 CI；默认仍只对 FAIL 返回 1。

建议 V0.1 **不用 Cobra**，采用 Go 标准库 `flag` + 小型 dispatcher，减少依赖和 binary surface。

### 关键文件模板

**`cmd/aistat/main.go`**

```go
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/ORG/aistat/internal/app"
)

func main() {
	ctx, cancel := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer cancel()

	if err := app.Run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(app.ExitCode(err))
	}
}
```

**`internal/collector/collector.go`**

```go
package collector

import (
	"context"

	"github.com/ORG/aistat/internal/clock"
	"github.com/ORG/aistat/internal/execx"
	"github.com/ORG/aistat/internal/fsx"
	"github.com/ORG/aistat/internal/model"
)

type ID string
type Capability string

type Collector interface {
	ID() ID
	Provides() []Capability
	Requires() []Capability
	Collect(context.Context, Env) Result
}

type Env struct {
	Runner     execx.Runner
	FileSystem fsx.FileSystem
	Clock      clock.Clock
}

type Result struct {
	Collector   ID
	Facts       []model.Fact
	Diagnostics []model.Diagnostic
}
```

**`internal/topology/graph.go`**

```go
package topology

import "github.com/ORG/aistat/internal/model"

type Graph struct {
	Nodes map[string]Node
	Edges []Edge
}

func Build(snapshot *model.Snapshot) (*Graph, error) {
	// Build only from normalized snapshot.
	// Never read /proc, /sys or execute commands here.
	panic("TODO")
}
```

**`internal/rules/engine.go`**

```go
package rules

import (
	"context"

	"github.com/ORG/aistat/internal/model"
)

type Engine struct {
	rules []Rule
}

func NewEngine(rules ...Rule) *Engine {
	return &Engine{rules: rules}
}

func (e *Engine) Evaluate(ctx context.Context, in RuleContext) []model.Finding {
	out := make([]model.Finding, 0)

	for _, rule := range e.rules {
		select {
		case <-ctx.Done():
			return out
		default:
			out = append(out, rule.Evaluate(in)...)
		}
	}

	return out
}
```

**`internal/report/report.go`**

```go
package report

import (
	"io"
	"time"

	"github.com/ORG/aistat/internal/model"
)

type Format string

const (
	FormatHuman Format = "human"
	FormatJSON Format = "json"
)

type Report struct {
	SchemaVersion        string          `json:"schema_version"`
	AIStatVersion        string          `json:"aistat_version"`
	CollectedAt          time.Time       `json:"collected_at"`
	Profile              string          `json:"profile"`
	CompatibilityVersion string          `json:"compatibility_version"`
	Readiness            model.Readiness `json:"readiness"`
	Summary              model.Summary   `json:"summary"`
	Node                 *model.Snapshot `json:"node"`
	Findings             []model.Finding `json:"findings"`
}

func Write(w io.Writer, format Format, report Report) error
```

Human 与 JSON Reporter 必须消费同一个 `Report`。Report builder 从 `Snapshot.Meta` 复制固定的顶层 metadata，Readiness 和 Summary 在 rule orchestration 阶段一次性计算并进入 Report；Reporter 只排序/展示，不重复判断。JSON 顶层结构冻结为 metadata + readiness + summary + node + findings，并必须通过 `docs/schema/report-v0.1.schema.json` 验证。

**`go.mod`**

GitHub org 未指定；以下 `ORG` 必须在 M0 替换。

```go
module github.com/ORG/aistat

go 1.26.0

toolchain go1.26.5
```

截至本计划修订日，Go 官方稳定下载版本为 1.26.5；M0 使用该 patch。后续升级 patch 版本必须通过 CI，不自动漂移到未来 minor。

V0.1 应尽量不引入 production dependencies。

**`Makefile`**

Makefile 只服务 Linux/CI，不作为 Windows 开发入口。Windows 本地以 `go` 原生命令和 `scripts/dev.ps1` 为准；两者必须调用相同的底层 Go targets，避免维护两套测试逻辑。

```makefile
BINARY := aistat
PKG := ./...

.PHONY: build test vet fmt lint staticcheck cross snapshot clean

build:
	CGO_ENABLED=0 go build -trimpath -o bin/$(BINARY) ./cmd/aistat

test:
	go test -race $(PKG)

vet:
	go vet $(PKG)

fmt:
	test -z "$$(gofmt -l .)"

lint:
	golangci-lint run ./...

staticcheck:
	staticcheck ./...

cross:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o dist/aistat-linux-amd64 ./cmd/aistat
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -o dist/aistat-linux-arm64 ./cmd/aistat

snapshot:
	goreleaser release --snapshot --clean

clean:
	rm -rf -- bin dist
```

**Windows 本地等价命令：**

```powershell
go test ./...
go vet ./...
$env:CGO_ENABLED = '0'
$env:GOOS = 'linux'
$env:GOARCH = 'amd64'
go build -trimpath -o dist/aistat-linux-amd64 ./cmd/aistat
$env:GOARCH = 'arm64'
go build -trimpath -o dist/aistat-linux-arm64 ./cmd/aistat
```

Windows 不运行上述 Linux binary。`scripts/dev.ps1` 只封装 build/test/vet/cross 等无副作用开发命令，不负责安装、发布或修改主机。

## Collector 开发任务与外部接口规范

### Collector 总矩阵

| Collector | P | 主要输入 | 主要输出 | 外部命令 |
|---|---:|---|---|---|
| `system` | P0 | `/etc/os-release`, `/proc`, `/sys/class/dmi` | Host facts | `uname` optional |
| `cpu` | P0 | `/proc/cpuinfo`, `/sys/devices/system/cpu` | socket/core/thread/cache/governor | none |
| `memory` | P0 | `/proc/meminfo`, THP sysfs | RAM/swap/THP/HugePages | none |
| `numa` | P0 | `/sys/devices/system/node` | node→CPU/memory/distance | none |
| `pci` | P0 | `/sys/bus/pci/devices` | BDF/tree/link/NUMA | `lspci` optional |
| `nvidia` | P0 | sysfs + NVIDIA CLI | GPU/driver/CUDA/NCCL/topology | `nvidia-smi` |
| `docker` | P0 | Docker CLI + cgroup | daemon/container/resources/NCT | `docker`,`nvidia-ctk` |
| `pytorch` | P0 | best-effort subprocess | torch version/CUDA/device_count | `python3` optional |
| `vllm` | P0 | `/proc`, args/env allowlist | runtime process/TP/PP/DP/GPU | no mandatory |
| `sglang` | P0 | `/proc`, args/env allowlist | runtime process/TP/PP/GPU | no mandatory |
| `storage` | P1 | `/sys/block`,`/proc/mounts` | device/fs/mount/capacity/BDF | none |
| `network` | P0 | `/sys/class/net` | NIC/BDF/NUMA/MTU/RDMA | `ip`,`ethtool`,`rdma` optional |

### system

输入：

```text
/etc/os-release
/proc/version
/proc/uptime
/sys/class/dmi/id/*
```

输出字段：

```text
hostname
os.id
os.version
kernel.release
arch
virtualization/bare-metal best effort
bios.vendor
bios.version
```

权限失败不是 fatal：

```json
{
  "state": "unknown",
  "diagnostic": "permission_denied"
}
```

### cpu

输入：

```text
/sys/devices/system/cpu/cpu*/topology/*
/sys/devices/system/cpu/cpu*/cache/*
/sys/devices/system/cpu/cpufreq/*
/proc/cpuinfo
```

输出：

```text
vendor
model
logical_cpu_count
socket_count
core_count
threads_per_core
SMT
socket→CPU
core→thread
cache
governor
current/max frequency
```

Linux kernel 文档公开了 CPU topology 通过 sysfs 的 ABI，因此应该优先使用 sysfs topology，而不是依赖 `lscpu` 文本格式。citeturn3search0turn3search16

### memory

输入：

```text
/proc/meminfo
/sys/kernel/mm/transparent_hugepage/enabled
/proc/sys/kernel/numa_balancing
resource limits
```

输出：

```text
MemTotal
MemAvailable
SwapTotal
SwapFree
HugePages_Total
HugePages_Free
Hugepagesize
THP mode
NUMA balancing
memlock current limit
```

THP/NUMA balancing 等在 V0.1 只作为事实，不应仅凭某个状态自动判 FAIL。

### numa

输入：

```text
/sys/devices/system/node/node*/
```

输出：

```text
node ID
cpulist
meminfo
distance
```

Linux cgroup/cpuset 官方接口中也存在 effective CPU/memory-node 约束，因此 Host NUMA 与 Container effective NUMA 必须分别保存。citeturn3search5

### pci

优先：

```text
/sys/bus/pci/devices/*
```

解析：

```text
BDF
vendor/device/class
numa_node
parent bridge
current_link_speed
current_link_width
max_link_speed
max_link_width
```

`lspci` 仅作为 enrichment/fallback，不作为 mandatory dependency。

### nvidia

建议一次运行：

```bash
nvidia-smi --query-gpu=...
```

再分别使用必要 topology command。

不要几十次 fork `nvidia-smi`。

输出：

```text
GPU UUID
index
model
BDF
VRAM
driver
compute mode
MIG mode
ECC
power limit
default power limit
temperature snapshot
utilization snapshot
PCIe
topology
```

Topology 尽可能使用稳定 machine-oriented NVIDIA output。当前 NVIDIA 文档已经给 `topo -gpu/-nic/-all` 提供固定宽度输出并明确说明适合 machine parsing。citeturn6view2

### CUDA / NCCL

放入：

```text
internal/collector/nvidia/stack.go
```

检测顺序：

```text
Driver
→ driver-supported CUDA
→ /usr/local/cuda*
→ nvcc if available
→ library search best effort
→ ldconfig cache best effort
→ NCCL package/library version
```

CUDA compatibility Rule 必须使用**版本化 compatibility table**，不能：

```go
if runtime > driverCUDA {
    fail
}
```

简单字符串比较。

CUDA 官方 compatibility 模型比这种逻辑复杂，尤其从 CUDA 11 开始存在 minor-version compatibility。citeturn0search1turn0search7

### docker

检查：

```text
docker binary
docker daemon reachable
server version
cgroup mode
running containers
AI runtime containers
cpuset
cpuset.mems
memory limit
swap
shm
ulimit memlock
GPU request/device visibility
NVIDIA Container Toolkit
```

Docker 官方支持 CPU quota、`cpuset-cpus`、`cpuset-mems`、memory/swap、`--shm-size` 和 ulimit；Docker GPU access 使用 `--gpus` 提供 GPU 设备，所以这些都是合法的 checklist 输入。citeturn2search0turn2search4turn2search5

### pytorch

Python 不是 AIStat 自身依赖。

逻辑：

```text
python3 exists?
   no → not_detected / SKIP
   yes
    ↓
short subprocess timeout
    ↓
import torch
    ↓
version
torch.version.cuda
torch.cuda.is_available()
torch.cuda.device_count()
```

PyTorch 官方本身建议用 `torch.cuda.is_available()` 判断 CUDA 是否对 PyTorch 可用，并提供 `device_count()` 查询 GPU 数量。citeturn1search7turn1search23

Probe 脚本不得：

```text
import user code
load model
download model
connect internet
initialize massive tensors
```

### vllm

识别 process：

```text
vllm serve
python -m vllm...
known executable patterns
```

只解析 allowlisted flags：

```text
--tensor-parallel-size / -tp
--pipeline-parallel-size / -pp
--data-parallel-size / -dp
--device-ids
--numa-bind-cpus
--distributed-executor-backend
--nnodes
--node-rank
```

vLLM 当前官方 CLI 已正式支持这些并行和 device-selection 选项。citeturn6view1

### sglang

识别：

```text
sglang serve
python -m sglang.launch_server
```

只解析：

```text
model path
tensor parallel
data parallel
device-related options
distributed-related options
known infrastructure flags
```

SGLang 官方说明 server args 控制模型、parallelism、memory management 和 performance behavior。citeturn6view0

### network

输出：

```text
interface
operstate
MAC
MTU
speed
driver
PCI BDF
NUMA
RDMA device
IB/RoCE best effort
```

以后建立：

```text
GPU → NUMA → PCIe → NIC/RDMA
```

### storage

v0.1 只 inventory：

```text
device
type
model
size
filesystem
mount
free
PCI BDF
NUMA best effort
```

不运行 `fio`。

## 首发 Rule 规范与 AI Infra 判断逻辑

规则应参考以下官方依据：NVIDIA topology path 与 affinity 来自 NVIDIA SMI 文档；CUDA compatibility 来自 NVIDIA CUDA compatibility 文档；GPU container 能力来自 NVIDIA Container Toolkit 与 Docker GPU/resource 文档；NCCL 环境变量的语义及若干 debugging/tuning 参数的使用注意来自 NCCL 官方手册；PyTorch CUDA health 可用官方 CUDA API 验证；vLLM 与 SGLang 的 parallelism 参数应按各自当前官方 CLI 文档解析。citeturn6view2turn0search1turn0search8turn2search5turn0search9turn1search1turn6view1turn6view0

Rule reference metadata 使用逻辑 ID：

```text
REF-NVIDIA-SMI
REF-CUDA-COMPAT
REF-NVIDIA-CONTAINER
REF-DOCKER-RESOURCES
REF-DOCKER-GPU
REF-NCCL-ENV
REF-PYTORCH-CUDA
REF-VLLM-SERVE
REF-SGLANG-SERVER
REF-LINUX-CGROUP
```

### 首批二十五条

| ID | P | 标题 | Trigger | Status / Confidence | Evidence | Recommendation |
|---|---:|---|---|---|---|---|
| `GPU001` | P0 | NVIDIA GPU 存在但 driver stack 不可用 | `pci_nvidia_gpu>0 && nvidia_driver_usable=false` | FAIL/high | PCI sysfs + nvidia-smi error | 修复 NVIDIA driver/userspace stack |
| `GPU002` | P0 | GPU compute mode 阻止常规 compute | workload 选中 GPU 且 `compute_mode == PROHIBITED` | FAIL/high | nvidia-smi | 使用允许 compute 的 GPU 或由管理员核对配置 |
| `GPU003` | P0 | 检测到 volatile uncorrectable ECC | ECC-capable GPU 的 volatile uncorrectable counter > 0 | FAIL/high | nvidia-smi/DCGM field | 进行 NVIDIA health/field diagnosis |
| `GPU004` | P0 | 近期严重 NVIDIA Xid | 指定观察窗内存在受支持 catalog 中的 critical Xid | FAIL/high | journal/dmesg normalized events | 按 Xid 官方处置流程排查 |
| `PCIE001` | P1 | Active GPU negotiated width 低于 max | GPU active 且 `current_width < max_width` | WARN/high | sysfs/nvidia-smi | 检查 slot、riser、BIOS、平台设计 |
| `PCIE002` | P1 | ACS 可能重定向 peer traffic | bare-metal + P2P/GDR context + 相关 bridge ACS redirect 风险 | WARN/high | sanitized lspci + PCI graph | 按 OEM/NVIDIA 指南核对，不自动关闭安全功能 |
| `NUMA001` | P1 | AI process 无 GPU-local CPU | `intersection(process.cpus,gpu.local_cpus)==0` | WARN/high | procfs + graph | 测试 GPU-local CPU placement |
| `NUMA002` | P1 | Effective memory nodes 排除 GPU-local NUMA | `gpu.numa ∉ effective_mems` | WARN/high | cgroup + graph | 对齐 effective memory-node allocation |
| `TOPO001` | P1 | Selected multi-GPU group 被严格支配 | 当前组存在较差 pair，且有同规模、可见、所有关键 pair 不差且至少一项更优的候选组 | WARN/high | topology + P2P + runtime | A/B 测试严格更优 GPU group |
| `NET001` | P0 | 显式选择的 NIC/HCA 不可用 | runtime/NCCL 明确指定的接口不存在、down 或不可用 | FAIL/high | env allowlist + NIC/RDMA | 修正选择或恢复设备 |
| `NET002` | P1 | GDR 路径跨不受支持 PCIe root | GDR context 已确认且 GPU/NIC root arrangement 明确不满足支持条件 | WARN/high | PCI root graph | 选择更近 pairing 并用官方 benchmark 验证 |
| `NET003` | P0 | RDMA/NCCL memlock 受限 | RDMA/IB context 且 effective memlock 明确不足 | FAIL/high | process/container limits | 按 deployment policy 提高 memlock |
| `NCCL001` | P0 | NCCL interface/HCA filter 无匹配 | `NCCL_SOCKET_IFNAME` 或 `NCCL_IB_HCA` 无可用匹配 | FAIL/high | exact env + NIC/RDMA | 修正或移除错误 override |
| `CUDA001` | P0 | Active CUDA runtime 与 driver 不兼容 | versioned compatibility table 判定无合法 driver/compat path | FAIL/high | active runtime + driver + compat dataset | 升级 driver 或使用兼容 runtime |
| `CUDA002` | P2 | NVIDIA driver branch 已 EOL | versioned lifecycle dataset 判定已 EOL | WARN/high | driver branch + embedded dataset | 规划迁移到受支持 branch |
| `TORCH001` | P0 | Active GPU runtime 的 PyTorch CUDA 不可用 | 同一 runtime interpreter 中 `torch.cuda.is_available()==false` | FAIL/high | bounded Python probe | 检查 build、driver、container visibility |
| `TORCH002` | P0 | PyTorch GPU 数低于 runtime 需求 | `local_world_size > torch.cuda.device_count()` | FAIL/high | runtime + PyTorch probe | 修正 GPU visibility 或 parallelism |
| `CTR001` | P0 | Docker workflow 已检测但 daemon 不可达 | `docker_needed && daemon_confirmed_unavailable`；permission denied → UNKNOWN | FAIL/high | docker command | 启动/修复 daemon；权限问题单独提示 |
| `CTR002` | P0 | Docker GPU workflow 缺少或未配置 NCT | 已确认 `docker_gpu_needed && (!nct_detected || !docker_configured)`；证据不可读 → UNKNOWN | FAIL/high | nvidia-ctk/package/runtime | 安装并配置 NVIDIA Container Toolkit |
| `CTR003` | P0 | AI container 期望 GPU 但 GPU 不可见 | `container_gpu_expected && visible_gpu_count==0` | FAIL/high | Docker + device nodes | 检查 GPU request/runtime/device mapping |
| `CTR004` | P0 | Multi-process NCCL container `/dev/shm` 过小 | multi-process/multi-GPU NCCL context 且 shm ≤ 64 MiB/default | FAIL/high | Docker inspect + runtime | 增大 shm 或使用受支持 IPC strategy |
| `VLLM001` | P0 | vLLM local GPU 需求超过 visibility | `local_world_size > effective_visible_gpu_count` | FAIL/high | vLLM args + resolved GPUs | 修正 parallelism 或 visibility |
| `VLLM002` | P0 | vLLM GPU selection 含无效引用 | resolved device map 有 duplicate/nonexistent/unmappable ref | FAIL/high | exact env + GPU inventory | 修正 device mapping |
| `SGL001` | P0 | SGLang local GPU 需求超过 visibility | `local_world_size > effective_visible_gpu_count` | FAIL/high | SGLang args + resolved GPUs | 修正 TP/DP/GPU visibility |
| `SGL002` | P0 | SGLang disaggregation HCA 无效 | PD mode 且显式 IB device 不存在或不可用 | FAIL/high | SGLang args + RDMA inventory | 修正 IB device mapping |

首发 Rule 清单以本表为唯一权威版本。`NCCL_P2P_DISABLE`、`NCCL_SHM_DISABLE`、power limit、idle PCIe speed、多 Toolkit 等继续作为事实或 INFO 候选，不占用首发 25 条高置信度 Rule ID。后续新增 Rule 必须使用新 ID，禁止复用上述 ID 改变语义。

同理，Docker `cpuset-cpus` 与 `cpuset-mems` 是正式资源约束机制，Linux cgroup v2 也定义了 effective CPU/memory sets，所以 NUMA001/002 属于跨层 Rule，而非 Docker “最佳实践猜测”。citeturn2search0turn3search5

### 每条 Rule 示例 Finding 核心 JSON

以下省略重复的 `why/references/evidence.source` 细节，但每个实际实现必须输出完整 Finding。

```json
{"rule_id":"GPU001","status":"fail","dimension":"deployment","priority":"P0","current_state":"NVIDIA PCI GPU detected but the driver stack is unusable."}
{"rule_id":"GPU002","status":"fail","dimension":"deployment","priority":"P0","current_state":"GPU0 compute mode is PROHIBITED."}
{"rule_id":"GPU003","status":"fail","dimension":"deployment","priority":"P0","current_state":"GPU0 reports volatile uncorrectable ECC errors."}
{"rule_id":"GPU004","status":"fail","dimension":"deployment","priority":"P0","current_state":"A recent critical NVIDIA Xid was detected."}
{"rule_id":"PCIE001","status":"warn","dimension":"performance","priority":"P1","current_state":"Active GPU0 negotiated PCIe x8 while capability is x16."}
{"rule_id":"PCIE002","status":"warn","dimension":"performance","priority":"P1","current_state":"ACS may redirect peer traffic on the selected path."}
{"rule_id":"NUMA001","status":"warn","dimension":"performance","priority":"P1","current_state":"vLLM worker CPUs do not overlap GPU0-local CPUs."}
{"rule_id":"NUMA002","status":"warn","dimension":"performance","priority":"P1","current_state":"Effective memory nodes exclude GPU0-local NUMA."}
{"rule_id":"TOPO001","status":"warn","dimension":"performance","priority":"P1","current_state":"The selected GPU set is strictly dominated by an available set."}
{"rule_id":"NET001","status":"fail","dimension":"deployment","priority":"P0","current_state":"The explicitly selected NIC or HCA is unavailable."}
{"rule_id":"NET002","status":"warn","dimension":"performance","priority":"P1","current_state":"The required GDR path crosses an unsupported PCIe root arrangement."}
{"rule_id":"NET003","status":"fail","dimension":"deployment","priority":"P0","current_state":"Effective memlock is insufficient for the detected RDMA workflow."}
{"rule_id":"NCCL001","status":"fail","dimension":"deployment","priority":"P0","current_state":"The NCCL interface or HCA filter matches no usable device."}
{"rule_id":"CUDA001","status":"fail","dimension":"deployment","priority":"P0","current_state":"The active CUDA runtime has no supported driver compatibility path."}
{"rule_id":"CUDA002","status":"warn","dimension":"deployment","priority":"P2","current_state":"The installed NVIDIA driver branch has reached EOL."}
{"rule_id":"TORCH001","status":"fail","dimension":"deployment","priority":"P0","current_state":"PyTorch reports CUDA unavailable in the active runtime interpreter."}
{"rule_id":"TORCH002","status":"fail","dimension":"deployment","priority":"P0","current_state":"PyTorch sees fewer GPUs than the runtime local world size requires."}
{"rule_id":"CTR001","status":"fail","dimension":"deployment","priority":"P0","current_state":"Docker is required but the daemon is unreachable."}
{"rule_id":"CTR002","status":"fail","dimension":"deployment","priority":"P0","current_state":"The NVIDIA Container Toolkit is missing or unconfigured."}
{"rule_id":"CTR003","status":"fail","dimension":"deployment","priority":"P0","current_state":"A GPU-required container has no effective GPU visibility."}
{"rule_id":"CTR004","status":"fail","dimension":"deployment","priority":"P0","current_state":"A multi-process NCCL container has default or tiny shared memory."}
{"rule_id":"VLLM001","status":"fail","dimension":"deployment","priority":"P0","current_state":"vLLM local GPU requirements exceed effective visibility."}
{"rule_id":"VLLM002","status":"fail","dimension":"deployment","priority":"P0","current_state":"vLLM GPU selection contains an invalid device reference."}
{"rule_id":"SGL001","status":"fail","dimension":"deployment","priority":"P0","current_state":"SGLang local GPU requirements exceed effective visibility."}
{"rule_id":"SGL002","status":"fail","dimension":"deployment","priority":"P0","current_state":"SGLang disaggregation references an unavailable HCA."}
```

上面仅用于快速核对 ID/状态/维度；正式 Finding 仍必须包含完整 Evidence、Impact、Recommendation、Verification、Confidence 和 References。

### Rule Unit Test 模板

```go
func TestNUMA001_RemoteCPUAffinity(t *testing.T) {
	t.Parallel()

	snapshot := fixture.Builder{
		GPU: model.GPU{
			UUID:     "GPU-0",
			NUMANode: ptr.Int(0),
		},
		Runtime: model.RuntimeInstance{
			Kind:   "vllm",
			PID:    1234,
			GPUs:   []string{"GPU-0"},
			CPUSet: []int{32, 33, 34, 35},
		},
		NUMA: []model.NUMANode{
			{ID: 0, CPUSet: ints.Range(0, 31)},
			{ID: 1, CPUSet: ints.Range(32, 63)},
		},
	}.Build(t)

	graph, err := topology.Build(snapshot)
	if err != nil {
		t.Fatal(err)
	}

	findings := numa.NewNUMA001().Evaluate(
		rules.RuleContext{
			Snapshot: snapshot,
			Graph:    graph,
			Profile:  model.LLMInferenceProfile(),
			Now:      time.Date(2026, 8, 13, 0, 0, 0, 0, time.UTC),
		},
	)

	if len(findings) != 1 || findings[0].Status != model.StatusWarn {
		t.Fatalf("got %#v, want one WARN finding", findings)
	}
}
```

每条首发 Rule 至少包含：

```text
positive trigger test
negative/pass test
missing-data test
skip/not-applicable test
boundary test
false-positive regression test for complex rules
```

## 测试、性能、安全与 CI/CD

### Windows 开发与分层验证策略

开发机是 Windows，但产品运行目标仍严格限定为 Linux。测试按“逻辑可移植、采集需 Linux、硬件需真实环境”分层，不能因为本地没有 Linux/GPU 就跳过模型和规则验证，也不能把 Windows 测试结果冒充 Linux 集成验证。

| 层级 | Windows 本地 | Ubuntu GitHub Actions | 真实 NVIDIA Linux |
|---|---|---|---|
| model/profile/compat | 必须运行 | 必须运行 | 随全量测试运行 |
| parser + sanitized fixtures | 必须运行 | 必须运行 | 回归验证 |
| normalizer/topology/rules | 必须运行 | 必须运行 | 回归验证 |
| reporter/JSON schema/golden | 必须运行 | 必须运行 | 回归验证 |
| redaction/privacy | 必须运行 | 必须运行 | 抽样复核 |
| execx helper-process tests | 必须运行 | 必须运行 | 随全量测试运行 |
| `/proc` `/sys` cgroup collectors | fixture 测试 | fixture + CPU-only live smoke | live 验证 |
| Linux CLI smoke | 只 cross-build，不执行 | 必须执行 | 必须执行 |
| nvidia-smi/Docker/RDMA live | 不运行 | 默认不运行 | manual/scheduled |
| performance budget | 不作为结论 | CPU-only baseline | release gate |

实现约束：

```text
1. parser 与 filesystem traversal 分离。
2. Collector 通过 fsx.FileSystem/Root 注入 fixture，不直接依赖开发机路径。
3. Linux-specific syscall/process logic 使用 *_linux.go。
4. 必要时提供 *_windows.go stub，使 Windows 能编译和运行纯逻辑测试；stub 返回 unsupported，不伪造 Linux facts。
5. 测试不得依赖 echo、sleep、sh、bash、/bin/true 等平台命令。
6. execx 使用当前 Go test binary 的 helper-process pattern 测 success/nonzero/timeout/oversized output。
7. Windows `go test ./...` 是开发门；Ubuntu CI 与真实 NVIDIA 验证仍是发布门。
8. 所有 Linux fixture 必须 sanitized、source-attributed，并记录采集工具/版本。
```

Release gate：没有至少一轮真实 NVIDIA Linux integration validation，不允许发布 `v0.1.0`；真实 GPU 测试不作为普通社区 PR blocker。

### Fixture 目录

```text
testdata/
├── fixtures/
│   ├── linux/
│   │   ├── ubuntu2204-dual-numa/
│   │   ├── ubuntu2404-single-numa/
│   │   └── cgroupv2/
│   │
│   ├── nvidia/
│   │   ├── single-l4/
│   │   ├── four-a100-pcie/
│   │   ├── eight-h100-nvlink/
│   │   ├── nvswitch/
│   │   ├── driver-broken/
│   │   └── pcie-degraded/
│   │
│   ├── docker/
│   │   ├── gpu-ready/
│   │   ├── toolkit-missing/
│   │   └── remote-cpuset/
│   │
│   ├── pytorch/
│   │   ├── cuda-ok.json
│   │   └── cuda-unavailable.json
│   │
│   ├── vllm/
│   │   ├── tp4-local/
│   │   └── tp4-suboptimal/
│   │
│   └── sglang/
│       └── tp4/
│
├── snapshots/
│   ├── cpu-only.json
│   ├── 1gpu.json
│   ├── 4gpu-dual-numa.json
│   ├── 8gpu-nvswitch.json
│   ├── docker-vllm.json
│   └── broken-stack.json
│
└── golden/
    ├── check-cpu-only.txt
    ├── check-vllm-warn.txt
    ├── topology-4gpu.txt
    └── stack-broken.txt
```

### Sanitized fixture 生成规则

真实机器采样可以通过开发模式：

```bash
aistat debug capture --output raw-fixture
```

**注意：`debug capture` 不属于用户公开 v0.1 CLI；可作为 build-tag 或 internal developer command。**

sanitize 必须删除或稳定替换：

```text
hostname        → host-01
MAC             → 00:00:00:00:00:00
IP              → 192.0.2.x
serial number   → REDACTED
GPU UUID        → deterministic test UUID
container ID    → deterministic test ID
model path      → /models/example
username        → user
home path       → /home/user
```

绝不保存：

```text
arbitrary environment
tokens
API keys
SSH material
cloud credentials
Docker registry credentials
HF tokens
raw /proc/PID/environ
```

### 环境变量 allowlist

基础：

```text
CUDA_VISIBLE_DEVICES
NVIDIA_VISIBLE_DEVICES
```

NCCL 仅 allowlist 明确需要的：

```text
NCCL_P2P_DISABLE
NCCL_P2P_LEVEL
NCCL_SHM_DISABLE
NCCL_SOCKET_IFNAME
NCCL_IB_HCA
NCCL_NET
NCCL_NET_GDR_LEVEL
NCCL_IGNORE_CPU_AFFINITY
NCCL_DEBUG
NCCL_DEBUG_SUBSYS
```

vLLM/SGLang：

> 只收集**与 topology/resource/distributed execution 直接相关且在代码中显式列出的变量**。

绝不：

```go
for _, env := range os.Environ() {
    snapshot.Env = append(snapshot.Env, env)
}
```

### Process command line 隐私

禁止保存：

```text
/proc/PID/cmdline full raw string
```

因为用户可能将 credential 通过参数传入。

正确流程：

```text
read raw cmdline
    ↓
detect runtime
    ↓
parse allowlisted flags only
    ↓
discard raw bytes
```

例如：

```text
--tensor-parallel-size     KEEP
--pipeline-parallel-size   KEEP
--device-ids               KEEP

--token                    DROP
--api-key                  DROP
unknown option             DROP
```

### Safe Runner

```go
type CommandSpec struct {
	Name       string
	Args       []string
	Env        []string
	Dir        string
	Timeout    time.Duration
	MaxStdout  int64
	MaxStderr  int64
}

type CommandResult struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
	Duration time.Duration
}

type Runner interface {
	Run(ctx context.Context, spec CommandSpec) (CommandResult, error)
}
```

要求：

```text
no sh -c
no bash -c
no user-controlled executable
known-command allowlist
absolute resolved path
context timeout
stdout/stderr byte limits
no stdin
sanitized explicit environment
non-user-controlled working directory
terminate child process tree on timeout
sanitized errors
```

`exec.CommandContext` 是底线，但在 Linux 上超时还必须处理命令派生的子进程，不能只杀直接 child 后留下 orphan。Runner 的平台层使用 `runner_linux.go` / `runner_windows.go` 封装进程终止语义；普通生产路径只启用 Linux 实现，Windows 实现用于本地测试。可执行文件在 resolver 初始化时按 allowlist 解析为绝对路径并缓存，运行时不得接受用户提供的 path。

PyTorch probe 是唯一允许执行 Python 的固定用途命令：使用受信 resolver 得到的解释器、固定内置代码、isolated flags、空 stdin、受控 cwd、最小环境和严格输出/时间上限。不得执行从任意命令行参数拼出的代码，也不得沿用目标进程的完整环境。

Go `os/exec` 本身不会隐式调用系统 shell，`CommandContext` 支持 context cancellation，这正适合该设计。citeturn3search2turn3search30

### Command allowlist

```text
nvidia-smi
nvidia-ctk
docker
python3
lspci
ip
ethtool
rdma
nvcc
```

`echo`、`sleep`、`sh`、`bash` 仅可通过 Go helper-process 测试替代，不进入 production allowlist。

不得有 API：

```go
RunArbitraryCommand(userString string)
```

### 非 root 策略

基本命令：

```bash
aistat check
```

必须以普通用户可运行。

权限不足：

```json
{
  "state": "unknown",
  "reason": "permission_denied"
}
```

不能：

```text
panic
FAIL because permission missing
automatically sudo
```

Docker socket 权限不足也应区分：

```text
Docker binary installed
Daemon state UNKNOWN
Reason permission_denied
```

### 无网络保证

Runtime 中禁止：

```text
HTTP client
automatic update checks
telemetry
dependency downloading
model downloads
package repository queries
```

测试增加：

```text
TestNoNetworkRequired
```

可以通过 CI network namespace 或 mocked transport 强化。

### 性能预算

产品目标：

```text
Typical cold check        < 5s
Hard soft-budget          < 10s
Global collection timeout  10s default

nvidia-smi query          <= 3s
nvidia topology           <= 3s
docker                    <= 3s
python probe              <= 5s
other optional commands   <= 2s

Collector concurrency     <= 8 by default
Runtime memory goal       < 100 MiB typical
Binary goal               < 20–30 MiB
```

这些是**项目验收目标**，不是外部事实。

Collector 独立并发：

```text
system ─┐
cpu ────┤
memory ─┤
numa ───┤
pci ────┼── normalize
network ┤
storage ┤
nvidia ─┤
docker ─┘
```

但相同工具不要重复启动，例如 NVIDIA Collector 内集中执行最少次数的 `nvidia-smi`。

### Golden Test

```go
func TestCheckGolden(t *testing.T) {
	snapshot := loadSnapshot(t, "testdata/snapshots/docker-vllm.json")

	got := renderCheck(t, snapshot)
	golden := "testdata/golden/check-vllm-warn.txt"

	if *update {
		os.WriteFile(golden, []byte(got), 0o644)
		return
	}

	want := mustRead(t, golden)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("golden mismatch (-want +got):\n%s", diff)
	}
}
```

若希望 production dependencies 最少，测试依赖可以接受 `go-cmp`；若坚持零第三方依赖，也可以用标准库比较。

### Parser Tests

例：

```text
TestParseNvidiaSMIQuery
TestParseNvidiaTopologyGPU
TestParseNvidiaTopologyNIC
TestParsePCILinkWidth
TestParseNUMACPUList
TestParseCgroupCPUSet
TestParseDockerInspect
TestParseVLLMArgs
TestParseSGLangArgs
```

每个 parser 必须至少包含：

```text
normal
missing field
new unknown field
malformed
empty
```

### Fuzz

优先：

```go
func FuzzParseNvidiaSMI(f *testing.F)
func FuzzParsePCI(f *testing.F)
func FuzzParseCPUList(f *testing.F)
func FuzzParseVLLMArgs(f *testing.F)
```

### CI

GitHub Actions 适合在 PR/push 自动 build/test；GitHub 官方 Go workflow 指南推荐 `setup-go` 管理固定 Go 工具链。citeturn6view7

**`.github/workflows/ci.yml`**

```yaml
name: ci

on:
  pull_request:
  push:
    branches: [main]

permissions:
  contents: read

jobs:
  linux:
    runs-on: ubuntu-24.04

    steps:
      - uses: actions/checkout@v7

      - uses: actions/setup-go@v7
        with:
          go-version-file: go.mod
          cache: true

      - name: Format
        run: test -z "$(gofmt -l .)"

      - name: Vet
        run: go vet ./...

      - name: Test
        run: go test -race -coverprofile=coverage.out ./...

      - name: Build amd64
        run: |
          CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
          go build -trimpath -o dist/aistat-linux-amd64 ./cmd/aistat

      - name: Build arm64
        run: |
          CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
          go build -trimpath -o dist/aistat-linux-arm64 ./cmd/aistat

      - name: Staticcheck
        run: |
          go install honnef.co/go/tools/cmd/staticcheck@2026.1
          staticcheck ./...

      - name: CPU-only CLI smoke
        run: ./dist/aistat-linux-amd64 check --format json

  windows-logic:
    runs-on: windows-latest

    steps:
      - uses: actions/checkout@v7

      - uses: actions/setup-go@v7
        with:
          go-version-file: go.mod
          cache: true

      - name: Test portable logic and platform stubs
        run: go test ./...
```

Staticcheck 固定为支持 Go 1.26 的 `2026.1`，禁止使用 `latest`。升级 lint 工具必须是显式依赖更新。Staticcheck 官方将其定位为 Go 静态分析工具，可检测 correctness、performance 和 simplification 问题。citeturn4search1turn4search5

Golangci-lint 建议增加独立 job，并固定 minor/patch。其官方 CI 文档明确建议使用特定版本以保持 reproducibility，并推荐 GitHub Action。citeturn8view0

### GoReleaser

`.goreleaser.yaml`：

```yaml
version: 2

project_name: aistat

builds:
  - id: aistat
    main: ./cmd/aistat
    binary: aistat

    env:
      - CGO_ENABLED=0

    goos:
      - linux

    goarch:
      - amd64
      - arm64

    flags:
      - -trimpath

    ldflags:
      - >-
        -s -w
        -X github.com/ORG/aistat/internal/version.Version={{.Version}}
        -X github.com/ORG/aistat/internal/version.Commit={{.Commit}}

archives:
  - formats:
      - tar.gz

checksum:
  name_template: checksums.txt

changelog:
  use: git
```

GoReleaser 官方支持配置 Linux/GOARCH 构建矩阵和 `CGO_ENABLED=0`；其当前 GitHub Actions 文档使用 `goreleaser/goreleaser-action@v7`。citeturn6view6turn9search0

**release workflow：**

```yaml
name: release

on:
  push:
    tags:
      - "v*"

permissions:
  contents: write
  id-token: write
  attestations: write
  artifact-metadata: write

jobs:
  release:
    runs-on: ubuntu-24.04

    steps:
      - uses: actions/checkout@v7
        with:
          fetch-depth: 0

      - uses: actions/setup-go@v7
        with:
          go-version-file: go.mod

      - uses: goreleaser/goreleaser-action@v7
        with:
          distribution: goreleaser
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}

      - name: Attest release artifacts
        uses: actions/attest@v4
        with:
          subject-checksums: dist/checksums.txt
```

GoReleaser 官方目前指出 release workflow 需要完整 history，并要求 GitHub release 上传具有 `contents: write` 权限。citeturn9search0

Artifact attestation 需要 `id-token: write`、`attestations: write` 与当前 action 所需的 `artifact-metadata: write`。若仓库是当前 GitHub 方案不支持 attestation 的 private/internal repository，AISTAT-026 必须记录这一外部限制；checksums 仍是强制交付物，不能因 attestation 不可用而省略。

### install.sh

用户不需要 Go。

```bash
#!/usr/bin/env sh

set -eu

REPO="${AISTAT_REPO:-ORG/aistat}"
VERSION="${AISTAT_VERSION:-latest}"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

[ "$OS" = "linux" ] || {
  echo "AIStat currently supports Linux only." >&2
  exit 1
}

case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *)
    echo "Unsupported architecture: $ARCH" >&2
    exit 1
    ;;
esac

if [ "$VERSION" = "latest" ]; then
  VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" |
    sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' |
    head -n1)"
fi

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT INT TERM

ARCHIVE="aistat_${VERSION#v}_linux_${ARCH}.tar.gz"
BASE="https://github.com/${REPO}/releases/download/${VERSION}"

curl -fsSL "${BASE}/${ARCHIVE}" -o "${TMP}/${ARCHIVE}"
curl -fsSL "${BASE}/checksums.txt" -o "${TMP}/checksums.txt"

(
  cd "$TMP"
  grep " ${ARCHIVE}\$" checksums.txt | sha256sum -c -
)

tar -xzf "${TMP}/${ARCHIVE}" -C "$TMP"

if [ -w /usr/local/bin ]; then
  DEST="/usr/local/bin/aistat"
else
  mkdir -p "${HOME}/.local/bin"
  DEST="${HOME}/.local/bin/aistat"
fi

install -m 0755 "${TMP}/aistat" "$DEST"

echo "Installed AIStat to ${DEST}"
"${DEST}" version
```

说明：安装阶段需要网络；**AIStat 正常运行阶段不需要网络**。

## 可交付物、验收标准与 Definition of Done

### M0 Deliverables

```text
[ ] go.mod
[ ] CLI dispatcher
[ ] version package
[ ] model package
[ ] collector interface
[ ] execx safe runner
[ ] injected fsx + clock abstractions
[ ] JSON report skeleton
[ ] JSON schema 0.1 skeleton
[ ] Makefile
[ ] scripts/dev.ps1
[ ] CI
[ ] AGENTS.md
```

验收：

```bash
go test ./...
go vet ./...
go build ./cmd/aistat
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/aistat
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build ./cmd/aistat
```

Linux CI 另外执行 `aistat version` 与 CPU-only `aistat check --format json`；Windows 本地只 cross-build Linux binary，不直接运行。

### M1 Deliverables

```text
[ ] system collector
[ ] cpu collector
[ ] memory collector
[ ] numa collector
[ ] pci collector
[ ] network collector
[ ] storage collector
[ ] fixture tests
```

验收：

```text
CPU-only Linux server:
PASS program execution
NO NVIDIA dependency
NO Docker dependency
NO panic
valid JSON snapshot
```

### M2 Deliverables

```text
[ ] NVIDIA PCI detection
[ ] GPU inventory
[ ] nvidia-smi parser
[ ] GPU UUID↔BDF
[ ] NUMA affinity
[ ] GPU topology
[ ] GPU P2P matrix
[ ] GPU/NIC topology
[ ] PCIe capability/state
[ ] Xid normalized event collection
```

验收：

```text
Graph can answer:
GPU0 NUMA?
GPU0 local CPUs?
GPU0↔GPU1 path?
GPU0 nearest NIC?
```

### M3 Deliverables

```text
[ ] driver model
[ ] CUDA model
[ ] versioned embedded compatibility dataset
[ ] active CUDA runtime model
[ ] NCCL detection
[ ] Docker detection
[ ] NCT detection
[ ] PyTorch probe
[ ] stack command
```

验收：

```bash
aistat stack
aistat stack --format json
```

输出必须区分：

```text
Driver-supported CUDA
Installed CUDA Toolkit
PyTorch CUDA Build
```

### M4 Deliverables

```text
[ ] process detector
[ ] safe env reader
[ ] command-line redaction
[ ] CPU affinity parser
[ ] effective memory-node parser
[ ] Docker container mapping
[ ] vLLM parser
[ ] SGLang parser
```

验收：

给 fixture：

```text
vLLM PID
CUDA_VISIBLE_DEVICES=0,1,2,3
TP=4
cpuset=32-63
```

得到标准化 runtime object。

### M5 Deliverables

```text
[ ] graph node types
[ ] graph edges
[ ] graph builder
[ ] topology distance
[ ] locality helpers
[ ] PCI root/bridge resolver
[ ] GPU-set strict dominance comparator
[ ] process/container/GPU relationship resolver
```

硬性验收：

```text
grep rules/ for:
  /proc
  /sys
  exec.Command
```

结果应该为 **0**。

### M6 Deliverables

```text
[ ] Rule registry
[ ] 本文冻结的 25 rules，ID 与语义完全一致
[ ] all 25 rule tests: trigger/pass/unknown/skip
[ ] explain command
[ ] profile system
[ ] readiness summary
[ ] human + JSON reporters from the same report model
```

默认输出示意：

```text
AIStat v0.1

NVIDIA AI Node Check

Hardware               PASS
NUMA                    WARN
PCIe                    PASS
NVIDIA                  PASS
CUDA                    PASS
Docker                  FAIL
PyTorch                 PASS
vLLM                    WARN

Deployment Readiness    NOT READY
Performance Readiness   WARN

1 blocker
2 warnings

FAIL CTR002
NVIDIA Container Toolkit was not detected.

Evidence
...

Why
...

Recommendation
...

Verification
...
```

### M7 Deliverables

```text
[ ] README
[ ] docs/architecture.md
[ ] docs/rules.md
[ ] docs/security.md
[ ] README contribution section
[ ] SECURITY.md
[ ] LICENSE
[ ] AGENTS.md
[ ] fixtures
[ ] golden tests
[ ] fuzz tests
[ ] Windows portable-logic CI
[ ] CPU-only Linux live smoke
[ ] real NVIDIA Linux validation record
[ ] CI
[ ] release workflow
[ ] GoReleaser
[ ] install.sh
[ ] SHA256 checksums
[ ] artifact provenance/attestation
[ ] linux/amd64 binary
[ ] linux/arm64 binary
```

### 全局 Definition of Done

v0.1 发布前必须全部满足：

| 项目 | 标准 |
|---|---|
| Platform | Linux amd64/arm64 |
| Accelerator | NVIDIA only |
| Installation | 单命令安装 |
| Runtime dependencies | 无强制语言 runtime |
| Go required for user | 否 |
| Root | 基础检查不需要 |
| Network | 运行检查不需要 |
| Mutation | 0 |
| Daemon | 0 |
| Inventory | CPU/Memory/NUMA/PCI/GPU/NIC/Storage |
| NVIDIA Stack | Driver/CUDA/NCCL |
| Container | Docker/NVIDIA Container Toolkit |
| Framework | PyTorch/vLLM/SGLang |
| Topology | CPU↔NUMA↔PCI↔GPU↔NIC |
| Rules | ≥25 |
| JSON | 稳定 schema |
| Secrets | 不采集 arbitrary env |
| Command execution | 无 shell |
| Typical runtime | `<5s` 目标 |
| Build | release binaries 必须 `CGO_ENABLED=0` |
| Binary | `<20–30 MiB` 目标 |
| Tests | Windows portable + Linux CPU-only + unit/fixture/rule/golden/fuzz |
| Release | GHA + GoReleaser |
| Real GPU | 发布前至少一轮 NVIDIA Linux integration validation |

### Exit Code

```text
0 = execution successful, no FAIL finding
1 = check completed, one or more FAIL findings
2 = AIStat internal execution failure
```

WARN 不影响默认 exit code。

### AGENTS.md 核心内容

```md
# AIStat Agent Engineering Rules

## Product
AIStat v0.1 is a read-only Linux NVIDIA AI infrastructure inspector.

## Scope
Do not implement monitoring daemons, tuning, mutation, cluster support,
AMD, Intel GPU, Ascend, Kubernetes, or benchmarks.

## Architecture
Collector -> Facts -> Normalizer -> Snapshot -> Graph -> Rules -> Report.

## Collector Rules
Collectors collect facts only.
Collectors must never emit recommendations.
Collectors must never modify the host.

## Rule Rules
Rules consume Snapshot + Graph + Profile only.
Rules must never read /proc or /sys directly.
Rules must never execute external commands.

## Security
Never serialize arbitrary environment variables.
Never serialize raw /proc/PID/environ.
Never retain full raw process command lines.
Use allowlists.
Never use sh -c or bash -c.
Every external command requires timeout and output-size limits.

## Tests
Every parser requires fixture tests.
Every rule requires PASS, trigger, UNKNOWN and SKIP tests.
Never require NVIDIA hardware in normal CI.
Windows must run portable logic tests; Linux-specific code uses build-tagged implementations and honest unsupported stubs.
Never use shell builtins or Unix utilities as cross-platform test helpers.

## Compatibility
Unknown fields must be tolerated.
Missing optional tools must degrade to UNKNOWN/NOT_DETECTED, not crash.
Permission denied, timeout, parse error and unsupported must remain distinguishable.

## Change Policy
Do not introduce production dependencies without explicit justification.
Do not change JSON schema silently.
```

## AI 编码代理执行协议与 Codex 任务卡

AI 代理应该严格按以下顺序工作：

```text
Foundation
   ↓
Fact model
   ↓
Safe runner
   ↓
Host collectors
   ↓
NVIDIA
   ↓
Software stack
   ↓
Runtime
   ↓
Topology
   ↓
Rules
   ↓
Reporter
   ↓
Hardening
```

**禁止 Codex 同时“顺手”实现 monitoring、auto-fix、benchmark 等未来能力。**

每完成一个任务，代理必须：

```text
1. 修改指定文件
2. 添加/更新测试
3. 运行要求的 go test
4. 运行 gofmt
5. 不修改 scope 外文件，除非编译所需
6. 输出：
   changed files
   tests run
   remaining TODO
```

以下 26 张任务卡可以直接复制给 Codex；AISTAT-001..020 建立核心，AISTAT-021..026 完成产品收口与发布门。

**Task Card — Foundation**

```text
[P0] AISTAT-001 — 初始化 Go 项目与 CLI 入口

目标：
创建 AIStat v0.1 最小可编译项目。不要实现任何 collector。

文件：
- go.mod
- cmd/aistat/main.go
- internal/app/app.go
- internal/version/version.go
- internal/app/app_test.go

要求：
- Module 使用执行门确认后的正式 path；禁止提交 ORG 占位符。
- `go 1.26.0` + `toolchain go1.26.5`。
- app.Run(ctx context.Context, args []string, stdout, stderr io.Writer) error
- 支持：
  aistat version
  aistat check
- check 暂时输出空 report。
- 不使用 Cobra。

示例：
$ aistat version
AIStat dev

测试：
go test ./...
go vet ./...
go build ./cmd/aistat

完成条件：
全部命令成功。
```

**Task Card — Core Model**

```text
[P0] AISTAT-002 — 实现 Fact 与 Source 数据模型

文件：
- internal/model/fact.go
- internal/model/types.go
- internal/model/fact_test.go

实现：
type Fact
type Subject
type SourceRef
type FactState
type Confidence
type Diagnostic

要求：
- JSON tags 完整。
- 支持 present/not_detected/unknown/error。
- 提供 NewFact helper，接受任意可 JSON marshal 的 value。
- 不允许 Fact 内保存 error object。

示例输入：
subject=gpu/GPU-0
field=pci_bdf
value="0000:31:00.0"

示例输出：
合法 JSON Fact。

测试：
go test ./internal/model/...
```

**Task Card — Safe Command Runner**

```text
[P0] AISTAT-003 — 实现安全外部命令 Runner

文件：
- internal/execx/runner.go
- internal/execx/resolver.go
- internal/execx/runner_test.go

签名：
type Runner interface {
    Run(context.Context, CommandSpec) (CommandResult, error)
}

要求：
- exec.CommandContext
- 禁止 shell
- allowlisted executable
- timeout
- stdout/stderr 最大长度
- 不读取 stdin
- 返回 exit code + duration
- timeout 要有 typed error

测试场景：
- Go helper-process/success（不依赖 echo/sh/bash）
- nonzero exit
- timeout
- oversized output
- command not allowlisted
- sanitized environment
- child process cleanup

测试：
go test ./internal/execx/... -race
```

**Task Card — Collector Framework**

```text
[P0] AISTAT-004 — 实现 Collector 接口、Registry 与并行执行

文件：
- internal/collector/collector.go
- internal/collector/registry.go
- internal/collector/run.go
- internal/collector/run_test.go

接口：
type Collector interface {
    ID() ID
    Provides() []Capability
    Requires() []Capability
    Collect(context.Context, Env) Result
}

函数：
func RunAll(ctx context.Context, env Env, collectors []Collector) []Result

要求：
- 独立 collectors 并发。
- 使用注入的 Runner/FileSystem/Clock。
- 按 capability DAG 调度，默认并发上限 8。
- deterministic result ordering by collector name。
- collector failure 不影响其他 collector。
- context cancellation 正常停止。

测试：
go test ./internal/collector/... -race
```

**Task Card — System Collector**

```text
[P0] AISTAT-005 — 实现 system collector

文件：
- internal/collector/system/system.go
- internal/collector/system/system_test.go
- testdata/fixtures/linux/os-release/*
- testdata/fixtures/linux/dmi/*

采集：
hostname
os id/version
kernel
architecture
uptime
BIOS vendor/version best effort

数据源：
/etc/os-release
/proc
/sys/class/dmi

要求：
- 支持 injected filesystem root 以便 fixture test。
- permission denied -> unknown，不 crash。

测试：
go test ./internal/collector/system/...
```

**Task Card — CPU Collector**

```text
[P0] AISTAT-006 — 实现 CPU topology collector

文件：
- internal/collector/cpu/cpu.go
- internal/collector/cpu/parser.go
- internal/collector/cpu/cpu_test.go
- testdata/fixtures/linux/cpu/*

采集：
vendor/model
logical CPUs
socket/core/thread topology
SMT
cache
governor
frequency best effort

优先使用：
/sys/devices/system/cpu

要求：
- 不强依赖 lscpu。
- CPU ranges parser 独立函数：
  func ParseCPUList(string) ([]int, error)

测试：
go test ./internal/collector/cpu/...
```

**Task Card — Memory Collector**

```text
[P0] AISTAT-007 — 实现 memory collector

文件：
- internal/collector/memory/memory.go
- internal/collector/memory/memory_test.go
- testdata/fixtures/linux/memory/*

采集：
MemTotal
MemAvailable
SwapTotal
SwapFree
HugePages
HugePageSize
THP state
NUMA balancing state
memlock best effort

注意：
collector 只输出事实，不判断 THP 好坏。

测试：
go test ./internal/collector/memory/...
```

**Task Card — NUMA Collector**

```text
[P0] AISTAT-008 — 实现 NUMA collector

文件：
- internal/collector/numa/numa.go
- internal/collector/numa/numa_test.go
- testdata/fixtures/linux/numa/*

采集：
node IDs
cpulist
memory total
distance matrix

关键函数：
func ParseDistance(string) ([]uint32, error)

输出：
NUMA-related Facts。

测试：
单 NUMA
双 NUMA
缺失 distance
offline node

命令：
go test ./internal/collector/numa/...
```

**Task Card — PCI Collector**

```text
[P0] AISTAT-009 — 实现 PCI sysfs collector

文件：
- internal/collector/pci/pci.go
- internal/collector/pci/link.go
- internal/collector/pci/pci_test.go
- testdata/fixtures/linux/pci/*

采集：
BDF
vendor
device
class
numa_node
parent bridge
current/max link width
current/max link speed

要求：
- sysfs 为主要来源。
- lspci 只能作为 optional enrichment。
- normalize BDF 为 canonical 0000:bb:dd.f。

测试：
go test ./internal/collector/pci/...
```

**Task Card — Network Collector**

```text
[P0] AISTAT-010 — 实现 network / RDMA inventory

文件：
- internal/collector/network/network.go
- internal/collector/network/rdma.go
- internal/collector/network/network_test.go

采集：
interface
operstate
MTU
speed best effort
driver best effort
PCI BDF
NUMA
RDMA mapping best effort

优先：
/sys/class/net
/sys/class/infiniband

外部命令：
ip/ethtool/rdma optional only

测试：
go test ./internal/collector/network/...
```

**Task Card — NVIDIA GPU Inventory**

```text
[P0] AISTAT-011 — 实现 NVIDIA GPU inventory

文件：
- internal/collector/nvidia/nvidia.go
- internal/collector/nvidia/query.go
- internal/collector/nvidia/query_test.go
- testdata/fixtures/nvidia/query/*

采集：
GPU index
UUID
model
PCI BDF
VRAM
driver
compute mode
MIG
ECC
power limit
default power limit
temperature/utilization snapshot
volatile uncorrectable ECC
recent normalized Xid events best effort

要求：
- 通过 Safe Runner 调用 nvidia-smi。
- 一次 query 获取尽量多字段。
- nvidia-smi 不存在 -> not_detected/unknown，不能 panic。

测试：
go test ./internal/collector/nvidia/...
```

**Task Card — NVIDIA Topology**

```text
[P0] AISTAT-012 — 实现 NVIDIA topology parser

文件：
- internal/collector/nvidia/topology.go
- internal/collector/nvidia/topology_test.go
- testdata/fixtures/nvidia/topology/*

解析：
nvidia-smi topo -gpu
nvidia-smi topo -nic
nvidia-smi topo -cpu 或 topo -all
nvidia-smi topo -p2p 或等价受支持 P2P 输出

支持 path：
NV#
PIX
PXB
PHB
NODE
SYS
X

输出：
结构化 connection facts。

要求：
未知未来 token 不 crash，应记录 UNKNOWN path。

测试：
go test ./internal/collector/nvidia/... -run Topology
```

**Task Card — Snapshot Normalizer**

```text
[P0] AISTAT-013 — 从 Facts 构建 typed Snapshot

文件：
- internal/model/snapshot.go
- internal/normalize/normalize.go
- internal/normalize/normalize_test.go

签名：
func Build(results []collector.Result) (*model.Snapshot, error)

要求：
- 将 GPU UUID 作为稳定 GPU identity。
- PCI BDF canonicalize。
- missing fact 与 zero value 明确区分。
- collector diagnostic 保存到 Snapshot.Collection。
- Meta/schema/profile/compatibility version 始终存在。
- Process/Container/RDMA/Network/Storage 使用正式 typed sections。

示例：
Fact gpu.GPU-0.pci_bdf
→ Snapshot.GPUs[...].PCIBDF

测试：
go test ./internal/normalize/...
```

**Task Card — Topology Graph**

```text
[P0] AISTAT-014 — 实现统一 Topology Graph

文件：
- internal/topology/graph.go
- internal/topology/builder.go
- internal/topology/distance.go
- internal/topology/graph_test.go

实现：
Node
Edge
Graph
Build(snapshot)
Neighbors
Distance
LocalNUMA

必须建立：
NUMA -> CPU
PCI root -> bridge -> device
PCI -> NUMA
GPU -> PCI
GPU -> NUMA
NIC -> PCI
NIC -> NUMA
GPU <-> GPU connection
GPU <-> GPU P2P state
Process -> Container/CPU/GPU

禁止：
访问 filesystem
执行任何命令

测试：
go test ./internal/topology/...
```

**Task Card — Docker Collector**

```text
[P0] AISTAT-015 — 实现 Docker 与 NVIDIA Container Toolkit collector

文件：
- internal/collector/docker/docker.go
- internal/collector/docker/container.go
- internal/collector/docker/nvidia.go
- internal/collector/docker/docker_test.go
- testdata/fixtures/docker/*

采集：
docker binary/version
daemon reachable
containers
cgroup mode
cpuset
cpuset.mems
memory limit
shm
memlock
GPU request
NVIDIA Container Toolkit
nvidia-ctk presence/config best effort

要求：
Docker 未安装不是 internal error。
Docker socket permission denied 必须能区分。

测试：
go test ./internal/collector/docker/...
```

**Task Card — PyTorch Probe**

```text
[P0] AISTAT-016 — 实现安全 PyTorch CUDA probe

文件：
- internal/collector/pytorch/pytorch.go
- internal/collector/pytorch/probe.go
- internal/collector/pytorch/pytorch_test.go
- testdata/fixtures/pytorch/*

probe 输出：
torch version
torch.version.cuda
torch.cuda.is_available()
torch.cuda.device_count()

要求：
- python3 optional。
- timeout <=5s 默认。
- 使用 fixed probe、isolated flags、最小环境、空 stdin 与受控 cwd。
- 不下载。
- 不 import 用户代码。
- 不加载 model。
- Python probe JSON 输出必须严格解析。

测试：
go test ./internal/collector/pytorch/...
```

**Task Card — Runtime Privacy Layer**

```text
[P0] AISTAT-017 — 实现 process discovery、环境变量 allowlist 与 redaction

文件：
- internal/runtime/process.go
- internal/runtime/environ.go
- internal/redact/redact.go
- internal/runtime/process_test.go
- internal/redact/redact_test.go

要求：
- 读取 /proc/PID/cmdline 后只提取 allowlisted flags。
- 不保存 raw cmdline。
- /proc/PID/environ 只返回 allowlisted env。
- 测试中加入 HF_TOKEN/AWS_SECRET_ACCESS_KEY/API_KEY，确保绝不输出。

函数：
func ReadAllowedEnv(fsys fsx.FileSystem, pid int, allowed map[string]struct{}) (map[string]string, error)
func ParseAllowedArgs(argv []string, allowed map[string]ArgSpec) map[string]string

测试：
go test ./internal/runtime/... ./internal/redact/...
```

**Task Card — vLLM Runtime Detector**

```text
[P0] AISTAT-018 — 实现 vLLM runtime detector

文件：
- internal/collector/vllm/vllm.go
- internal/collector/vllm/args.go
- internal/collector/vllm/vllm_test.go
- testdata/fixtures/vllm/*

识别：
vllm serve
python -m vllm...

解析：
tp
pp
dp
device-ids
CUDA_VISIBLE_DEVICES
NUMA CPU binding
distributed backend
nnodes

输出：
model.RuntimeInstance

要求：
未知参数忽略。
secret-looking unknown args 不保留。
只消费注入的 process facts/source，不读取 Windows 本机进程作为 Linux fixture 的替代。

测试：
go test ./internal/collector/vllm/...
```

**Task Card — SGLang Runtime Detector**

```text
[P0] AISTAT-019 — 实现 SGLang runtime detector

文件：
- internal/collector/sglang/sglang.go
- internal/collector/sglang/args.go
- internal/collector/sglang/sglang_test.go
- testdata/fixtures/sglang/*

识别：
sglang serve
python -m sglang.launch_server

提取：
model path sanitized
tensor parallel
data/distributed config
selected device information
CPU affinity inherited from process model

要求：
不保存完整 raw command。
不保存 arbitrary env。
只消费注入的 process facts/source，不直接依赖开发机 OS。

测试：
go test ./internal/collector/sglang/...
```

**Task Card — Rule Engine + Representative Rules**

```text
[P0] AISTAT-020 — 实现 Rule Engine 并验证代表性 Rules

文件：
- internal/model/finding.go
- internal/rules/rule.go
- internal/rules/engine.go
- internal/rules/registry.go
- internal/rules/gpu/
- internal/rules/pcie/
- internal/rules/numa/
- internal/rules/container/
- internal/rules/cuda/
- internal/rules/pytorch/
- internal/rules/nccl/
- internal/rules/vllm/
- internal/rules/sglang/

接口：
type Rule interface {
    ID() RuleID
    Meta() RuleMeta
    Evaluate(RuleContext) []model.Finding
}

首批实现以下代表性规则，用于证明数据契约可以跨域闭环：
GPU001
NUMA001
NET001
CTR001
CUDA001
VLLM001

要求：
- Rules 只能使用 Snapshot + Graph + Profile。
- 本任务每条规则至少：
  trigger test
  pass test
  missing-data test
- Finding 必须包含：
  Evidence
  Why
  Recommendation
  Verification
  Confidence
  References

禁止：
/proc
/sys
exec.Command
host mutation

测试：
go test ./internal/rules/... -race
go test ./...
```

完成这二十张核心任务卡后，AIStat 只达到 **Core Complete**，还不满足 V0.1 Definition of Done。必须继续完成下面的收口任务，才能进入 release candidate。

**Task Card — Storage Completion**

```text
[P1] AISTAT-021 — 完成 Storage inventory 与 topology mapping

实现：
- block device inventory
- filesystem/mount/free capacity
- NVMe/PCI BDF/NUMA best effort
- mountinfo/path parsing and redaction

测试：
- Windows fixture/parser tests
- Linux fixture tests
- malformed/missing/permission cases
```

**Task Card — NVIDIA Health and Advanced Topology**

```text
[P0] AISTAT-022 — 补齐 P2P、Xid、PCI root/bridge 与 ACS facts

实现：
- GPU P2P matrix
- recent normalized Xid event model with injected Now
- PCI root/bridge hierarchy
- sanitized lspci ACS enrichment best effort
- GPU/NIC shared-root resolution

要求：
- 无 journal 权限时 UNKNOWN，不误报 PASS。
- 未支持的新平台/未知 topology token gracefully degrade。
- 不自动修改 ACS/IOMMU/driver 配置。
```

**Task Card — Compatibility Data and Stack**

```text
[P0] AISTAT-023 — 完成 Driver/CUDA/NCCL/NCT compatibility layer

实现：
- versioned, source-attributed embedded compatibility dataset
- driver lifecycle dataset
- installed Toolkit detection
- active runtime/framework CUDA build
- forward/minor compatibility decision model
- NCCL/NCT detection

验收：
- supported/incompatible/unknown/compat-package fixtures
- multiple Toolkit 不自动等于 failure
- runtime 不联网更新 compatibility data
```

**Task Card — Frozen 25 Rules**

```text
[P0] AISTAT-024 — 完成本文冻结的全部 25 条 Rules

唯一清单：
GPU001 GPU002 GPU003 GPU004
PCIE001 PCIE002
NUMA001 NUMA002 TOPO001
NET001 NET002 NET003 NCCL001
CUDA001 CUDA002
TORCH001 TORCH002
CTR001 CTR002 CTR003 CTR004
VLLM001 VLLM002
SGL001 SGL002

每条必须：
- trigger/pass/unknown/skip tests
- evidence/impact/recommendation/verification/references
- deterministic output order
- no I/O

复杂规则额外包含 false-positive regression case。
```

**Task Card — Report, Readiness and JSON Contract**

```text
[P0] AISTAT-025 — 完成 Human/JSON Reporter、Readiness 与 Explain

实现：
- one canonical Report model
- Human Reporter
- JSON Reporter
- JSON Schema 0.1
- Deployment READY/NOT_READY/UNKNOWN
- Performance READY/WARN/UNKNOWN
- explain RULE_ID
- --fail-on fail|warn

要求：
- JSON enum lowercase；Human status uppercase。
- Reporter 不重新计算 Rule。
- UNKNOWN 不计为 PASS。
- schema validation + human/json golden tests。
```

**Task Card — Cross-platform Hardening and Release**

```text
[P0] AISTAT-026 — 完成 V0.1 hardening 与 release pipeline

必须完成：
- Windows portable-logic CI
- Ubuntu CPU-only full CI and CLI smoke
- parser fuzzing
- timeout/process-tree/output-limit tests
- secret/redaction review
- linux/amd64 + linux/arm64 cross-build
- GoReleaser snapshot and tagged release dry run
- checksum + artifact provenance
- installer verification
- at least one real NVIDIA Linux integration run

只有 AISTAT-001..026 全部达到各自验收条件，才允许 tag v0.1.0。
```
