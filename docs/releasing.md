# 打包与发布

## 当前状态

发布配置已完成，本地已成功生成六个平台的 snapshot 并核对 SHA-256。尚未发布 `v0.1.0`，真实 SSH、剪贴板、终端视觉效果和三个系统的运行验收仍待完成。构建成功不代表跨平台运行已验证。

## 本地打包

需要 Go 1.26、Git 和 GoReleaser **v2.12.0**（与工作流一致）。从官方 GoReleaser release 下载适合本机的工具并核对官方 checksums，再放入 PATH；不需要将工具提交到仓库。

```sh
go mod verify
go vet ./...
go test ./...
go build ./...
goreleaser check
goreleaser release --snapshot --clean
# 或 make snapshot
```

`--clean` 清理的目标是生成目录 `dist/`，不要将手工文件放进去。Snapshot 不发布 release、不上传文件、不创建 tag；没有 tag 时版本由 GoReleaser 的回退值计算（当前为 `0.0.1-snapshot`），并非正式 `v0.1.0`。

`dist/` 中包含：

- `addresspad_<version>_{darwin,linux}_{amd64,arm64}.tar.gz`（4 个包）
- `addresspad_<version>_windows_{amd64,arm64}.zip`（2 个包）
- `checksums.txt`（SHA-256）及构建元数据

每个包附带 `apad` / `apad.exe`、LICENSE、README 和 docs。二进制统一 `CGO_ENABLED=0`，版本、commit、时间由 GoReleaser 注入。项目发布目标为 64 位 amd64/arm64，不包含 386。

## GitHub 工作流

`.github/workflows/release.yml` 在 `v[0-9]*` tag 推送时运行，也可从 Actions 手工触发：

1. macOS、Linux、Windows 执行测试、vet 和构建；Linux 额外运行 `-race`。
2. 检查通过后构建六个平台包及校验清单。
3. tag 运行创建 **draft release**；从分支手工运行只构建 snapshot，不发布。
4. 打包文件和校验清单额外保存为 Actions artifact，保留 14 天。

完整发布需要仓库允许 Actions 使用 `GITHUB_TOKEN` 写入 contents；无需在仓库中保存个人 token。工作流尚未在远端运行验证。分叉仓库需修改 `.goreleaser.yaml` 的 `release.github.owner/name`。

## 正式发布前人工门禁

- [ ] 三个系统上解压、`--version`、`--help`、`list --json`、TUI 正常
- [ ] 在支持的平台执行 `go test -race ./...`，确认远端 CI 通过
- [ ] 真实主机连接；密码/密钥认证、host key 提示、退出后屏幕恢复
- [ ] Windows Terminal 及 PowerShell 剪贴板，macOS/Linux 剪贴板与无工具回退
- [ ] 配置损坏不写盘、无 ssh 提示、中文备注/输入法
- [ ] 新增、编辑、删除、取消确认、保存失败后重试
- [ ] 导入真实 SSH config 后核对并连接（特别是 Include、ProxyJump）
- [ ] 60×20 与 200×50 真实终端截图或 asciinema 录制；不得包含真实私钥、地址或账号
- [ ] 检查六个平台包、checksums、文档及已知限制

以上通过并获得发布授权后，由维护者提交发布版本、创建并推送 `v0.1.0` tag。tag 会触发草稿 release；下载草稿产物复核后，再在 GitHub 手工点击 Publish。此文档中的流程不会自动创建或推送 tag。

## 本次本地验证

GoReleaser 2.12.0 `check` 和 snapshot 构建成功，六个归档校验均通过；Windows amd64 产物 `--version` 正确显示 snapshot 版本、commit 和构建时间。尚未运行 macOS/Linux/Windows arm64 产物，尚未触发远端 Actions，也没有创建正式 tag/release。
