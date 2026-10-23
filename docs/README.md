# AddressPad 文档

用 Go + Bubble Tea 编写的 TUI SSH 管理器：在终端里搜索、维护服务器列表，选中后一键连接。

> 状态：M1–M6 已实现，下一阶段为 M7 发布。命令名 `apad`。支持交互列表、搜索、收藏、SSH 连接、命令复制、增删改表单、SSH 配置导入以及新增/列表/直连 CLI，详见根目录 README。真实 SSH/终端交互与跨平台验证仍待验收。

## 文档索引

| 文档 | 内容 |
|---|---|
| [architecture.md](./architecture.md) | 设计原则、技术栈、目录结构、连接时序、TUI 状态机、错误处理、测试策略 |
| [configuration.md](./configuration.md) | 配置文件位置、字段说明、校验规则、保存机制、ssh config 导入映射 |
| [cli.md](./cli.md) | 子命令、参数、JSON 输出、退出码、环境变量、脚本示例 |
| [keybindings.md](./keybindings.md) | 导航 / 搜索 / 表单 / 确认四种模式的完整键位 |
| [roadmap.md](./roadmap.md) | 里程碑、验收清单、风险与缓解、后续版本 |

## 三句话概括

1. `apad` 打开 TUI，输入即过滤，`Enter` 连接；`a/e/d` 增删改。
2. 连接时把终端交给系统 `ssh`，所以 agent、ProxyJump、host key 提示全部原生可用。
3. 数据存在 `servers.yaml`，可用 `apad import` 从 `~/.ssh/config` 导入，也可用 `apad <name>` 直接连接。
