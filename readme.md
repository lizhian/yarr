# yarr

**yarr**（yet another RSS reader）是一款使用 Go 编写的 Web RSS/Feed 阅读器，既可以作为带系统托盘图标的桌面应用使用，也可以部署为个人自托管服务。

yarr 将 Web 界面和静态资源嵌入单个可执行文件，并使用 SQLite 保存数据。运行时不需要额外部署数据库、Node.js 或前端服务，适合个人电脑、家庭服务器和轻量 VPS。

![yarr 界面截图](etc/promo.png)

## 主要功能

- 支持 RSS、Atom、RDF 和 JSON Feed。
- 三栏阅读界面：订阅源、文章列表和文章正文，可在窄屏设备上逐层浏览。
- 按全部、未读和收藏筛选文章，支持搜索、正序/倒序、未读优先和批量标记已读。
- 支持文件夹管理、订阅源重命名、移动、批量删除以及单独刷新。
- 支持列表/卡片两种文章列表布局，并可按订阅源保存布局偏好。
- 支持浅色、护眼和夜间主题，可切换字体、字号以及图标/文字工具栏。
- 支持滚动自动标记已读、触屏下拉刷新和键盘快捷键。
- 提供普通、正文和嵌入三种阅读方式；正文模式可抓取网页主要内容，并支持为订阅源配置 CSS 正文选择器。
- 自动发现订阅地址和站点图标，也可以手动指定订阅链接和图标链接。
- 支持 OPML 导入与导出，方便从其他阅读器迁移订阅。
- 集成 RSSHub，可配置多个实例，并可快速添加 Bilibili、Telegram 等订阅。
- 提供榜单模式，可将周期内的条目合并为带图或无图的榜单文章。
- 支持 Web 访问认证、内置 HTTPS、子路径部署和 Unix Socket。
- 支持手动备份和每日自动备份，备份同时包含 SQLite 数据库和 OPML 订阅清单。
- 提供 Fever API 和 FreshRSS 兼容的 Google Reader API，可连接第三方 RSS 客户端。

> yarr 定位为日常 Feed 阅读器，而不是永久归档工具。系统会定期清理每个订阅源超过 500 篇的已读且未收藏文章；未读文章和收藏文章不会被自动清理。

## 快速开始

### 使用预编译版本

从 [Releases](https://github.com/lizhian/yarr/releases/latest) 下载适合系统和 CPU 架构的压缩包。文件名通常为 `yarr_{OS}_{ARCH}[_gui].zip`：

- `OS`：操作系统，例如 `linux`、`darwin` 或 `windows`。
- `ARCH`：CPU 架构，常见值为 `amd64`（x86-64）和 `arm64`（AArch64）。
- `_gui`：带托盘图标的桌面版本；不带该后缀的是命令行版本。

命令行版本可直接启动：

```bash
./yarr -open
```

默认监听 `127.0.0.1:7070`，浏览器访问 <http://127.0.0.1:7070>。默认数据库是可执行文件同目录下的 `storage.db`。

### 桌面应用

#### macOS

1. 将 `yarr.app` 移动到 `/Applications`。
2. 首次运行时，按照 [Apple 的说明](https://support.apple.com/zh-cn/guide/mac-help/mh40616/mac) 允许打开来自未认证开发者的应用。
3. 点击菜单栏中的锚形图标，选择 **Open** 打开阅读器。

#### Windows

1. 解压 GUI 版本并运行 `yarr.exe`。
2. 点击系统托盘中的锚形图标，选择 **Open**。
3. Windows Defender 防火墙询问时，仅按实际需要允许网络访问。

#### Linux

将命令行程序放入用户可执行目录，然后安装桌面入口：

```bash
install -Dm755 yarr "$HOME/.local/bin/yarr"
bash etc/install-linux.sh
```

之后可以从应用菜单启动 yarr，也可以运行 `yarr -open`。

## 使用教程

### 1. 添加订阅

1. 点击左上角的设置按钮。
2. 选择 **新订阅源**。
3. 输入 Feed 地址或网站地址。若网页中声明了可用 Feed，yarr 会尝试自动发现。
4. 按需选择文件夹、内容方式和榜单模式，然后确认添加。

添加后，订阅源右侧的菜单可以修改标题、订阅链接、图标链接和正文选择器，也可以查看最近一次刷新的状态、获取数量和错误信息。

### 2. 导入已有订阅

在 **设置 > 订阅 > 导入** 中选择其他阅读器导出的 `.opml` 或 `.xml` 文件。yarr 会导入订阅源及其文件夹结构。

需要迁移到其他阅读器时，选择 **设置 > 订阅 > 导出** 下载 OPML 文件。

### 3. 阅读与整理

- 左栏用于选择全部订阅、未读、收藏、文件夹或单个订阅源。
- 中栏用于选择文章，可切换新旧排序、滚动自动已读以及列表/卡片布局。
- 右栏用于阅读正文、收藏、切换已读状态、打开原文，以及在普通/正文/嵌入模式间切换。
- 搜索按钮可在当前数据中查找文章。
- 文件夹菜单支持重命名和删除；订阅源菜单支持移动到已有或新建文件夹。

### 4. 配置正文抓取

如果 Feed 只提供摘要，可在添加或编辑订阅源时将 **内容方式** 设为 **正文**。yarr 会抓取原网页并提取主要内容。

自动提取不准确时，可以填写 CSS 选择器，例如 `.article-content` 或 `main article`，让 yarr 只读取匹配区域。该选择器应以目标网页实际 DOM 结构为准。

### 5. 配置 RSSHub

1. 打开 **设置 > RSSHub**。
2. 每行填写一个 RSSHub 实例基础地址，例如 `https://rsshub.example.com`。
3. 可以配置多个地址；以 `#` 开头可暂时停用某个地址。
4. 添加订阅时可输入完整 RSSHub 地址、`rsshub://` 路由，或使用 Bilibili/Telegram 快捷入口。

**RSSHub 详情** 页面会显示实例命中情况、刷新失败统计和失败的订阅源，便于排查不稳定实例。

### 6. 开启访问认证

打开 **设置 > 访问认证**，设置用户名和密码并启用认证。公开部署前应先完成此步骤，或在反向代理层增加认证；未开启认证的实例可以被任何能访问监听端口的人使用。

### 7. 备份与恢复

点击 **设置 > 备份数据** 可立即备份。开启 **自动备份** 后，yarr 每天零点生成一次备份。

备份目录位于数据库同目录的 `backups/YYYY-MM-DD/`，包含：

- `storage.db`：完整数据库，可用于恢复订阅、文章、阅读状态和设置。
- `subscriptions.opml`：订阅源和文件夹清单。

恢复完整数据时先停止 yarr，用备份的 `storage.db` 替换当前数据库，再重新启动。操作前建议额外保留一份当前数据库。

### 键盘快捷键

| 按键 | 功能 |
| --- | --- |
| `1` / `2` / `3` | 显示全部 / 未读 / 收藏 |
| `j` / `k` | 下一篇 / 上一篇文章 |
| `l` / `h` | 下一个 / 上一个订阅源 |
| `q` | 关闭文章 |
| `r` | 切换已读/未读 |
| `R` | 当前列表全部标为已读 |
| `s` | 切换收藏状态 |
| `o` | 在新窗口打开原文 |
| `i` | 切换正文模式 |
| `f` / `b` | 向前 / 向后滚动正文 |

## 部署教程

### 方案一：Docker

仓库提供多阶段 Dockerfile。先在源码根目录构建镜像：

```bash
docker build -t yarr:local -f etc/dockerfile .
mkdir -p data
```

启动容器并持久化数据库：

```bash
docker run -d \
  --name yarr \
  --restart unless-stopped \
  -p 127.0.0.1:7070:7070 \
  -v "$PWD/data:/data" \
  yarr:local
```

此时数据库保存在宿主机的 `./data/yarr.db`，备份保存在 `./data/backups/`。查看运行状态：

```bash
docker logs -f yarr
```

使用 Docker Compose 时，可直接使用 GHCR 中的最新镜像：

```yaml
services:
  yarr:
    image: ghcr.io/lizhian/yarr:latest
    container_name: yarr
    restart: unless-stopped
    pull_policy: always
    ports:
      - "7070:7070"
    volumes:
      - ~/yarr:/data
      - /etc/localtime:/etc/localtime:ro
      - /etc/timezone:/etc/timezone:ro
    environment:
      - TZ=Asia/Shanghai
      - NUM_WORKERS=4
```

然后运行：

```bash
docker compose up -d
```

订阅和文章数据保存在宿主机的 `~/yarr` 目录中。`7070:7070` 会在所有网络接口上开放端口，应启用 yarr 的访问认证并配置防火墙；若仅通过本机反向代理访问，可将端口映射改为 `127.0.0.1:7070:7070`。

#### 通过 Cloudflare WARP 访问订阅源

yarr 的 HTTP 客户端会读取 `HTTP_PROXY`、`HTTPS_PROXY` 和 `NO_PROXY` 环境变量。可以将 [warp-docker](https://github.com/cmj2002/warp-docker) 作为同一 Docker Compose 网络中的代理，让订阅刷新和正文抓取请求通过 Cloudflare WARP 访问：

```yaml
services:
  warp:
    image: caomingjun/warp:latest
    container_name: warp
    restart: unless-stopped
    device_cgroup_rules:
      - "c 10:200 rwm"
    cap_add:
      - MKNOD
      - AUDIT_WRITE
      - NET_ADMIN
    sysctls:
      net.ipv6.conf.all.disable_ipv6: "0"
      net.ipv4.conf.all.src_valid_mark: "1"
    environment:
      WARP_SLEEP: "2"
      # WARP_LICENSE_KEY: "your-warp-plus-key"
    volumes:
      - ./data/warp:/var/lib/cloudflare-warp
    expose:
      - "1080"

  yarr:
    image: ghcr.io/lizhian/yarr:latest
    container_name: yarr
    restart: unless-stopped
    pull_policy: always
    depends_on:
      warp:
        condition: service_healthy
    ports:
      - "127.0.0.1:7070:7070"
    volumes:
      - ./data/yarr:/data
      - /etc/localtime:/etc/localtime:ro
      - /etc/timezone:/etc/timezone:ro
    environment:
      TZ: Asia/Shanghai
      HTTP_PROXY: http://warp:1080
      HTTPS_PROXY: http://warp:1080
      NO_PROXY: localhost,127.0.0.1,::1
```

启动服务并检查状态：

```bash
docker compose up -d
docker compose ps
docker compose exec warp \
  curl --socks5-hostname 127.0.0.1:1080 \
  https://cloudflare.com/cdn-cgi/trace
```

最后一条命令输出 `warp=on` 或 `warp=plus` 时，表示 WARP 已正常连接。`WARP_LICENSE_KEY` 仅用于 WARP+，使用免费 WARP 时无需配置。`./data/warp` 保存 WARP 的注册身份和配置，重建容器后可以继续使用；更换 License Key 时，应先停止服务并清空该目录再重新注册。yarr 的数据库和备份保存在 `./data/yarr`。

WARP 的 `1080` 端口只在 Compose 内部网络中提供给 yarr，没有映射到宿主机。如果部分内网订阅源不应经过 WARP，可将相应域名或 IP 加入 `NO_PROXY`。WARP 只改变出口 IP，不能保证消除 HTTP 429：目标站点仍可能按照账号、共享出口 IP 或请求频率进行限流。

### 方案二：Linux 原生二进制与 systemd

安装程序并创建独立运行用户：

```bash
sudo useradd --system --home /var/lib/yarr --shell /usr/sbin/nologin yarr
sudo install -Dm755 yarr /usr/local/bin/yarr
sudo install -d -o yarr -g yarr /var/lib/yarr
```

创建 `/etc/systemd/system/yarr.service`：

```ini
[Unit]
Description=yarr RSS reader
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=yarr
Group=yarr
WorkingDirectory=/var/lib/yarr
ExecStart=/usr/local/bin/yarr -addr 127.0.0.1:7070 -db /var/lib/yarr/storage.db
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
```

加载并启动服务：

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now yarr
sudo systemctl status yarr
```

查看日志：

```bash
journalctl -u yarr -f
```

### 配置 Nginx 反向代理

下面的配置将域名转发到只监听本机的 yarr：

```nginx
server {
    listen 80;
    server_name rss.example.com;

    location / {
        proxy_pass http://127.0.0.1:7070;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

生产环境建议为域名配置 HTTPS。也可以不使用反向代理，直接通过 `-cert-file` 和 `-key-file` 让 yarr 提供 HTTPS；证书和私钥必须同时指定。

若需要部署在 `/yarr` 这类子路径下，请为进程增加 `-base /yarr` 或环境变量 `YARR_BASE=/yarr`，并让反向代理保留该路径前缀。

### 升级

升级前先备份数据库。

- Docker Compose：执行 `docker compose pull && docker compose up -d` 拉取最新镜像并重新创建容器。只要数据目录仍挂载到 `/data`，数据库不会随容器删除。
- 原生二进制：停止服务、替换 `/usr/local/bin/yarr`，然后重新启动。yarr 启动时会自动执行所需的 SQLite schema migration。

```bash
sudo systemctl stop yarr
sudo install -m755 ./yarr /usr/local/bin/yarr
sudo systemctl start yarr
```

## 命令行参数与环境变量

命令行参数均有对应环境变量；显式传入的命令行参数会覆盖环境变量提供的默认值。

| 参数 | 环境变量 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `-addr` | `YARR_ADDR` | `127.0.0.1:7070` | 服务监听地址；也支持 `unix:/path/yarr.sock` |
| `-base` | `YARR_BASE` | 空 | 服务 URL 的子路径，例如 `/yarr` |
| `-db` | `YARR_DB` | 可执行文件同目录的 `storage.db` | SQLite 数据库路径 |
| `-cert-file` | `YARR_CERTFILE` | 空 | HTTPS 证书路径 |
| `-key-file` | `YARR_KEYFILE` | 空 | HTTPS 私钥路径 |
| `-log-file` | `YARR_LOGFILE` | 空 | 日志文件路径；为空时输出到标准输出 |
| `-open` | 无 | `false` | 启动后在默认浏览器中打开页面 |
| `-version` | 无 | `false` | 显示版本和 Git 提交信息 |

`NUM_WORKERS` 环境变量用于设置订阅刷新和 RSSHub 可用性探测的并发 worker 数量，默认为 `4`；该值必须是正整数，没有对应的命令行参数。

查看当前版本支持的完整参数：

```bash
yarr -h
```

## 第三方客户端 API

### Fever API

接口地址为：

```text
http(s)://your-yarr.example.com/fever/
```

已知可配合 Reeder、ReadKit、Fluent Reader、Unread 和 Fiery Feeds 等客户端使用。不同客户端对 URL 末尾 `/` 的要求可能不同，连接失败时可分别尝试带或不带末尾斜杠的地址。详见 [Fever API 文档](doc/fever.md)。

### Google Reader API

yarr 实现了 FreshRSS 兼容的常用 Google Reader API 子集，接口地址为：

```text
http(s)://your-yarr.example.com/api/greader.php
```

使用 yarr 访问认证中配置的用户名和密码。详见 [Google Reader API 文档](doc/greader.md)。

## 从源码构建与开发

### 环境要求

- Go 1.23 或更高版本（项目 toolchain 为 Go 1.23.5）。
- GCC、Clang 或其他 C 编译器。SQLite 驱动依赖 CGO。
- Make。
- Zig 0.14.0 或更高版本仅用于部分跨平台命令行构建。
- binutils 仅用于构建 Windows GUI 版本。

获取源码并构建当前平台命令行版本：

```bash
git clone https://github.com/lizhian/yarr.git
cd yarr
make host
./out/yarr -open
```

常用开发命令：

```bash
make test   # 运行完整测试
make host   # 构建当前平台二进制到 out/yarr
make serve  # 使用 local.db 启动 debug server，并从磁盘读取前端资源
make        # 运行测试并构建当前平台二进制
```

项目已将 Go 依赖提交到 `vendor/`。Makefile 会自动启用 SQLite 所需的 `sqlite_foreign_keys` 和 `sqlite_json` build tags，建议优先使用上述目标。更多平台构建方式见[构建文档](doc/build.md)。

## 数据与安全说明

- 数据库包含订阅、文章、访问凭据和阅读状态，请像保护其他应用数据一样限制文件权限并妥善备份。
- 不要在未启用访问认证或其他访问控制时将服务直接暴露到公网。
- 反向代理、TLS 和防火墙配置取决于实际部署环境；公网服务应使用 HTTPS。
- 删除数据库会丢失全部本地数据。升级或恢复前应先停止进程，避免在 SQLite 正在写入时直接覆盖文件。

## 相关文档

- [构建说明](doc/build.md)
- [Fever API](doc/fever.md)
- [Google Reader API](doc/greader.md)
- [设计说明](doc/rationale.txt)
- [更新记录](doc/changelog.md)

## 致谢

界面图标来自 [Feather](https://feathericons.com/)。

## 许可证

本项目使用 MIT License，详见 [license](license)。
