# AIStat

[English](README.md) | [简体中文](README.zh-CN.md)

**面向大模型部署与高性能推理的 AI Infra 检查、诊断与优化工具。**

AIStat 用于统一分析 Linux NVIDIA 节点的硬件拓扑、CUDA 软件栈、容器和推理运行时，发现大模型部署中的阻塞项与性能瓶颈，解释问题背后的证据，并给出工程师能够验证的优化建议。

AIStat 的最终目标不是再做一个 GPU 信息查询命令，而是成为 GPU 服务器的大模型部署与推理优化系统：先理解节点每一层的真实状态，再诊断跨层问题、制定优化方案，并验证调整是否真正改善了推理工作负载。

> **看懂节点，诊断全栈，优化推理链路。**

## 项目状态

AIStat 正在准备 `v0.1.0`：

- Windows 可移植逻辑测试和 Linux fixture 测试已通过。
- Ubuntu CPU-only GitHub Actions、race 测试、静态分析、发布交叉构建、安装器模拟和 GoReleaser snapshot 已通过。
- 发布 `v0.1.0` 前仍需至少完成一次真实 NVIDIA Linux 集成验证。
- 当前尚未发布稳定版本标签。

准确的验证证据和剩余发布门槛见 [验证状态](docs/validation.md)。

## 产品愿景

```text
检查 -> 建模 -> 诊断 -> 建议 -> 验证 -> 优化
```

AIStat 将围绕一条持续演进的优化闭环建设：

1. **检查：**从硬件、Linux、NVIDIA、容器和推理运行时采集可信事实。
2. **建模：**把 CPU、NUMA、PCIe、GPU、NIC/RDMA、CUDA、容器和进程连接成统一的拓扑感知节点模型。
3. **诊断：**发现部署阻塞、版本兼容问题、资源放置错误和性能瓶颈。
4. **建议：**依据事实和运行上下文生成优化方案，而不是输出泛化的“调优最佳实践”。
5. **验证：**对比调整前后的关键事实与工作负载结果，确认优化是否有效。
6. **优化：**逐步发展为面向大模型推理 GPU 服务器的可控、可审计优化工作流。

当前 `v0.1` 是这套长期能力的只读基础层，重点是节点查询、统一建模、拓扑关系、运行时上下文和保守诊断；它目前**不宣称已经具备自动调优或自主修改主机的能力**。

## 当前 v0.1 基础能力

| 层级 | 检查范围 |
|---|---|
| 主机 | OS、内核、架构、CPU 拓扑/缓存/频率、内存、HugePages、NUMA |
| I/O 拓扑 | PCIe 层级和链路、GPU 到 NUMA、GPU 到 GPU、GPU 到 NIC/RDMA、存储 |
| NVIDIA 软件栈 | GPU 清单与健康、Driver、Driver 支持的 CUDA、已安装 CUDA Toolkit、NCCL、Xid 事件 |
| 容器 | Docker、NVIDIA Container Toolkit、cgroup/cpuset/内存、共享内存、GPU 可见性 |
| AI Runtime | 进程亲和性、PyTorch CUDA 探针、vLLM 和 SGLang 的资源与并行配置 |
| 工程诊断 | 部署就绪性、性能就绪性及 25 条基于证据的规则 |

冻结的 25 条规则见 [规则目录](docs/rules.md)。

## 支持范围

- Linux `amd64` 和 `arm64`
- NVIDIA GPU
- Docker 与 NVIDIA Container Toolkit
- PyTorch、vLLM、SGLang 的 best-effort 发现
- 单节点检查
- 普通用户、只读运行

Kubernetes、Slurm、多节点清单、AMD/Intel 加速器、长期监控、性能 profiling、自动调优、主机修改和 benchmark 执行均不属于 v0.1。

## 从源码构建

项目固定使用 Go 1.26.5 工具链。

```bash
git clone https://github.com/arpingblue/AIStat.git
cd AIStat
go test ./...
CGO_ENABLED=0 go build -trimpath -o aistat ./cmd/aistat
./aistat version
```

Windows 开发命令：

```powershell
.\scripts\dev.ps1 test
.\scripts\dev.ps1 cross
```

交叉构建只生成 Linux 二进制，不代表已经验证真实 Linux 或 NVIDIA 硬件采集。

## 安装

当前还没有稳定二进制 Release。以后发布 `v0.1.0` 等版本后，可在 Linux 上使用带 SHA256 校验的安装脚本：

```bash
curl -fsSL https://raw.githubusercontent.com/arpingblue/AIStat/main/scripts/install.sh | \
  AISTAT_VERSION=v0.1.0 sh
```

安装过程需要网络；安装后的 `aistat` 正常检查不需要网络。

## 使用方法

不带子命令运行 `aistat` 等同于 `aistat check`。

```text
aistat check [--format human|json] [--profile general|llm-inference]
aistat info [--format human|json]
aistat topology [--view tree|gpu|gpu-nic] [--format human|json]
aistat stack [--format human|json]
aistat runtime [--format human|json]
aistat explain RULE_ID [--format human|json]
aistat version [--format human|json]
```

常用示例：

```bash
aistat check
aistat check --format json
aistat check --profile general --fail-on warn
aistat topology --view gpu-nic
aistat explain NUMA001
```

每次检查都有总超时限制，默认 `10s`。更严格的 CI 可以使用 `--fail-on warn`。

### 退出码

| 退出码 | 含义 |
|---:|---|
| `0` | 检查完成，且没有 FAIL Finding |
| `1` | 检查完成但存在 FAIL；或使用 `--fail-on warn` 时存在 WARN |
| `2` | 参数错误或内部执行错误 |

### 证据状态

缺失证据永远不会被当作成功。Collector 会保留 `available`、`not_detected`、`unsupported`、`permission_denied`、`timeout`、`parse_error` 和 `unknown` 等状态。Rule 因而可以诚实地返回 `pass`、`warn`、`fail`、`info`、`unknown` 或 `skip`，而不是猜测。

版本化 JSON 契约见 [数据模型](docs/data-model.md) 和 [JSON Schema 0.1](docs/schema/report-v0.1.schema.json)。

## 架构

```text
Collectors -> Facts -> Normalizer -> Snapshot -> Topology Graph
                                                    |
                                                    v
                                  Profile + Rules -> Report
```

- Collector 只采集事实和诊断，不输出修改建议。
- Rule 只能使用标准化 Snapshot、Topology Graph、Profile 和时钟。
- Reporter 只渲染统一 Report，不重新计算规则。

进一步说明见 [架构](docs/architecture.md)、[采集器](docs/collectors.md) 和 [拓扑](docs/topology.md)。

## 隐私与安全

AIStat 使用固定的可执行文件 allowlist，从不调用 shell，限制命令输出大小，并会终止超时的进程树。进程参数和环境变量使用严格 allowlist。原始命令行、任意环境变量、Token、Prompt、凭据、主机修改和遥测均被设计为禁止项。

参见 [安全设计](docs/security.md) 和 [安全策略](SECURITY.md)。

## 测试

```bash
go test ./...
go vet ./...
make fuzz       # 有时间上限的解析器模糊测试
```

普通 CI 不要求 NVIDIA 硬件。脱敏 fixtures 覆盖可移植解析器、数据模型、拓扑、规则、报告、隐私和失败状态。真实 NVIDIA 验证仍是人工发布门，完成后必须记录到 [docs/validation.md](docs/validation.md)。

## 贡献

请先阅读 [CONTRIBUTING.md](CONTRIBUTING.md) 和 [AGENTS.md](AGENTS.md)。规则变更必须包含 trigger、pass、unknown、skip、边界和适用的误报回归测试。

## License

[Apache License 2.0](LICENSE)。
