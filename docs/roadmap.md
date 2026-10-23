# 路线图

当前阶段：M1–M5 代码已实现，下一阶段为 M6 导入与 CLI。目标首个版本 `v0.1.0`。真实主机与终端交互验收尚未完成。

验证记录：本机通过 `go test ./...`、`go vet ./...`、`go build`；测试覆盖率 main 91.7%、model 86%、store 89.1%、ui 87.8%。M3 已自动测试 60×20、200×50、24×8 等尺寸的行数/宽度、模式切换、搜索排序、收藏及错误页；1000 条记录过滤基准约 0.22 ms/op（windows/386），真实终端交互与截图仍待手工验收。本机为 windows/386，无法运行 `-race`，Unix 权限验证也需在 Linux/macOS 执行，已配置跨平台 CI，尚待 CI 验证。M7 发布配置尚未实现，`snapshot` 目前仅为预留命令。

## 里程碑

### M1 项目骨架

- [x] 初始化 go.mod（Go 1.26）
- [x] `main.go`：子命令分发、`--config`、`--version`
- [x] 版本号通过 `-ldflags -X` 注入（version / commit / date）
- [x] Makefile：`build`、`run`、`test`、`lint`、`snapshot`
- [x] `.gitignore`、LICENSE

**完成标准**：`go build` 产出可执行文件，`apad --version` 与 `apad --help` 正常，未知子命令返回退出码 2。

### M2 数据层

- [x] `model.Server` / `model.Config` 与校验、默认值、`~` 展开
- [x] `store.Load` / `Save`：路径解析、权限、原子写入、备份
- [x] 损坏文件识别（语法错误、版本过高）
- [x] CRUD：add / update / delete / toggle favorite
- [x] 表驱动单元测试

**完成标准**：`go test -race ./internal/...` 全绿；手工破坏 YAML 后程序只读报错且原文件不变；保存后权限为 0600。

### M3 TUI 列表

- [x] 根 Model 与模式枚举
- [x] 列表布局、主题、自适应宽度、空状态
- [x] 导航与滚动（`↑/↓`、`j/k`、`g/G`）
- [x] 搜索模式与子序列过滤、多关键词 AND
- [x] 排序：收藏 → 最近使用 → 名称
- [x] 帮助浮层、底部状态栏
- [x] 过滤与模式切换的单元测试

**完成标准**：在 60×20 与 200×50 终端下布局不破；1000 条数据下按键无可感延迟；搜索与排序结果符合文档。

### M4 一键连接

- [x] `sshcmd.Build`：端口、identity、ProxyJump、extra_args、`--`
- [x] `exec.LookPath("ssh")` 检查与错误提示
- [x] `tea.ExecProcess` 挂起 / 恢复、返回后清屏重绘
- [x] 更新 `last_used` 并重新排序（SSH 退出码 0 时）
- [x] 复制 ssh 命令（macOS / Linux / Windows 剪贴板回退）
- [x] argv 表驱动测试

M4 验证：普通测试、静态检查、构建通过；sshcmd 覆盖率 79.4%，ui 覆盖率 91.3%。使用测试子进程验证 `tea.ExecProcess` 的输出移交、正常退出与退出码 23，不访问网络、不操作真实剪贴板。真实 SSH、密码/密钥认证、host key 交互、剪贴板及终端残影验证待手工完成。Windows 剪贴板使用 UTF-8 PowerShell 替代 clip.exe，复制命令按 PowerShell 语法转义。

**完成标准**：本机连接真实主机成功；密码提示与 host key 提示可正常交互；退出后 TUI 无残影；`c` 复制内容可直接粘贴执行。

### M5 增删改查表单

- [x] 表单渲染与字段切换
- [x] 内联校验、首个错误自动聚焦
- [x] 新增 / 编辑保存
- [x] 删除确认
- [x] 脏数据取消确认
- [x] 保存失败时保留输入

M5 验证：`go test ./...`、`go vet ./...`、构建通过，UI 覆盖率 94.4%。覆盖新增/编辑持久化、稳定 ID 与 last_used 保留、标签/参数往返、非法端口/重名/主机/引号错误、首个错误聚焦、取消保护、保存失败保留输入及重试、删除取消与失败重试、60×20/200×50/24×8 等表单布局。真实终端键盘、输入法、多行备注与视觉效果待手工验收。

**完成标准**：全流程不碰配置文件也能完成增删改；非法端口、重复名称被拦截；`esc` 不会静默丢改动。

### M6 导入与 CLI

- [ ] `sshconfig.Parse`：Host 多别名、HostName、User、Port、IdentityFile、ProxyJump
- [ ] Include：glob、相对路径、深度上限、循环检测
- [ ] 跳过通配符 Host 与 Match 块
- [ ] `apad import` / `--dry-run` / `--overwrite`
- [ ] `apad add` / `apad list [--json]` / `apad config path`
- [ ] 直连：`apad <name>`、`apad connect <name>`、模糊匹配与候选提示

**完成标准**：用 fixture 覆盖多别名、Include、通配符、Match；导入后条目可直接连接；`apad list --json` 可被 `jq` 解析。

### M7 发布

- [ ] `.goreleaser.yaml`：darwin / linux / windows × amd64 / arm64，`CGO_ENABLED=0`
- [ ] GitHub Actions：tag 触发 release、checksums、产物归档
- [ ] README：截图 / asciinema、安装、常用操作
- [ ] `docs/` 与实现保持一致
- [ ] 打 tag `v0.1.0`

**完成标准**：`goreleaser release --snapshot --clean` 产出 6 个平台包；三个系统上解压即用（Windows 需自带 OpenSSH）。

## 总验收清单

- [ ] `go vet ./...` 无告警
- [ ] `go test -race ./...` 全绿
- [ ] `go build ./...` 成功
- [ ] `goreleaser release --snapshot --clean` 成功
- [ ] 本机连接 OrbStack / 真实服务器各一次
- [ ] 密码认证与密钥认证各验证一次
- [ ] 配置损坏、无 ssh、UTF-8 中文备注三种异常路径验证
- [ ] 60 列窄终端与 200 列宽终端各截图

## 风险与缓解

| 风险 | 影响 | 缓解 |
|---|---|---|
| Windows 上 `ExecProcess` + OpenSSH 的 alt-screen 切换异常 | 连接后界面错乱 | M4 优先在 Windows Terminal 验证；必要时降级为“退出 TUI 后 exec，结束不回列表” |
| ssh 输出后 TUI 残留 | 显示错乱 | 返回时主动 `tea.ClearScreen` 并全量重绘；关闭鼠标模式 |
| Linux 无剪贴板工具 | 复制失败 | 依次尝试 `wl-copy` / `xclip` / `xsel`，全部失败时在状态栏显示命令 |
| Include 循环 | 导入卡死 | 记录已访问真实路径，深度上限 8，超限报错 |
| 配置被外部同时修改 | 丢更新 | 文档明示后写入者胜出；v1 不做锁 |
| 中文 / emoji 宽度 | 列错位 | 用 `lipgloss.Width` 计算显示宽度，测试覆盖 CJK 与 emoji |
| ssh 参数注入 | 命令执行异常 | 拒绝 `-` 开头的 host，命令行使用 `--` 分隔 |

## 后续版本候选

### v0.2

- 密码存入 macOS Keychain / Windows Credential Manager / libsecret，通过 `SSH_ASKPASS` 免交互连接
- 分组侧边栏、标签筛选、最近连接视图
- 连通性检测（并发 TCP 探测，显示在线状态）
- 配置热加载

### v0.3

- 与 `~/.ssh/config` 双向同步（生成带 `# addresspad` 标记的片段）
- 主题切换、自定义键位
- 会话内端口转发快捷方式
- 多选批量操作

### 暂不做

- SFTP 文件浏览
- 多标签 / 分屏会话
- 团队共享配置与云端同步
- 自己实现 SSH 协议

## 决策记录

| 日期 | 决策 | 理由 |
|---|---|---|
| 2026-10-23 | Go + Bubble Tea | 单二进制、启动快、TUI 生态成熟 |
| 2026-10-23 | 连接交给系统 `ssh` | 原生兼容 agent、ProxyJump、known_hosts，维护成本最低 |
| 2026-10-23 | 自有 YAML + 从 ssh config 导入 | 可编辑、可版本化，同时不破坏现有 config |
| 2026-10-23 | v1 不存密码 | 降低安全风险，ssh 自身提示足够可用 |
| 2026-10-23 | 跨平台发布（darwin / linux / windows） | 单二进制无运行时依赖，goreleaser 成本低 |
