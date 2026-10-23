# AddressPad

用 Go 编写的终端 SSH 地址簿，命令名 `apad`。连接将交给系统 OpenSSH，不保存密码。

## 当前进度

已实现 M1 项目骨架、M2 配置数据层及其单元测试。**目前尚未实现 TUI、SSH 连接、导入与增删改 CLI**；完整设计见 [docs](docs/README.md)，后续计划见 [路线图](docs/roadmap.md)。

## 构建

需要 Go 1.26（支持 Go 自动下载工具链）。

```sh
go build -o bin/apad .
# Windows: go build -o bin/apad.exe .
go run . --version
go run . --help
go run . config path
go run . --config ./servers.yaml config path
```

`config path` 只打印路径，不创建文件。路径优先级为 `--config` → `ADDRESSPAD_CONFIG` → 系统用户配置目录下的 `addresspad/servers.yaml`。

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
