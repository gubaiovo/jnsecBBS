# JNSEC BBS (Go + React)

[![CI](https://github.com/exzco/BBS/actions/workflows/ci.yml/badge.svg)](https://github.com/exzco/BBS/actions/workflows/ci.yml)

原 Flarum（PHP）论坛的重写版：Go 后端（REST API + 单静态二进制）+ Bun/React 前端（Vite + shadcn/ui + Tailwind CSS v4 + lucide-react 图标）。SPA 产物由 Go 二进制直接内嵌伺服，保证旧链接（`/t/{slug}`、`/d/{id}`）与用户密码（bcrypt，含 PHP `$2y$` 前缀）兼容，可直接从 Flarum 数据库原地导入。

内部论坛语义：未登录仅见欢迎页，版块与帖子内容需登录后访问（前端路由守卫 + 后端接口 401 双层控制）。

架构与阶段规划见 [PLAN.md](PLAN.md)。

## 功能

- 版块（两级 Tag，颜色/描述/排序/按组授权可见与发帖），URL 与 Flarum 一致
- 帖子/回复（Markdown 编辑器带预览，置顶/锁定/隐藏，编辑记录，分页）
- 用户（邀请码或验证码注册、头像上传、签名档、用户组：管理员/版主/成员/游客）
- 通知（回复、@提及、点赞），铃铛未读数实时轮询
- 帖子点赞、全文搜索（关键词高亮）
- 管理后台：版块/用户/邀请码/已删除帖/站点设置（含未登录欢迎语）
- 深色/浅色主题切换（跟随系统）
- 安全：CSRF（SPA 走 X-CSRF-Token 头）、XSS 白名单消毒（goldmark + bluemonday）、bcrypt、按 IP 限速、HttpOnly 会话

## 快速开始（本地开发）

前端（Bun 1.1+）：

```bash
cd frontend
bun install
bun run build     # 产物输出到 internal/frontend/dist，随后被 go:embed 打进二进制
```

后端：

```bash
go build -o bbs.exe ./cmd/bbs
bbs.exe serve                # 默认 0.0.0.0:8080，SQLite ./bbs.db
bbs.exe seed-admin -username admin -email admin@example.com -password admin12345
```

打开 http://127.0.0.1:8080 。默认注册方式为邀请码：先登录管理员，到 `/admin` → 邀请码 生成。

前端开发模式（热更新；API 代理到本机 8080 的 Go 服务，见 `vite.config.ts`）：

```bash
cd frontend && bun dev
```

## 配置

复制 `config.example.yaml` 为 `config.yaml`。要点：

| 项 | 说明 |
|---|---|
| `database.driver` | `sqlite`（默认，本地开发）或 `mysql`（生产） |
| `database.dsn` | MySQL 示例：`user:pass@tcp(127.0.0.1:3306)/bbs?charset=utf8mb4&parseTime=true&loc=Local` |
| `session_secret` | 生产必须改为随机 64 位字符串 |
| `cookie_secure` | HTTPS 反代下设 `true` |
| `registration` | `invite`（邀请码）或 `open`（验证码开放注册） |
| `assets_dir` | 旧 Flarum 静态资源目录，设置后 `/assets/*`（头像/附件）继续可访问 |

所有配置项均可用 `JNSEC_*` 环境变量覆盖（`JNSEC_DB_DSN`、`JNSEC_SESSION_SECRET`、`JNSEC_ASSETS_DIR` 等），容器部署无需挂载配置文件。

## 生产部署（Docker Compose + 三容器灰度）

VPS 上只需 Docker（无需装 Go/Bun）：`git clone` / `git pull` 拉到代码后，`docker compose build` 会在容器内完成前端与后端构建（多阶段 Dockerfile）。

两种取镜像的方式任选：
- **源码构建**（git 部署路径）：`docker compose build`；
- **Release 镜像包**：`wget` + `docker load`（见下）。

`compose.yaml` 为**三容器同一网络**：`mysql`（官方 MySQL 5.7，挂现有数据卷 `bbsmysql`——旧 `flarum` 库原样保留 + 新 `bbs` 库）、`app`（新站，`bbs` 库）、`php`（旧站 Flarum，`flarum` 库，灰度期并行，全量切换后移除）。PHP 与 Go 两个后端**共享同一个 MySQL 实例**，服务名即旧 Flarum `config.php` 的 host（`mysql`），旧站零配置改动。

镜像不发布到任何 Registry，部署 = 从 GitHub Release 下载镜像包，`docker load` 导入本地：

```bash
# 1. 下载 Release 附件中的镜像包并导入（导入后本地得到 bbs:latest）
wget https://github.com/exzco/BBS/releases/download/v0.1.0/bbs-image-v0.1.0.tar.gz
docker load -i bbs-image-v0.1.0.tar.gz

# 2. 配置：.env.example 已按生产环境预填（数据库密码、会话密钥、资产路径），
#    复制为 .env 即可（若现有数据卷的 root 密码不同，仅需修改 MYSQL_ROOT_PASSWORD）
cp .env.example .env

# 3. 启动（灰度期 mysql + php + app 三容器；全量切换后移除 php 即双容器）
docker compose up -d
docker compose exec app /app/bbs seed-admin -username admin -email admin@example.com -password admin12345
```

说明：
- `app` 服务使用本地镜像 `bbs:latest`（即上一步 docker load 导入的镜像），不引用任何镜像仓库；`build: .` 仅供有源码的环境自行构建。
- `.env` 含真实密码与密钥，已被 `.gitignore` 排除，不会上传 GitHub。
- `mysql` 服务挂载外部数据卷 `bbsmysql`（复用现有 MySQL 数据）；`wwwdata` 卷同时挂给 `app`（只读，旧头像与附件经 `/assets/*` 继续）与 `php`（旧站站点目录）。
- `php` 使用的 `jnsecbbs:php8.3.36` 镜像只存在于服务器本地，无法从仓库拉取。

## 从 Flarum 到 Go：零迁移并行运行（路线 3）

**Go 后端直接读写 Flarum 原表**（`flarum_users`、`flarum_discussions`、`flarum_posts`、`flarum_tags`、`flarum_groups`、`flarum_group_permission`、`flarum_post_likes`、`flarum_access_tokens`、`flarum_settings`），PHP 与 Go 两个后端操作**同一套表**——数据实时一致，**没有导入步骤、没有停机窗口、没有数据分裂**。

要点：

- **密码互认**：同一列的 bcrypt 哈希（含 PHP `$2y$`），旧账号密码直接登录新站。
- **封禁互认**：flarum/suspend 的 `suspend_until` 即封禁语义，两端互通。
- **版块权限互认**：版块可见/发帖授权读写 `flarum_group_permission`（`tag{id}.viewDiscussions` 等），管理后台的修改对 PHP 端同样生效。
- **会话**：token 存储共享 `flarum_access_tokens`；但浏览器登录态是各自 session（PHP session cookie vs Go cookie），灰度期间用户需在两站分别登录。
- **通知**：Go 端通知存于自建表 `bbs_notifications`（不写 Flarum 的 PHP 序列化格式），从启用日起累计。
- **Go 专属辅助表**（自动创建，Flarum 不感知）：`bbs_notifications`、`bbs_invite_codes`。
- **正文的消毒 HTML 不落库**：Go 每次读取时实时渲染（带内存缓存），对表结构零侵入。

> 本地开发同理：`bbs.exe serve` 用 SQLite 时首次启动会按 Flarum 结构自动建表并预置默认版块。

### 步骤一：准备

```bash
# 1.1 兜底备份（量级很小）
docker exec bbs104-mysql-fix-php2shell sh -c \
  'mysqldump -uroot -p"密码" --single-transaction flarum | gzip' > flarum-backup-$(date +%F).sql.gz

# 1.2 确认旧 Flarum 的数据库 host（决定新编排是否零修改兼容）
docker exec bbs104-php-fix-php2shell grep -n "host" /var/www/bbs/config.php
# 输出 'host' => 'mysql'（官方容器部署惯例）即与 compose.yaml 服务名一致 ✓

# 1.3 确认旧 Flarum 版本 ≥ 1.0（composer.json require.flarum/core ^1.0）
```

### 步骤二：切换到三容器编排（旧站对外无感）

```bash
mkdir -p /opt/bbs && cd /opt/bbs
# 放入 compose.yaml 与 .env（.env.example 已按生产环境预填；
# 若现有数据卷的 root 密码不是 123456，修改 MYSQL_ROOT_PASSWORD 即可）
# 镜像导入见上文「生产部署」第 1 步

# 停止旧 compose 的容器（数据卷由新编排接管）
docker stop bbs104-php-fix-php2shell bbs104-mysql-fix-php2shell

# 启动 mysql + php（旧站照常对外，数据库换由新容器提供）
docker compose up -d mysql php
```
server {
    listen 80;
    server_name bbs.jnsec.org;
    client_max_body_size 20m;

    # 默认指向旧站（PHP）
    set $bbs_backend "http://127.0.0.1:8081";

    # ⬇️ 核心开关 1：访问这个路径，切换到新站(Go)
    location = /switch-to-new {
        add_header Set-Cookie "site_version=go; Path=/; Max-Age=31536000; SameSite=Lax";
        return 302 /;
    }

    # ⬇️ 核心开关 2：访问这个路径，切回旧站(PHP)
    location = /switch-to-old {
        add_header Set-Cookie "site_version=old; Path=/; Max-Age=31536000; SameSite=Lax";
        return 302 /;
    }

    # 正常业务请求
    location / {
        # 如果 Cookie 里带有 site_version=go，则把后端地址修改为新站
        if ($cookie_site_version = "go") {
            set $bbs_backend "http://127.0.0.1:8080";
        }

        # 使用上面动态修改后的变量进行代理
        proxy_pass $bbs_backend;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

宿主机 nginx 接管 80 端口，写入 `/etc/nginx/sites-available/bbs.jnsec.org`。

### 收尾

- 确认稳定后：`flarum` 库改名保留为历史存档，或维持现状（Go 端持续使用）。
- Go 端表结构迁移自动执行（幂等），升级只需替换镜像后 `docker compose up -d`。

## CI/CD（GitHub Actions）
## CI/CD（GitHub Actions）

- **CI**（`.github/workflows/ci.yml`，push/PR 触发）：Go `gofmt` 检查、`go vet`、`go test -race`；前端 `bun install --frozen-lockfile` + `bun run build`（tsc 类型检查）。
- **CD**（`.github/workflows/release.yml`，推送 `v*` 标签或手动触发）：
  1. 用仓库内 Dockerfile 多阶段构建镜像（bun 构建前端 → Go 编译）；
  2. 交叉编译 linux / macOS / Windows 二进制；
  3. 创建 **GitHub Release**，附件包含：镜像 tar 包（`docker save` 导出，服务器 `docker load` 导入）、三平台二进制压缩包（内含 compose、.env 示例）。

## 目录结构

```
cmd/bbs/              入口：serve / seed-admin / import-flarum
frontend/             Bun + Vite + React + shadcn/ui SPA 源码
internal/frontend/    go:embed 内嵌的 SPA 构建产物（bun run build 输出）
internal/config/      配置
internal/models/      GORM 模型
internal/db/          连接、迁移、默认版块
internal/auth/        密码（$2y$ 兼容）、会话、权限
internal/markdown/    goldmark + bluemonday 渲染与 @提及
internal/slug/        URL slug
internal/api/         REST JSON 接口（SPA 数据源）
internal/web/         会话中间件、静态端点（头像/验证码）、SPA 伺服
internal/importer/    Flarum 导入器
deploy/               nginx 配置示例
.github/workflows/    CI 与 Release 工作流
```

## 开发

```bash
go build ./... && go vet ./... && go test ./...
```
