# CLI 参考

`apad` 既可以打开 TUI，也可以作为脚本里的快捷 ssh 入口。

## 用法

```
apad [全局参数] [子命令] [参数...]
```

不写子命令时打开 TUI：

```bash
apad
```

## 全局参数

| 参数 | 说明 |
|---|---|
| `--config <path>` | 指定配置文件，覆盖环境变量与默认路径；放在子命令之前 |
| `--version` | 打印版本、commit、构建时间 |
| `-h`, `--help` | 帮助 |

## 子命令

### `apad connect <name>` / `apad <name>`

跳过 TUI 直接连接。两种写法等价，区别只在名称与子命令冲突时必须用前者：

```bash
apad prod-web-1
apad connect list      # 服务器名恰好叫 list 时
```

查找规则：

1. 大小写不敏感精确匹配 `name`
2. 没有精确匹配时做子串匹配；恰好命中一条则连接
3. 命中多条则列出候选并退出（退出码 1）
4. 无命中则提示，并给出最接近的 3 个名称

连接行为：

- 拼装 `ssh [-p port] [-i identity] [-J proxy_jump] [extra_args...] -- [user@]host`
- 使用 `tea.ExecProcess` 等价机制直接 exec，标准输入输出完全交给 `ssh`
- 进程退出码原样透传（包括 ssh 自己的 1、2、255）
- 连接成功后写回 `last_used`

### `apad list`

以纯文本列出所有服务器，按“收藏 → 最近使用 → 名称”排序。

```bash
apad list
apad list --json
```

文本格式（列宽自适应）：

```
★ prod-web-1   deploy@10.0.1.5:22    [prod]   #web #k8s   生产 Web 入口
  staging      ubuntu@1.2.3.4:22     [stage]
```

`--json` 输出适合脚本消费的结构：

```json
{
  "version": 1,
  "servers": [
    {
      "id": "01J8XQ7K3M9P2R5T7V0W1Y4B6C",
      "name": "prod-web-1",
      "host": "10.0.1.5",
      "user": "deploy",
      "port": 22,
      "identity_file": "~/.ssh/id_ed25519",
      "proxy_jump": "bastion",
      "group": "prod",
      "tags": ["web", "k8s"],
      "favorite": true,
      "note": "生产 Web 入口",
      "extra_args": [],
      "last_used": "2026-10-23T09:12:00+08:00"
    }
  ]
}
```

- 输出到 stdout；错误与提示到 stderr
- `tags`、`extra_args` 永远输出数组，不输出 `null`
- 未设置的可选字符串字段输出空字符串

### `apad add`

非交互新增一条服务器。

```bash
apad add \
  --name prod-web-1 \
  --host 10.0.1.5 \
  --user deploy \
  --port 22 \
  --identity ~/.ssh/id_ed25519 \
  --group prod \
  --tag web --tag k8s \
  --note "生产 Web 入口"
```

| 参数 | 必填 | 说明 |
|---|---|---|
| `--name` | 是 | 唯一名称 |
| `--host` | 是 | IP 或域名 |
| `--user` | 否 | 登录用户 |
| `--port` | 否 | 默认 22 |
| `--identity` | 否 | 私钥路径 |
| `--proxy-jump` | 否 | 传给 `-J` |
| `--group` | 否 | 单分组 |
| `--tag` | 否 | 可重复，一个参数一个标签 |
| `--note` | 否 | 备注 |
| `--favorite` | 否 | 设为收藏 |
| `--extra-arg` | 否 | 可重复，透传给 ssh |

成功时 stdout 输出 `added <name>`，退出码 0；重名或校验失败退出码 2，不会修改文件。

### `apad import`

从 OpenSSH 配置导入 Host 条目。

```bash
apad import
apad import --ssh-config ./testdata/ssh_config
apad import --dry-run
apad import --overwrite
```

| 参数 | 默认 | 说明 |
|---|---|---|
| `--ssh-config` | `~/.ssh/config` | 待解析文件 |
| `--dry-run` | false | 只显示将导入 / 跳过的条目，不写盘 |
| `--overwrite` | false | 用 ssh config 更新已存在的同名条目 |

完整映射与去重规则见 [configuration.md](./configuration.md#从-sshconfig-导入)。

输出示例：

```
parsed  /Users/me/.ssh/config
include ~/.orbstack/ssh/config (depth 1)

would import 4 servers:
  prod-web-1   10.0.1.5
  prod-db      10.0.1.9
  staging      1.2.3.4
  orb          127.0.0.1

skipped:
  *            wildcard host
  github.com   already exists (use --overwrite)
```

解析失败（文件不存在、Include 循环、语法错误）退出码 1，不写入任何内容。

### `apad config path`

打印当前生效的配置文件路径，便于脚本使用：

```bash
vim "$(apad config path)"
```

文件不存在时仍打印路径（不会创建）。

### `apad help`

打印帮助；`apad help <command>` 打印子命令帮助。

## 退出码

| 码 | 含义 |
|---|---|
| 0 | 成功 |
| 1 | 运行时错误：配置读写失败、找不到名称、ssh 不存在、导入失败 |
| 2 | 用法错误：未知子命令、缺少必填参数、校验失败 |
| * | `connect` / `apad <name>` 透传 ssh 的退出码（常见 1、2、255） |

TUI 中按 `q` 或 `ctrl+c` 正常退出返回 0；被系统信号终止时为 128+信号值。

## 环境变量

| 变量 | 说明 |
|---|---|
| `ADDRESSPAD_CONFIG` | 配置文件路径，优先级低于 `--config` |
| `NO_COLOR` | 设置后禁用所有颜色与样式，输出纯文本 |

## 脚本示例

```bash
# 快速连接
apad prod-db

# 收藏列表
apad list --json | jq -r '.servers[] | select(.favorite) | .name'

# 从 kubectl 之类生成的名单批量添加
while read -r name host; do
  apad add --name "$name" --host "$host" --group imported
done < servers.txt
```
