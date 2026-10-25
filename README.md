# JNSEC BBS (Go + React)

Go 论坛服务与 React 前端，兼容现有 Flarum 用户、版块、帖子及链接。前端构建产物内嵌于 Go 二进制。未登录仅能访问欢迎页，内容接口要求登录。

## 当前进度（2026-10-25）

本轮修复：

- 历史正文实际上是 s9e TextFormatter XML（`<r>` / `<t>`），不是 JSON。`internal/flarum` 负责还原原始 Markdown，保留标记、换行与实体；旧 Go JSON 格式仍可读取。输出通过 goldmark + bluemonday 渲染和消毒。
- 不存在的头像、资源和 API 返回真实 404，避免 SPA HTML 被误当成图片。
- Flarum 头像字段实际存储文件名，Go 现在补齐 `/assets/avatars/`，同时兼容已有 `/avatars/`、`/assets/` 和 HTTP(S) URL；头像更新失败时不会误删旧文件。
- 修正用户组和版块关联的 `group_id` / `tag_id` 映射；SQLite 初始化补齐 `flarum_group_permission`。
- 增加 `init-db`，可在独立空 MySQL/SQLite 库启动，不必先部署 PHP。命令拒绝任何已有表的数据库。新安装仅预置通用版块，站点名使用配置或后台设置，不再自动植入俱乐部版块。
- 详情、搜索、个人页、通知和多版块列表统一过滤版块权限；继承父版块权限，发帖与回复授权分开，权限读取失败时拒绝访问。私密扩展内容不对普通内容接口开放。
- 增加只读 `audit-content`：输出评论 ID 与还原正文的 SHA-256，可与旧 PHP Unparser 比较，无需导出明文帖子。

仍存在的兼容边界（不能视为完整 Flarum 框架替代）：

- 新写入正文采用有效的 s9e `<t>` 文本封装。Go 渲染 Markdown；旧 PHP 站显示原始 Markdown，直到 PHP formatter 重新解析。尚未实现完整 s9e 富文本写入及扩展插件协议。
- 历史 XML 不回写、不批量替换。新旧站仍共享 Flarum 表，当前无需数据搬迁；这不代表全部 Flarum 扩展已迁移。
- 旧通知保留在 Flarum 表；Go 通知使用独立 `bbs_notifications`，未导入 PHP 序列化通知。系统事件帖保留在库中，当前 UI 仅展示 comment。
- Go 上传头像仍存于独立 `appdata`；旧头像通过只读 `wwwdata` 访问。旧 PHP 站对 Go 新上传头像的互通仍需后续处理。
- 全局权限和第三方权限扩展、历史通知与插件数据需要继续专项核对；不宣称已完成全量安全审计。

## 本轮验证与发布

- `go test -race ./...`、`go vet ./...` 通过；最后的静态路由修改另通过 `go test -race ./internal/web` 与 `go vet ./internal/web`。
- 全部 9,288 条历史评论还原正文的 SHA-256 与原 PHP s9e Unparser 逐条一致；生产全部 12,545 条帖子（包括事件）保留。
- MySQL 副本验证登录、用户组、历史帖子、Markdown/XSS、发帖、编辑、回复、点赞，以及头像上传/替换/删除与文件清理。独立空 MySQL 库初始化通过，重复初始化拒绝执行。
- 公网 HTTPS 验证新旧站 Cookie 切换、CSRF、Secure 登录会话、退出、历史帖、PNG 头像、预览、404 与匿名 401。
- 线上 Go 更新为 `bbs:79b3232`，`bbs:latest` 指向该镜像；PHP/MySQL 未重启。CSRF 密钥已替换为独立随机值，用户若遇到旧页面 CSRF 失败请刷新后重试。
- 测试写入在隔离数据库中完成；生产临时账号只用于登录和读取，未发布内容。验证后删除测试账号、会话与隔离数据库。
- 服务器源代码快照位于 `/opt/backup/refactorBBS/releases/20261025` Git worktree；原部署目录中的 `run.sh` 等未提交文件保留。前端资源未修改，继续使用仓库内嵌产物。

## 开发与独立安装

```sh
cd frontend
bun install --frozen-lockfile
bun run build
cd ..
go build -o bbs ./cmd/bbs
export JNSEC_SESSION_SECRET="$(openssl rand -hex 32)"
./bbs serve
```

默认 SQLite `./bbs.db`，首次启动自动建表。前端热更新使用 `cd frontend && bun dev`。管理账号由 `seed-admin` 创建（密码请自行设置）：

```sh
./bbs seed-admin -username admin -email admin@example.com -password '自行设置强密码'
```

独立 MySQL 安装：先创建空数据库和专用用户，配置 `JNSEC_DB_DRIVER=mysql` 与 `JNSEC_DB_DSN`，运行 `./bbs init-db`，再执行 `seed-admin` 和 `serve`。不要对旧 Flarum 库执行 `init-db`。普通 MySQL 启动只检查旧表并创建两张 Go 辅助表，不对旧表运行 GORM AutoMigrate。

```sh
go test -race ./...
go vet ./...
```

## 配置

配置读取 `config.yaml`，环境变量覆盖 YAML。示例文件只提供占位符，生产密码与密钥必须自行填写；不要覆盖现有生产 `.env`。

| 配置 / 环境变量 | 作用 |
|---|---|
| `listen` / `JNSEC_LISTEN` | 默认 `0.0.0.0:8080` |
| `site_name` / `JNSEC_SITE_NAME` | 无数据库站点标题时的默认名称 |
| `database.driver` / `JNSEC_DB_DRIVER` | `sqlite` 或 `mysql`，其他值报错 |
| `database.dsn` / `JNSEC_DB_DSN` | MySQL 示例 `bbs:password@tcp(mysql:3306)/flarum?charset=utf8mb4&parseTime=true&loc=Local` |
| `session_secret` / `JNSEC_SESSION_SECRET` | 每个站点独立随机密钥，`openssl rand -hex 32` |
| `cookie_secure` / `JNSEC_COOKIE_SECURE` | HTTPS 为 true；线上 Cloudflare 提供 HTTPS，当前为 true |
| `trust_proxy` / `JNSEC_TRUST_PROXY` | 仅在受信任网关后启用，应用端口应限制外部访问 |
| `trusted_origins` / `JNSEC_TRUSTED_ORIGINS` | CSRF 信任来源，逗号分隔 |
| `data_dir` / `JNSEC_DATA_DIR` | 新头像存储目录 |
| `assets_dir` / `JNSEC_ASSETS_DIR` | Flarum 静态资源目录，线上 `/flarum/public/assets` |
| `registration` / `JNSEC_REGISTRATION` | `invite` 或 `open` |

## 线上部署拓扑

服务器 `139.162.121.96`，当前 Compose 目录 `/opt/backup/refactorBBS/jnsecBBS`，文件为 **`docker-compose.yaml`**。

- `bbs-mysql`：MySQL 5.7，外部卷 `bbsmysql`，生产数据库 `flarum`。
- `bbs-php`：旧 Flarum，`jnsecbbs:php8.3.36` 为服务器本地镜像，宿主机端口 8081。
- `bbs-app`：Go，镜像 `bbs:latest`（本轮代码 `79b3232`），仅绑定宿主机 `127.0.0.1:8080`，通过 nginx 访问。
- `wwwdata` 同时挂给 PHP 的 `/var/www/bbs` 与 Go 的 `/flarum`（只读）；Go 新头像持久化在 `appdata`。
- 公网由 Cloudflare 提供 HTTPS 并将 HTTP 重定向到 HTTPS；源站 nginx 接收 HTTP，不能据此将 Secure Cookie 关闭。
- 宿主机 nginx 配置 `/etc/nginx/sites-available/bbs.jnsec.org`，80 端口服务 `bbs.jnsec.org`。

当前 nginx 以 Cookie 灰度路由：默认旧 PHP 站；`/switch-to-new` 设置 `site_version=go`，`/switch-to-old` 设置 `site_version=old`，随后 302 到首页。主 `location /` 根据 Cookie 选择 `127.0.0.1:8080` 或 `8081`，转发 Host、X-Real-IP、X-Forwarded-For、X-Forwarded-Proto。新旧站登录会话独立。

本机 VPN 会把目标地址路由到 utun。macOS 可通过 SSH ProxyCommand 的 TCP socket 设置 `IP_BOUND_IF` 绑定实际物理网卡（本次为 en0），同时禁用其他代理；不要仅靠清除 HTTP_PROXY 判断已绕过 VPN。

旧最佳答案扩展的 `best_answer_notified` 为 NOT NULL 且无默认值，Go 建帖会返回 500。仅在具有该列的旧库执行 `deploy/migrations/20261025-best-answer-default.sql`，不要对独立 Go 新库执行该扩展脚本。

更新前备份数据库、`.env`、nginx 配置并保留旧镜像标签。仅重建/替换 app，避免重启旧 PHP 和 MySQL：

```sh
docker compose build app
docker compose up -d --no-deps app
```

服务器旧版环境可使用 `docker-compose`。若从其他机器交叉编译，必须带上已构建的 `internal/frontend/dist`，目标 `CGO_ENABLED=0 GOOS=linux GOARCH=amd64`，再将二进制装入运行镜像。

本次备份目录：`/opt/backup/bbs-refactor-20261025`。回滚镜像：`bbs:rollback-20261025`。回滚 app：先 `docker tag bbs:rollback-20261025 bbs:latest`，再在原 Compose 目录执行 `docker-compose up -d --no-deps app`。本轮仅为最佳答案扩展列补充默认值 0（见 `deploy/migrations/20261025-best-answer-default.sql`），不改历史值。旧 PHP 与旧 Go 均可保留此默认值运行，回退程序无需恢复数据库；需要撤销默认值时参照 SQL 文件中的回滚语句。

## 目录

- `cmd/bbs`：serve、seed-admin、init-db、audit-content。
- `internal/flarum`：Flarum 存储格式兼容边界。
- `internal/models`、`internal/db`：数据模型、初始化与兼容检查。
- `internal/auth`、`internal/api`、`internal/web`：权限、API、会话与静态资源。
- `internal/markdown`：Markdown 渲染和 HTML 消毒。
- `frontend`、`internal/frontend/dist`：React 源码与嵌入式产物。
- `deploy`：网关示例。服务器当前配置以上述实测拓扑为准。

README 记录当前实施进度；PLAN.md 是旧规划，未据此执行本轮工作。
