# AddressPad

用 Go 编写的终端 SSH 地址簿，命令名 `apad`。连接将交给系统 OpenSSH，不保存密码。

## 当前进度

已实现 M1 项目骨架、M2 配置数据层与 M3 TUI 列表及其单元测试。**目前尚未实现 SSH 连接、表单、导入与增删改 CLI**；完整设计见 [docs](docs/README.md)，后续计划见 [路线图](docs/roadmap.md)。

## 构建

需要 Go 1.26（支持 Go 自动下载工具链）。

```sh
go build -o bin/apad .
# Windows: go build -o bin/apad.exe .
go run . --version
go run . --help
go run . config path
go run . --config ./servers.yaml config path
go run .
```

`config path` 只打印路径，不创建文件。路径优先级为 `--config` → `ADDRESSPAD_CONFIG` → 系统用户配置目录下的 `addresspad/servers.yaml`。

## 当前 TUI 用法

直接运行 `apad` 或 `go run .` 打开交互列表，首次运行自动创建空配置。现阶段可按 [配置说明](docs/configuration.md) 手动填写服务器；配置损坏时只显示错误页，不修改文件。

- `↑/↓`、`j/k`、`ctrl+p/ctrl+n` 导航，`g/G` 跳到首尾。
- `/` 搜索：名称、用户、主机、分组、标签支持大小写不敏感的子序列匹配，空格分隔的关键词必须全部命中。
- 搜索时 `ctrl+u` 清空、`ctrl+w` 删除上一个词；`esc` 返回列表并保留过滤，无匹配时清空过滤。列表中 `esc` 清空过滤。
- `f` 切换收藏并保存，列表始终按收藏、最近使用、名称排序。
- `?` 打开帮助，`esc` / `?` / `q` 关闭；列表中 `q` 或任意模式 `ctrl+c` 退出。
- 支持窄终端、中文显示及 `NO_COLOR`。连接、复制、表单和导入快捷键目前仅提示后续阶段，尚不执行操作。

可通过 `-ldflags "-X main.version=v0.1.0 -X main.commit=COMMIT -X main.date=DATE"` 注入构建信息，或使用 `make build VERSION=v0.1.0`。Makefile 的 `DATE` 默认使用最新提交时间，可通过 `make build DATE=...` 覆盖。

## 检查

```sh
go test ./...
go vet ./...
go test -race ./...
```

`-race` 需要受支持的平台和 C 工具链，不支持 `windows/386`。CI 在三个系统上执行普通测试，在 Linux 上额外执行竞态检测。

数据层支持 YAML 校验、默认值、稳定 ID、原子写入、备份、增删改与收藏切换；保存失败不提交内存修改，损坏或版本过高的配置拒绝覆盖。非 Windows 下新目录权限为 `0700`、配置及备份为 `0600`；Windows 权限位仅尽力处理。

`make snapshot` 是后续发布流程的预留入口，需在 M7 补齐 GoReleaser 配置后使用。

## 许可

[MIT](LICENSE)
