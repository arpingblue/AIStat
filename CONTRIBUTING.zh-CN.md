# 参与AIStat开发

[English](CONTRIBUTING.md) | **简体中文**

欢迎提交Issue和Pull Request。来自真实Linux NVIDIA服务器的问题反馈尤其有价值，但上传日志或fixture前必须完成脱敏。

## 提交Issue

下面这些情况都可以提交Issue：

- 报告程序错误或错误诊断；
- 提出新功能；
- 讨论一条诊断规则；
- 提供AIStat尚未正确处理的服务器配置。

一份有效的Bug报告最好包含：

- `aistat version`输出的AIStat版本；
- Linux发行版、内核、CPU架构和GPU型号；
- 实际执行的完整命令；
- 预期行为和实际行为；
- 最小化且已经脱敏的输出或JSON片段；
- Docker、内核日志或进程检查是否受到权限限制。

上传内容前，请删除主机名、用户名、IP和MAC地址、容器ID、模型路径、Prompt、Token以及客户信息。不要公开凭据，也不要直接上传未经检查的完整支持包。

提出功能建议时，请先描述运维问题：工程师需要做出什么判断、能够取得哪些证据、最后如何验证结果。只提出一个命令名称通常不足以说明需求。

安全漏洞不要提交公开Issue，请按照 [SECURITY.md](SECURITY.md)报告。

## 提交Pull Request

范围小且目标明确的PR更容易审核。涉及大型架构调整、新增主机命令、修改JSON契约或增加诊断规则时，请先创建Issue讨论。

提交前至少运行：

```bash
gofmt -w ./cmd ./internal
go test ./...
go vet ./...
```

如果开发环境支持，还应运行：

```bash
go test -race ./...
staticcheck ./...
```

一份合格的PR应该：

- 说明用户遇到的问题以及最终行为；
- 保持AIStat只读且所有采集都有边界；
- 为新增行为和失败状态补充测试；
- 必要时同步更新中英文用户文档；
- 修改报告模型时同步更新JSON Schema和golden测试；
- 不提交构建产物、原始采集、主机身份信息或密钥。

## 修改采集器

Collector只负责采集事实和诊断信息，不能生成修改建议，也不能改变服务器状态。

解析逻辑应与文件遍历或命令执行分离。新增外部命令必须具备：

- 固定的可执行文件白名单；
- 不经过Shell插值的固定参数；
- 超时和输出大小限制；
- 命令不存在、权限不足、超时、输出损坏和输出过大的测试；
- 对进程参数或环境变量使用范围很窄且有文档说明的采集方式。

## 修改规则

Rule只能使用标准化Snapshot、Topology Graph、Profile和Clock，不能自行读取文件或执行命令。

每次规则修改应根据实际情况覆盖：

- 触发和通过；
- 未知或证据不足；
- 不适用或SKIP；
- 阈值边界；
- 误报回归。

Finding需要包含可操作的证据、影响、建议、验证方式、置信度和权威参考。缺少证据永远不能被判断为PASS。

详细规则见 [docs/contributing-rules.md](docs/contributing-rules.md)和 [docs/rules.md](docs/rules.md)。

## JSON与兼容性

公开报告是带版本的契约。修改枚举、必填字段或字段语义时必须决定是否升级Schema版本。新增可选字段也需要同步更新Schema、校验代码和测试。

JSON枚举保持小写，并使用 [report-v0.1.schema.json](docs/schema/report-v0.1.schema.json)验证变更。

## 审核原则

维护者可能要求缩小修改范围、补充证据、提供脱敏fixture或采用更保守的诊断状态。这些要求用于保证AIStat在陌生生产服务器上安全运行。

提交贡献即表示你同意按照仓库的 [Apache License 2.0](LICENSE)许可该贡献。
