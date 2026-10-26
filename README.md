# JNSEC BBS（Go + React）

Go + React + Flarum 兼容。前端内嵌于 Go 二进制。

## 当前线上状态（2026-10-26）

迁移期间**旧站可写、新站只读**，以旧站为唯一数据来源：

| 服务 | 数据与入口 |
|---|---|
| 旧 PHP Flarum | MySQL `flarum` 库，`bbs-php` 容器，源站 8081 |
| 新 Go 论坛 | 独立 MySQL `bbs` 库，`bbs-app` 容器，仅绑定 `127.0.0.1:8080` |
| 影子转换库 | `bbs_v2`，使用独立账号，仅用于 native 数据模型验收，不供在线应用读取 |
| 数据库 | 复用 `bbs-mysql` / MySQL 5.7 实例，没有新增 MySQL 容器 |
| 网关 | Cloudflare HTTPS → 宿主机 nginx → PHP / Go |
| 同步 | `jnsec-bbs-sync.timer` 每五分钟镜像到 `bbs`；`jnsec-bbs-native-sync.timer` 错开两分钟转换到 `bbs_v2` |

公网入口已切换为**子域名分流**：`bbs.jnsec.org` → 旧 PHP 站（8081），`new.bbs.jnsec.org` → 新 Go 站（8080）。nginx 配置以仓库 [`deploy/nginx.conf.example`](deploy/nginx.conf.example) 为准，由部署脚本每次自动同步并 reload（需在 Cloudflare 为新子域添加 DNS 记录）；此前的 `site_version` Cookie 灰度已被取代。两站登录独立。新站页面显示只读提示，并链接到旧站进行发布和修改。

新站允许登录、浏览和退出；禁止注册、发布、回复、编辑、点赞、修改账号/头像、标记通知和后台修改。前端隐藏或禁用写入入口，后端统一返回 `403` / `code=read_only`。新站数据库账号 `bbs_go_ro` 仅有 `bbs.*` SELECT，以及 `flarum_access_tokens` 的 INSERT/UPDATE/DELETE；应用启动不执行 DDL 或默认数据写入。

## 已完成的重构

- `internal/flarum` 还原 s9e XML 原文，兼容旧 Go JSON；goldmark 渲染 Markdown 后经 bluemonday 消毒，不把消毒 HTML 回写旧数据。历史 9,288 条评论的源文本 SHA-256 与旧 PHP Unparser 逐条一致。
- 修复 Flarum 头像文件名到 `/assets/avatars/` 的映射、头像替换/删除顺序，以及不存在的头像/资源/API 被返回 HTML 200 的问题。
- 修正用户组/版块关联字段；补齐本地权限表。详情、搜索、个人页、通知、多版块列表统一过滤权限，继承父版块限制，区分发帖与回复授权，权限读取失败时拒绝访问。
- `init-db` 支持独立空 MySQL/SQLite 初始化，拒绝已有表的数据库；新安装不植入俱乐部专用版块。
- `audit-content` 只读输出评论 ID 与还原正文的 SHA-256，用于迁移核对。
- 旧库最佳答案扩展缺省值修复位于 `deploy/migrations/20261025-best-answer-default.sql`，仅补充默认值 0，不修改历史记录。
- 实名与博客链接扩展表由显式数据库迁移管理；只读应用启动不建表。`bbs_user_profiles` 与 `bbs_schema_migrations` 属于目标库，镜像同步保留它们。
- 此前生产 CSRF 密钥已轮换；后续 owner 决策将实际配置纳入模板，风险及部署规则见下文。页面沿用现有布局，补全 Markdown 标题样式与只读提示。

## 自动部署（CICD）

推送到 `main` 后，CI 完成 Go 检查和前端构建，SSH 只发送 `deploy <提交SHA>`。
`run.sh` 严格解析强制命令中的 `SSH_ORIGINAL_COMMAND`，确认该 SHA 仍为远程 main，
再以 Git archive 固定构建输入；不会把检查结束后出现的新提交或未跟踪文件混入构建。
CI 和 Docker 均使用固定 Bun 1.4.4 与 `bun install --frozen-lockfile`。

- GitHub Secrets：`VPS_HOST` / `VPS_USER` / `VPS_SSH_KEY` / `VPS_FINGERPRINT`。指纹必须对应服务器实际协商算法；现有部署使用 ECDSA。
- VPS 的 authorized_keys 强制命令仍可使用 `cd /opt/backup/refactorBBS/jnsecBBS && chmod +x run.sh && ./run.sh`。新版脚本从原始 SSH 命令提取 SHA，不执行远端传入的任意 shell 文本。
- 构建、应用健康检查或 nginx 验证失败时，恢复原镜像和配置；失败回滚无法完成时保留备份并报错。健康检查使用 `/api/health`，数据库不可连返回 503。
- 保留此前 owner 明确决定的配置同步行为：每次部署从 `.env.example` / `config.example.yaml` 覆盖生产配置。公开模板携带实际配置的风险仍存在，本轮没有擅自轮换凭据。
- 管理员部署已审阅 PR：在普通管理员会话运行 `./run.sh --reviewed <完整SHA>`；受限部署密钥无法使用此模式。自动部署只接受当前 main 的确切提交。
- 首次安装新版部署脚本应由管理员完成；旧脚本尚不识别新 CI 的提交绑定协议。

## 五分钟同步脚本

脚本：[`scripts/sync-flarum.py`](scripts/sync-flarum.py)。仅依赖宿主机 Python 3、Docker CLI，以及现有 MySQL 容器内的 `mysql` / `mysqldump`。密码从容器已有 `MYSQL_ROOT_PASSWORD` 环境读取，不写入脚本或命令参数。源库只执行查询/快照，写入目标必须是不同数据库；拒绝系统库和非法标识符。

行为：

1. 检查镜像范围的表集合、列和索引，所有表必须为 InnoDB。目标允许独有的 `bbs_user_profiles`、`bbs_schema_migrations`；`bbs_notifications`、`bbs_invite_codes` 若源库已有则仍以源为准，否则作为目标本地表保留。未知目标表或镜像结构漂移仍会阻止同步。
2. 使用 `mysqldump --single-transaction` 获取一致性快照；快照临时文件权限受限，结束后删除。
3. 目标端在单个事务内 DELETE + INSERT；初始化不复制源触发器，避免重复执行业务逻辑。不使用 TRUNCATE、LOCK TABLES 或 `--force`。失败自动回滚，读取方看到提交前或提交后的完整版本。
4. 以旧库为准同步新增、修改和删除。**会覆盖新库论坛数据，必须保持只读模式。**
5. 保留目标 `flarum_access_tokens`，新站会话不会每五分钟消失。该表只复制结构，不复制旧站会话；拆库后的首次访问需要重新登录。
6. 其他 34 张表全部复制，包括帖子、用户、关系表、旧通知、扩展记录和已有 Go 辅助数据。保存到新库不等于新 UI 已支持每一种扩展。
7. 使用文件锁避免同一计划任务重入，在 `_bbs_sync_state` 保存最后成功时间和逐表行数；日志只记录计数与耗时，不输出帖子正文或密码。

这是**全量快照刷新**，不是双向复制或增量 CDC。本次实测约 15 万行、35 MB 数据，约 15 秒完成，适合当前五分钟周期。没有把新站改动同步回旧站的逻辑。

初次使用（目标必须为空；先备份）：

```sh
sudo python3 scripts/sync-flarum.py --source flarum --target bbs \
  --read-only-target --initialize
```

后续手动同步：

```sh
sudo python3 scripts/sync-flarum.py --source flarum --target bbs --read-only-target
```

线上安装位置：`/opt/bbs-sync/sync-flarum.py`；配置 `/etc/jnsec-bbs-sync.conf`（仅容器和数据库名），状态目录 `/var/lib/jnsec-bbs-sync`。systemd 单元在 `deploy/systemd/`：

```sh
sudo systemctl status jnsec-bbs-sync.timer
sudo systemctl list-timers jnsec-bbs-sync.timer
sudo journalctl -u jnsec-bbs-sync.service -n 20 --no-pager
sudo systemctl start jnsec-bbs-sync.service   # 立即执行一次
sudo systemctl stop jnsec-bbs-sync.timer    # 暂停后续执行
```

迁移结束后，先停止并禁用 timer，等待运行中的 service 完成，再做最终核对。若决定开放新站写入，还需给应用账号授予必要写权限并设 `READ_ONLY=false`；**绝不能在同步仍启用时开放新站写入**。

## 独立 schema 影子迁移

新增 `bbs native-import`，将旧 Flarum 核心数据转换到独立 `bbs_v2` schema，保留旧 ID、
密码哈希、原始正文和完整源表归档。它可以重复刷新，并保留目标本地会话和资料。
影子库不供在线应用读取，当前 Go 站仍使用 `bbs` 兼容库。

数据模型、配置、五分钟影子任务和切换前提见 [docs/native-migration.md](docs/native-migration.md)。
**不能直接把应用 DSN 改成 `bbs_v2`：native 运行时与扩展语义适配仍未完成。**

## 开发与独立安装

```sh
cd frontend
bun install --frozen-lockfile
bun run build
cd ..
go build -o bbs ./cmd/bbs
export JNSEC_SESSION_SECRET="$(openssl rand -hex 32)"
./bbs init-db # 仅用于空库，已有数据库不要执行
./bbs serve
```

默认 SQLite `./bbs.db`，新安装先显式执行 `./bbs init-db`；默认非只读。使用配置示例时请按场景设置 `read_only`，它是生产迁移模板。独立 MySQL 安装：创建空库并配置 DSN，以非只读模式执行 `./bbs init-db`，然后创建管理员和启动服务：

```sh
./bbs seed-admin -username admin -email admin@example.com -password '自行设置强密码'
```

`seed-admin` 在只读模式下拒绝执行。不要对已导入的 Flarum 数据库运行 `init-db`。

已有目标库由管理员使用独立迁移凭据执行 `./bbs migrate-db -c <配置文件>`；
它只升级 Go 辅助表并登记 `bbs_schema_migrations`，不修改 Flarum 业务表。
不要将该命令指向旧站源库。在线应用继续使用只读账号，启动只校验，不执行 DDL 或种子写入。

## 配置与部署

`config.yaml` 可由 `JNSEC_*` 环境变量覆盖；生产 Compose 使用 `.env`（由 `.env.example` 同步而来，见"自动部署"一节的入库决策说明）。

| 配置 | 用途 |
|---|---|
| `APP_DB_DSN`（Compose）/ `JNSEC_DB_DSN` | Go 数据库 DSN，线上指向 `bbs`；旧 PHP 仍指向 `flarum` |
| `READ_ONLY`（Compose）/ `JNSEC_READ_ONLY` | 迁移只读模式，线上 true |
| `JNSEC_SESSION_SECRET` | 独立随机 CSRF 密钥 |
| `JNSEC_COOKIE_SECURE` | 线上 true，Cloudflare 提供公网 HTTPS |
| `JNSEC_SITE_NAME` | 无数据库站点名称时的默认值 |
| `JNSEC_DATA_DIR` | Go 上传数据，线上 `/app/data` |
| `JNSEC_ASSETS_DIR` | 旧资源路径，线上 `/flarum/public/assets` |
| `JNSEC_REGISTRATION` | 非只读时的 invite/open 注册策略 |

服务器 `139.162.121.96`；Compose 目录 `/opt/backup/refactorBBS/jnsecBBS`（Git 检出，部署时固定到验收提交）。日常由 CICD 自动部署；管理员手动部署也只替换 app，不重启旧 PHP/MySQL：

```sh
./run.sh <当前远程main的完整提交SHA>
```

`wwwdata` 给 PHP 挂到 `/var/www/bbs`，给 Go 只读挂到 `/flarum`；两站共享旧头像和附件文件，无需每五分钟重复复制资源文件。`appdata` 保留 Go 原上传文件。MySQL 外部卷为 `bbsmysql`。

nginx 源站配置 `/etc/nginx/sites-available/bbs.jnsec.org`，80 端口按域名转发 `8080` / `8081`。外部 Cloudflare 提供 HTTPS，因此不能仅根据源站 HTTP 就关闭 Secure Cookie。本机 VPN 需绕过时，可通过 SSH ProxyCommand 将 TCP socket 的 `IP_BOUND_IF` 绑定实际物理网卡（本次 en0）。

## 备份与回滚

本次备份目录 `/opt/backup/bbs-refactor-20261025`（root 私有）：数据库备份、拆库前 `.env` / Compose、nginx 配置、校验和与发布 bundle。拆库前程序镜像 `bbs:pre-mirror-20261025`。

回到拆库前的共享库部署：停止 timer 并等待 service 退出，恢复 `env.before-mirror` 和 `compose.before-mirror.yaml` 到原 Compose 目录，执行 `docker tag bbs:pre-mirror-20261025 bbs:latest`，再 `docker-compose up -d --no-deps app`。源 `flarum` 从未被同步脚本覆盖；不要删除它或其资源卷。旧站仍通过 `bbs.jnsec.org` 提供服务；恢复网关时应使用对应备份配置，不依赖已移除的 Cookie 切换入口。

## 2026-10-26 部署验收

- 已部署代码提交 `a37c8dd`，镜像带确切 Git revision 标签；应用及数据库健康检查通过。后续验收文档提交不改变已部署代码。
- 已在目标 `bbs` 执行辅助表迁移，版本为 1、2。原镜像同步在新增目标本地表后继续成功。
- 已初始化隔离的 `bbs_v2`，源账号仅 SELECT 旧库，目标账号仅授权影子库；实际连接配置保存于 root 私有的 `/etc/jnsec-bbs-native-sync.conf`。
- 首轮及重复影子转换成功；重复刷新实测约 130 秒。两条五分钟 timer 均已启用，原镜像与影子转换相互独立。
- 验收快照中，源库、在线镜像与影子库均为 12,590 条帖子；全部源正文哈希与影子原文匹配，35 张源表完成归档。行数随旧站使用变化，以任务成功记录为准。
- 源站 nginx 验收登录、首页/最新列表、历史正文、头像、活动统计、管理员只读拒绝、跨同步会话及退出。临时账号在三个库均为 0，测试数据库已删除，未发布测试内容。
- 冻结依赖构建的 CSS/JS 文件名哈希与原版本相同；本轮未修改前端源码和跟踪的产物。
- 旧站公网 HTTPS 返回 200。`new.bbs.jnsec.org` 验收时尚无 DNS 解析，因此新站已验收的是源站 Host 路由，公网子域名还需补 DNS。
- 新备份位于 `/opt/backup/bbs-refactor-20261026`，包含部署前数据库、环境、Compose 和 nginx；原应用镜像保留为 `bbs:pre-native-20261026`，部署脚本也保留 `bbs:rollback`。影子库回退只需停用影子 timer，不切换在线 DSN。

## 验证与后续工作

Go race/vet、前端冻结依赖构建、MySQL 5.7/8.4 转换及回滚测试已通过。部署验收见上节。

```sh
go test -race ./...
go vet ./...
python3 -m unittest discover -s scripts/tests -v
```

原始扩展数据已归档，但 native 运行时、附件、通知及第三方权限语义仍需适配。
下一阶段见 [PLAN.md](PLAN.md)，独立 schema 说明见 [docs/native-migration.md](docs/native-migration.md)。
