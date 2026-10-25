# JNSEC BBS（Go + React）

Go 论坛服务、React 前端与 Flarum 数据兼容层。前端产物内嵌于单个 Go 二进制，保留 `/d/{id}`、`/t/{slug}` 等旧链接与 bcrypt 密码。未登录只显示欢迎页。

## 当前线上状态（2026-10-25）

迁移期间**旧站可写、新站只读**，以旧站为唯一数据来源：

| 服务 | 数据与入口 |
|---|---|
| 旧 PHP Flarum | MySQL `flarum` 库，`bbs-php` 容器，源站 8081 |
| 新 Go 论坛 | 独立 MySQL `bbs` 库，`bbs-app` 容器，仅绑定 `127.0.0.1:8080` |
| 数据库 | 复用 `bbs-mysql` / MySQL 5.7 实例，两个独立数据库；没有新增 MySQL 容器 |
| 网关 | Cloudflare HTTPS → 宿主机 nginx → PHP / Go |
| 同步 | `jnsec-bbs-sync.timer` 每五分钟将旧库一致性快照同步到新库 |

公网入口已切换为**子域名分流**：`bbs.jnsec.org` → 旧 PHP 站（8081），`new.bbs.jnsec.org` → 新 Go 站（8080）。nginx 配置以仓库 [`deploy/nginx.conf.example`](deploy/nginx.conf.example) 为准，由部署脚本每次自动同步并 reload（需在 Cloudflare 为新子域添加 DNS 记录）；此前的 `site_version` Cookie 灰度已被取代。两站登录独立。新站页面显示只读提示，并链接到旧站进行发布和修改。

新站允许登录、浏览和退出；禁止注册、发布、回复、编辑、点赞、修改账号/头像、标记通知和后台修改。前端隐藏或禁用写入入口，后端统一返回 `403` / `code=read_only`。新站数据库账号 `bbs_go_ro` 仅有 `bbs.*` SELECT，以及 `flarum_access_tokens` 的 INSERT/UPDATE/DELETE；应用启动不执行 DDL 或默认数据写入。

## 已完成的重构

- `internal/flarum` 还原 s9e XML 原文，兼容旧 Go JSON；goldmark 渲染 Markdown 后经 bluemonday 消毒，不把消毒 HTML 回写旧数据。历史 9,288 条评论的源文本 SHA-256 与旧 PHP Unparser 逐条一致。
- 修复 Flarum 头像文件名到 `/assets/avatars/` 的映射、头像替换/删除顺序，以及不存在的头像/资源/API 被返回 HTML 200 的问题。
- 修正用户组/版块关联字段；补齐本地权限表。详情、搜索、个人页、通知、多版块列表统一过滤权限，继承父版块限制，区分发帖与回复授权，权限读取失败时拒绝访问。
- `init-db` 支持独立空 MySQL/SQLite 初始化，拒绝已有表的数据库；新安装不植入俱乐部专用版块。
- `audit-content` 只读输出评论 ID 与还原正文的 SHA-256，用于迁移核对。
- 旧库最佳答案扩展缺省值修复位于 `deploy/migrations/20261025-best-answer-default.sql`，仅补充默认值 0，不修改历史记录。
- 实名与博客链接扩展表 `deploy/migrations/20261025-user-profiles.sql`：只读镜像阶段需在 `bbs` 库手工执行一次（应用只读启动不建表；未执行时读取降级为空）。
- 生产 CSRF 密钥已轮换；配置模板不再带真实密钥。页面沿用现有布局，补全 Markdown 标题样式与只读提示。

## 首页与个人主页改版（2026-10-25）

- **三栏首页**：左栏九大板块导航（色点 + 帖数，选中态 nanostores 持久化）；中栏当前板块文章列表（默认全部，`/api/recent`；分页加载更多）；右栏"最新文章"排除周报板块（`/api/recent?exclude=weekly`，按名称匹配周报）。
- **内嵌文章详情**：中栏点击整行在 `?d=<id>` 打开详情（复用整页路由组件，旧链接 `/d/{id}-{slug}` 行为不变），"返回列表"/浏览器后退回列表；右栏最新文章同样内嵌打开。
- **移除独立板块主页**：`/t/{slug}` 重定向到首页并选中对应板块（含子板块，父板块在左栏高亮）。
- **个人主页**：GitHub 风格年度贡献热力图（`/api/users/:username/activity`，按北京时间分桶聚合发帖+回复；月份/星期标签、五档色、年份切换）；实名徽章与博客外链（`bbs_user_profiles` 表，编辑入口随只读模式禁用，切写后开放）。
- 设置页新增实名/博客字段并修复签名框首次加载为空的问题；Profile 页区分 401/404 提示；导航栏最左新增后退按钮。

## 自动部署（CICD）

推送到 `main` 即自动部署：CI（gofmt/vet/test + 前端构建）全绿后，deploy job 经 `appleboy/ssh-action` 连接 VPS 执行强制命令 `cd /opt/backup/refactorBBS/jnsecBBS && chmod +x run.sh && ./run.sh`（git pull → 同步配置 → `docker-compose build app` + `up -d --no-deps app` → nginx 子域名配置同步与 reload）。镜像在 VPS 本地构建：npm 走 npmmirror、Go 走 goproxy.cn。

- GitHub Secrets 四个：`VPS_HOST` / `VPS_USER` / `VPS_SSH_KEY`（部署私钥）/ `VPS_FINGERPRINT`（主机指纹）。**指纹必须是握手实际协商算法的值**：Go SSH 客户端对本服务器实测协商 ECDSA（非 ED25519/RSA），换服务器后需用 Go 客户端重测，勿凭经验猜算法。
- VPS 侧 authorized_keys 一行锁死：`restrict,command="cd /opt/backup/refactorBBS/jnsecBBS && chmod +x run.sh && ./run.sh",expiry-time="20270125"` + 公钥。任何一次连接等同触发一次部署。
- `.env.example` 经 owner 决策携带真实生产配置（run.sh 每次部署同步为 `.env`）。**注意：仓库为公开仓库，密钥入库的泄露风险已知悉并接受**；后续如需收敛，在 VPS 轮换密码/密钥后把模板改回 CHANGE-ME 即可。
- 手动部署兜底不变：VPS 上 `docker-compose build app && docker-compose up -d --no-deps app`。

## 五分钟同步脚本

脚本：[`scripts/sync-flarum.py`](scripts/sync-flarum.py)。仅依赖宿主机 Python 3、Docker CLI，以及现有 MySQL 容器内的 `mysql` / `mysqldump`。密码从容器已有 `MYSQL_ROOT_PASSWORD` 环境读取，不写入脚本或命令参数。源库只执行查询/快照，写入目标必须是不同数据库；拒绝系统库和非法标识符。

行为：

1. 检查源/目标表集合、列和索引结构，所有表必须为 InnoDB；结构变化时停止并保留上次成功数据，不能静默遗漏新扩展表。
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

默认 SQLite `./bbs.db`，首次启动建表；默认非只读。使用配置示例时请按场景设置 `read_only`，它是生产迁移模板。独立 MySQL 安装：创建空库并配置 DSN，以非只读模式执行 `./bbs init-db`，然后创建管理员和启动服务：

```sh
./bbs seed-admin -username admin -email admin@example.com -password '自行设置强密码'
```

`seed-admin` 在只读模式下拒绝执行。不要对已导入的 Flarum 数据库运行 `init-db`。

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

服务器 `139.162.121.96`；Compose 目录 `/opt/backup/refactorBBS/jnsecBBS`（git 克隆，`main` 分支）。日常由 CICD 自动部署；手动兜底只替换 app，不重启旧 PHP/MySQL：

```sh
docker-compose build app
docker-compose up -d --no-deps app
```

`wwwdata` 给 PHP 挂到 `/var/www/bbs`，给 Go 只读挂到 `/flarum`；两站共享旧头像和附件文件，无需每五分钟重复复制资源文件。`appdata` 保留 Go 原上传文件。MySQL 外部卷为 `bbsmysql`。

nginx 源站配置 `/etc/nginx/sites-available/bbs.jnsec.org`，80 端口按 Cookie 转发 `8080` / `8081`。外部 Cloudflare 提供 HTTPS，因此不能仅根据源站 HTTP 就关闭 Secure Cookie。本机 VPN 需绕过时，可通过 SSH ProxyCommand 将 TCP socket 的 `IP_BOUND_IF` 绑定实际物理网卡（本次 en0）。

## 备份与回滚

本次备份目录 `/opt/backup/bbs-refactor-20261025`（root 私有）：数据库备份、拆库前 `.env` / Compose、nginx 配置、校验和与发布 bundle。拆库前程序镜像 `bbs:pre-mirror-20261025`。

回到拆库前的共享库部署：停止 timer 并等待 service 退出，恢复 `env.before-mirror` 和 `compose.before-mirror.yaml` 到原 Compose 目录，执行 `docker tag bbs:pre-mirror-20261025 bbs:latest`，再 `docker-compose up -d --no-deps app`。源 `flarum` 从未被同步脚本覆盖；不要删除它或其资源卷。旧站仍可通过 Cookie 切回。

## 验证与后续工作

已执行 Go race 测试、go vet、前端 TypeScript/Vite 构建；验证过历史内容、头像、登录、发布/编辑/回复/点赞，以及只读写接口拦截。同步已通过真实 MySQL 的新增/修改/删除、UTF-8、会话保留、故障回滚和重试测试。

北京时间 2026-10-25 17:20 已观察到 timer 自动成功运行，耗时约 15 秒。公网 HTTPS 验证管理员写操作同样被拒绝，登录会话跨同步保留。清理测试账号后，源库与目标库各有 12,546 条帖子，全部评论正文校验和一致；测试账号在两库均为 0，生产未发布测试内容。可重复运行：

```sh
go test -race ./...
go vet ./...
python3 -m unittest discover -s scripts/tests -v
# 真实 MySQL 集成测试会创建、删除专用测试库；不要用未知容器名
BBS_SYNC_TEST_MYSQL=bbs-mysql python3 -m unittest discover -s scripts/tests -v
```

剩余边界详见 [PLAN.md](PLAN.md)：旧通知和部分扩展已完整保存但尚未接入新 UI；全局权限/第三方权限扩展仍需核对；新站写入格式和新上传头像在旧 PHP 中的完整互通尚未完成。本次只读迁移不会触发这些反向写入问题，不能据此宣称已完整替代所有 Flarum 功能。
