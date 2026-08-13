# AIStat

[English](README.md)

AIStat用于检查Linux NVIDIA服务器的GPU拓扑、CUDA软件栈、容器、AI运行时和部署问题。

它是一个只读的单二进制工具，帮助工程师在部署或排查PyTorch、vLLM、SGLang之前，快速看懂一台陌生GPU节点。

[![Release](https://img.shields.io/github/v/release/arpingblue/AIStat)](https://github.com/arpingblue/AIStat/releases/latest)
[![CI](https://github.com/arpingblue/AIStat/actions/workflows/ci.yml/badge.svg)](https://github.com/arpingblue/AIStat/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

## 安装

支持Linux `amd64`和 `arm64`。下面这条命令会自动下载最新版本，并安装到当前用户：

```bash
curl -fsSL https://raw.githubusercontent.com/arpingblue/AIStat/main/scripts/install.sh | sh
```

二进制会安装到 `~/.local/bin/aistat`。如果该目录不在 `PATH`中，安装器会输出需要执行的命令。

也可以指定版本和安装目录：

```bash
curl -fsSL https://raw.githubusercontent.com/arpingblue/AIStat/main/scripts/install.sh | \
  AISTAT_VERSION=v0.1.0 AISTAT_INSTALL_DIR=/your/bin sh
```

所有压缩包和校验文件也可以从 [Releases页面](https://github.com/arpingblue/AIStat/releases/latest)手动下载。

## 使用

直接运行 `aistat`查看节点总览：

```text
AIStat 0.1.0 — Node Status

Hardware
  GPUs       AVAILABLE 4 × NVIDIA L20
  CPU        AVAILABLE 2 sockets, 96 cores, 192 logical
  Memory     AVAILABLE 1007.4 GiB
  NUMA       AVAILABLE 2 nodes

NVIDIA Stack
  Driver     PASS      580.159.03
  CUDA       AVAILABLE driver capability 13.0; selected toolkit 13.0
  Xid log    PERMISSION DENIED kernel log could not be fully inspected

Containers
  Client     AVAILABLE docker 29.5.2
  Daemon     AVAILABLE 29.5.2
  NVIDIA CTK AVAILABLE 1.19.0

AI Runtimes
  PyTorch    installed=AVAILABLE running=NOT DETECTED instances=0
  vLLM       installed=AVAILABLE running=NOT DETECTED instances=0
  SGLang     installed=NOT DETECTED running=NOT DETECTED instances=0

Readiness
  Deployment  READY
  Performance UNKNOWN — runtime workload evidence is not available
```

常用命令：

```bash
aistat                             # 快速查看节点
aistat check                       # 详细诊断
aistat check --format json         # 完整JSON报告
aistat info                        # 硬件清单
aistat stack                       # NVIDIA与容器软件栈
aistat runtime                     # PyTorch、vLLM和SGLang
aistat topology --view gpu-nic     # GPU、网卡和NUMA拓扑
aistat explain CTR002              # 解释一条规则
```

## 检查范围

| 范围 | 采集和诊断内容 |
|---|---|
| 硬件 | CPU、内存、NUMA、PCIe、NVIDIA GPU、存储 |
| GPU拓扑 | GPU↔GPU P2P、GPU↔NUMA、GPU↔NIC/RDMA |
| NVIDIA软件栈 | 驱动、CUDA能力与Toolkit、NCCL、Xid可见性 |
| 容器 | Docker客户端/daemon、NVIDIA Container Toolkit、runtime/CDI、GPU请求 |
| AI运行时 | PyTorch、vLLM、SGLang安装位置和活跃进程 |
| 诊断 | 部署就绪度、性能就绪度、25条规则 |

AIStat会区分“没有安装”和“无法检查”。例如，Docker权限不足不会被说成“Docker未安装”，无法读取内核日志也不会被说成“没有Xid错误”。

## 命令说明

| 命令 | 用途 |
|---|---|
| `aistat` / `aistat status` | 简短节点总览和首要操作 |
| `aistat check` | Finding、证据、影响、建议和验证方法 |
| `aistat info` | 硬件资产清单 |
| `aistat topology` | 紧凑拓扑树和GPU P2P矩阵 |
| `aistat stack` | NVIDIA、CUDA、Docker和Container Toolkit |
| `aistat runtime` | 运行时安装位置和运行实例 |
| `aistat explain RULE_ID` | 规则详情 |
| `aistat version` | 版本和构建信息 |

所有命令都可以通过 `--timeout`限制执行时间。颜色只在终端中自动启用；JSON、重定向输出、`--no-color`和 `NO_COLOR`都不会包含ANSI字符。

退出码：

| 退出码 | 含义 |
|---:|---|
| `0` | 检查完成且没有FAIL |
| `1` | 存在FAIL，或使用 `--fail-on warn`时存在WARN |
| `2` | 参数错误或内部执行错误 |

## 安全性

AIStat只读运行，不会：

- 修改驱动、sysctl、Docker配置、用户组或工作负载位置；
- 启动容器、导入Python包或执行benchmark；
- 使用Shell执行外部命令；
- 上传报告或发送遥测。

外部命令使用固定白名单，并具有超时、输出限制和进程树清理。运行时发现只读取有界的软件包元数据，不扫描其他用户的HOME目录。详细说明见[安全设计](docs/security.md)。

## 当前边界

`0.1.0`检查单台Linux NVIDIA节点。目前不包含Kubernetes、Slurm、多节点诊断、长期监控、性能profiling、benchmark和自动调优。

没有活跃工作负载或时间序列证据时，Performance Readiness可以保持 `UNKNOWN`。AIStat会解释缺少什么证据，而不是猜测结果。

下一阶段的重点是对比优化前后的报告，验证建议是否真正有效。更多资料见[验证记录](docs/validation.md)、[架构](docs/architecture.md)、[采集器](docs/collectors.md)、[规则](docs/rules.md)和 [JSON Schema](docs/schema/report-v0.1.schema.json)。

## 构建

项目使用Go 1.26.5：

```bash
git clone https://github.com/arpingblue/AIStat.git
cd AIStat
go test ./...
CGO_ENABLED=0 go build -trimpath -o aistat ./cmd/aistat
```

欢迎参与开发。修改采集器、规则或公开报告模型前，请先阅读 [CONTRIBUTING.md](CONTRIBUTING.md)。

## 开源许可

[Apache License 2.0](LICENSE)
