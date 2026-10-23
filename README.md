# AddressPad

用 Go 编写的终端 SSH 地址簿，命令名 `apad`。连接将交给系统 OpenSSH，不保存密码。

## 当前进度

已实现 M1–M6 功能及 M7 打包/发布配置，本地已生成六个平台 snapshot。**尚未正式发布 v0.1.0**，真实 SSH/终端交互、跨平台运行和远端工作流仍待验收。完整设计见 [docs](docs/README.md)，进度见 [路线图](docs/roadmap.md)。

## 安装

正式发布后，可在 [GitHub Releases](https://github.com/Chen-Christins/AddressPad/releases) 下载对应系统和 CPU 的包；若尚无公开版本，请使用下方源码构建方式。支持 macOS/Linux/Windows × amd64/arm64，单二进制无需 Go 运行时；连接需另有系统 OpenSSH。

1. 下载归档和同版本 `checksums.txt`，核对 SHA-256（Linux：`sha256sum -c checksums.txt --ignore-missing`；macOS：`shasum -a 256 文件名`；PowerShell：`Get-FileHash 文件名 -Algorithm SHA256`，后两者与清单手工比较）。
2. macOS/Linux 解压 `tar -xzf 文件名.tar.gz`，将 `apad` 放入 PATH 下可写目录（如 `~/.local/bin`，并将其加入 PATH）。Windows 解压 zip，将包含 `apad.exe` 的目录加入用户 PATH。
3. 运行 `apad --version`、`ssh -V`，再运行 `apad`。Windows 推荐 Windows Terminal + PowerShell，并安装 OpenSSH Client。

发布包中的版本不带 `v` 前缀，例如 tag `v0.1.0` 对应 `addresspad_0.1.0_windows_amd64.zip`。本地 snapshot 位于 `dist/`，不属于正式版本。

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

直接运行 `apad` 或 `go run .` 打开交互列表，首次运行自动创建空配置。按 `a` 添加第一条服务器，无需手动编辑配置；也可按 [配置说明](docs/configuration.md) 手动填写。配置损坏时只显示错误页，不修改文件。

- `↑/↓`、`j/k`、`ctrl+p/ctrl+n` 导航，`g/G` 跳到首尾。
- `/` 搜索：名称、用户、主机、分组、标签支持大小写不敏感的子序列匹配，空格分隔的关键词必须全部命中。
- 搜索时 `ctrl+u` 清空、`ctrl+w` 删除上一个词；`esc` 返回列表并保留过滤，无匹配时清空过滤。列表中 `esc` 清空过滤。
- `f` 切换收藏并保存，列表始终按收藏、最近使用、名称排序。
- `enter` 连接选中服务器，终端交给系统 `ssh`；退出 SSH 后返回列表并清屏重绘。SSH 正常退出（退出码 0）时保存 `last_used` 并重新排序；失败不更新。搜索模式下也支持连接并保留搜索条件。
- `c` 复制等效 SSH 命令。Windows 使用 PowerShell 命令语法和 UTF-8 剪贴板，macOS/Linux 使用 POSIX shell 语法；不支持粘贴到 Windows cmd.exe。
- `?` 打开帮助，`esc` / `?` / `q` 关闭；列表中 `q` 或 `ctrl+c` 退出。SSH 运行期间按键直接交给 SSH，包括 `ctrl+c`。
- `a` 新增、`e` 编辑、`d` 删除；删除需 `y` / `enter` 确认，`n` / `esc` 取消。
- 表单用 `tab` / `shift+tab` / `↑` / `↓` 切换字段，`ctrl+s` 保存；`enter` 跳到下一字段，在最后字段保存。备注用 `alt+enter` 换行，收藏字段用空格切换。
- 表单 `esc` 取消，有修改时需确认放弃；`n` 继续编辑。`ctrl+c` 直接取消并退出，不保存。非法字段内联提示并聚焦首个错误，保存失败保留输入以便重试。
- 标签用逗号分隔；额外参数支持单/双引号和反斜线转义，不做变量或命令展开。Windows 路径建议用单引号，如 `-i 'C:\Users\name\my key'`。编辑会保留 ID、最近使用时间和原始参数边界。
- 保存后选中对应条目；若新名称或字段不再匹配原过滤条件，会清空过滤以显示该条目。
- `i` 从 `~/.ssh/config` 导入，已存在的名称跳过，不覆盖。需要预览或覆盖时使用 CLI。
- 支持窄终端、中文显示及 `NO_COLOR`。

连接需安装 OpenSSH 并将 `ssh` 加入 PATH。剪贴板依赖：macOS `pbcopy`，Windows `powershell.exe` / `Set-Clipboard`，Linux 依次尝试 `wl-copy`、`xclip`、`xsel`；失败时打开命令展示页供手动复制。`extra_args` 会传给 OpenSSH（可包含 `ProxyCommand` 等会执行外部程序的选项），请仅使用可信配置。

M4 已通过模拟子进程的终端移交和退出码集成测试；真实主机、密码/密钥认证、host key 提示及 Windows Terminal 恢复效果尚待手工验收。

M5 已自动测试增删改、取消确认、错误聚焦、失败重试及多尺寸表单布局；实际终端键盘/输入法与多行备注显示仍待手工验收。

## CLI

```sh
apad add --name prod-web --host 10.0.0.1 --user deploy --tag web
apad list
apad list --json
apad import --dry-run
apad import --ssh-config ./ssh_config --overwrite
apad prod-web
apad connect prod-web
apad help import
```

全局 `--config PATH` 放在子命令之前；`--tag` 和 `--extra-arg` 可重复。列表与导入预览不创建配置文件，非法新增、导入解析失败也不写盘。导入在内存中合并后一次保存，`--overwrite` 只更新连接字段，保留 ID、收藏、备注、分组、标签、额外参数与最近使用时间。

直连先大小写不敏感精确匹配，再做唯一子串匹配；多条命中列出候选，无命中提示最接近的 3 个名称。SSH 退出码透传，正常退出后保存 `last_used`；保存失败只警告，不改变 SSH 退出码。与子命令重名的服务器使用 `apad connect NAME`。

导入支持 Host 多别名、HostName、User、Port、IdentityFile、ProxyJump，以及 Include glob/递归（上限 8、检测当前递归链的真实路径循环）。相对 Include 基于用户 `~/.ssh`，未匹配 glob 忽略。通配符 Host、Match 块和未支持指令不做求值；这不是完整的 OpenSSH 配置解析器，全局/通配符默认值不会合并，重复别名保留第一条。复杂配置导入后请先核对列表。

可通过 `-ldflags "-X main.version=v0.1.0 -X main.commit=COMMIT -X main.date=DATE"` 注入构建信息，或使用 `make build VERSION=v0.1.0`。Makefile 的 `DATE` 默认使用最新提交时间，可通过 `make build DATE=...` 覆盖。

## 检查

```sh
go test ./...
go vet ./...
go test -race ./...
```

`-race` 需要受支持的平台和 C 工具链，不支持 `windows/386`。CI 在三个系统上执行普通测试，在 Linux 上额外执行竞态检测。

数据层支持 YAML 校验、默认值、稳定 ID、原子写入、备份、增删改与收藏切换；保存失败不提交内存修改，损坏或版本过高的配置拒绝覆盖。非 Windows 下新目录权限为 `0700`、配置及备份为 `0600`；Windows 权限位仅尽力处理。

## 打包与发布

使用 GoReleaser v2.12.0：

```sh
goreleaser check
goreleaser release --snapshot --clean
# 或 make snapshot
```

生成六个平台包和 SHA-256 清单到 `dist/`，snapshot 不上传。GitHub tag 工作流通过测试后创建草稿 release，手工从分支运行只归档 snapshot。详见 [发布说明与人工验收清单](docs/releasing.md)。本轮未创建或推送 tag。

界面布局示意（非真实终端截图，截图/录屏仍待人工验收）：

```text
AddressPad  2/2 servers
Filter:
────────────────────────────────────────────────────────────
> ★ prod-web      deploy@10.0.0.1:22     [prod] #web
    staging       ubuntu@example.com:2222

────────────────────────────────────────────────────────────
LIST 1/2 | enter connect · a/e/d edit · / search · ? help
```

## 许可

[MIT](LICENSE)
