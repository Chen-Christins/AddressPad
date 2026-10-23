# 配置说明

AddressPad 的全部数据保存在一个 YAML 文件里，不依赖数据库或后台进程。

## 文件位置

按以下顺序解析，先命中者生效：

1. 命令行参数 `--config <path>`
2. 环境变量 `ADDRESSPAD_CONFIG`
3. `os.UserConfigDir()/addresspad/servers.yaml`

`os.UserConfigDir()` 的取值：

| 系统 | 实际路径 |
|---|---|
| macOS | `~/Library/Application Support/addresspad/servers.yaml` |
| Linux | `$XDG_CONFIG_HOME/addresspad/servers.yaml`（缺省 `~/.config/...`） |
| Windows | `%AppData%\addresspad\servers.yaml` |

`apad config path` 可随时查看当前生效的路径。

首次启动时自动创建目录与空配置：

- 目录权限 `0700`
- 文件权限 `0600`
- Windows 上权限位不生效，属于尽力而为

## 完整示例

```yaml
version: 1
servers:
  - id: 01J8XQ7K3M9P2R5T7V0W1Y4B6C
    name: prod-web-1
    host: 10.0.1.5
    user: deploy
    port: 22
    identity_file: ~/.ssh/id_ed25519
    proxy_jump: bastion
    group: prod
    tags: [web, k8s]
    favorite: true
    note: 生产 Web 入口
    extra_args: []
    last_used: 2026-10-23T09:12:00+08:00

  - id: 01J8XQ8A1B2C3D4E5F6G7H8J9K
    name: bastion
    host: 5.6.7.8
    user: ops
    port: 22
    group: infra
    tags: [jump]
    favorite: false
    note: 跳板机
```

## 字段说明

| 字段 | 类型 | 必填 | 默认 | 说明 |
|---|---|---|---|---|
| `version` | int | 否 | `1` | 配置格式版本，用于未来迁移 |
| `id` | string | 否 | 自动生成 | 稳定标识；手写时可省略，加载时补齐 |
| `name` | string | 是 | — | 唯一名称，用于列表显示和 `apad <name>` 查找 |
| `host` | string | 是 | — | IP 或域名，传给 ssh 的目标主机 |
| `user` | string | 否 | 系统默认 | 登录用户；为空时用 ssh 的默认值 |
| `port` | int | 否 | `22` | 范围 1–65535；为 22 时不传 `-p` |
| `identity_file` | string | 否 | — | 私钥路径，支持 `~`；不校验文件是否存在 |
| `proxy_jump` | string | 否 | — | 传给 `-J`，格式如 `user@bastion:22` |
| `group` | string | 否 | — | 单分组名，用于显示与后续筛选 |
| `tags` | []string | 否 | `[]` | 多标签，保存时去空白、去重 |
| `favorite` | bool | 否 | `false` | 收藏置顶 |
| `note` | string | 否 | — | 自由文本备注 |
| `extra_args` | []string | 否 | `[]` | 追加到 ssh 命令行（`-J` 之后、目标之前） |
| `last_used` | string (RFC3339) | 否 | — | 程序在连接成功后写入 |

未知字段会被忽略，并在下一次保存时丢失；如需保存额外信息请使用 `note` 或 `extra_args`。

## 校验规则

保存前在内存中校验，非法时在表单内联提示，不写盘。

| 规则 | 说明 |
|---|---|
| `name` 非空 | 去除首尾空白后不能为空 |
| `name` 唯一 | 大小写不敏感比较 |
| `host` 非空 | 不能包含空白字符，不能以 `-` 开头 |
| `port` | 缺省 22；必须是 1–65535 的整数 |
| `tags` | 去空白、丢弃空项、去重（保持原顺序） |
| `identity_file` | 保存原值（含 `~`），构造命令时才展开 |
| `extra_args` | 原样保存，允许空数组 |

`host` 以 `-` 开头会被拒绝，同时构造命令行时始终使用 `--` 分隔符，双重防护参数注入。

## 保存机制

每次增删改都执行完整写入：

1. 序列化为 YAML
2. 写入同目录临时文件 `servers.yaml.tmp.<rand>`
3. `fsync` 后 `close`
4. 若原文件存在，先复制为 `servers.yaml.bak`
5. `rename` 原子覆盖目标文件
6. 设置权限 `0600`（非 Windows）

任何一步失败都会保留原文件不变，并在 UI 中提示错误。写入采用“后写入者胜出”，不提供文件锁；请避免同时开多个 `apad` 实例编辑。

## 损坏文件处理

以下情况进入只读错误页，不写盘、不自动修复：

- YAML 语法错误（显示行号与原因）
- `version` 大于当前程序支持的版本
- 结构类型不匹配（例如 `servers` 不是数组）

错误页显示：

```
配置文件解析失败
路径: /Users/me/.config/addresspad/servers.yaml
行 12: 无法解析 port，期望整数
按 q 退出；修复文件后重新启动
```

## 手改配置

- 可以手动编辑，退出编辑器后重启 `apad` 生效（v1 不做热加载）
- `id` 可省略，加载时自动生成并在下次保存时写回
- 建议保留 `servers.yaml.bak`，误删时可直接恢复
- 需要注释时请写在 YAML 允许的位置；保存后注释不会保留

## 从 `~/.ssh/config` 导入

`apad import` 把现有 Host 条目转换为 AddressPad 条目，映射如下：

| ssh config | AddressPad |
|---|---|
| `Host foo bar` | 每个别名生成一条记录，`name` 为别名 |
| `HostName` | `host`；缺省时用别名本身 |
| `User` | `user` |
| `Port` | `port`；缺省 22 |
| `IdentityFile` | `identity_file`（保留 `~`） |
| `ProxyJump` | `proxy_jump` |
| `Include` | 递归解析，glob 展开，相对路径基于 `~/.ssh`，深度上限 8 |
| `Host *` / 通配符 | 跳过 |
| `Match` 块 | 跳过，直到下一个 `Host` |
| 其他指令 | 忽略（`ForwardAgent`、`LocalForward` 等需手动写入 `extra_args`） |

这是一个有限子集导入器，不执行 `Match exec`，不合并全局或通配符默认值，不完整模拟 OpenSSH 的配置求值。Include 像文本插入一样保留当前 Host 上下文；Match 内 Include 跳过；未匹配的 Include glob 忽略。重复别名保留第一条记录。复杂配置请在导入后核对连接字段。

去重规则：按 `name` 大小写不敏感匹配。已存在时默认跳过；`--overwrite` 更新 `host`、`user`、`port`、`identity_file`、`proxy_jump`，保留 `id`、`favorite`、`note`、`group`、`tags` 和 `last_used`。

## 版本迁移

`version: 1` 为首个格式版本。未来若变更结构，加载时按版本做内存迁移，并在保存前备份原文件。低于当前版本的配置自动升级；高于当前版本则拒绝加载，避免新版字段被旧版程序丢弃。
