# JNSEC BBS（Go + React）

Go + React + Flarum 兼容

## 当前状态

迁移期间**旧站可写、新站只读**，以旧站为唯一数据来源：

| 服务 | 数据与入口 |
|---|---|
|  PHP Flarum | MySQL `flarum` 库，`bbs-php` 容器，源站 8081 |
|  Go  | 独立 MySQL `bbs` 库，`bbs-app` 容器，仅绑定 `127.0.0.1:8080` |
| 数据库 | 复用 `bbs-mysql` / MySQL 5.7 实例，两个独立数据库；没有新增 MySQL 容器 |
| 网关 | Cloudflare HTTPS → 宿主机 nginx → PHP / Go |
| 同步 | `jnsec-bbs-sync.timer` 每五分钟将旧库一致性快照同步到新库 |

公网入口已切换为**子域名分流**：`bbs.jnsec.org` → 旧 PHP 站（8081），`new.bbs.jnsec.org` → 新 Go 站（8080）。nginx 配置以仓库 [`deploy/nginx.conf.example`](deploy/nginx.conf.example) 为准，由部署脚本每次自动同步并 reload。当前需在 Cloudflare 为新子域添加 DNS 记录；

新站允许登录、浏览和退出；禁止注册、发布、回复、编辑、点赞、修改账号/头像、标记通知和后台修改。前端隐藏或禁用写入入口，后端统一返回 `403` / `code=read_only`。新站数据库账号 `bbs_go_ro` 仅有 `bbs.*` SELECT，以及 `flarum_access_tokens` 的 INSERT/UPDATE/DELETE；应用启动不执行 DDL 或默认数据写入。

## 已完成的重构

- `internal/flarum` 还原 s9e XML 原文，兼容旧 Go JSON；goldmark 渲染 Markdown 后经 bluemonday 净化。历史 9,288 条评论的源文本 SHA-256 与旧 PHP Unparser 逐条一致。
- 修复 Flarum 头像文件名到 `/assets/avatars/` 的映射、头像替换/删除顺序，以及不存在的头像/资源/API 被返回 HTML 200 的问题。
- 修正用户组/版块关联字段；补齐本地权限表。详情、搜索、个人页、通知、多版块列表统一过滤权限，继承父版块限制，区分发帖与回复授权，权限读取失败时拒绝访问。
- `audit-content` 只读输出评论 ID 与还原正文的 SHA-256，用于迁移核对。
- 旧库最佳答案扩展缺省值修复位于 `deploy/migrations/20261025-best-answer-default.sql`，仅补充默认值 0，不修改历史记录。
- 实名与博客链接扩展表 `deploy/migrations/20261025-user-profiles.sql`：只读镜像阶段需在 `bbs` 库手工执行一次（应用只读启动不建表；未执行时读取降级为空）。

## CICD

推送到 `main` 即自动部署：CI（gofmt/vet/test + 前端构建）全绿后，deploy job 经 `appleboy/ssh-action` 连接 VPS 执行命令 `cd /opt/backup/refactorBBS/jnsecBBS && chmod +x run.sh && ./run.sh`（git pull → 同步配置 → `docker-compose build app` + `up -d --no-deps app` → nginx 子域名配置同步与 reload）。镜像在 VPS 本地构建：npm 走 npmmirror、Go 走 goproxy.cn。

- GitHub Secrets 四个：`VPS_HOST` / `VPS_USER` / `VPS_SSH_KEY`（部署私钥）/ `VPS_FINGERPRINT`（主机指纹）。**指纹必须是握手实际协商算法的值**。
- VPS 侧 authorized_keys 一行锁死：`restrict,command="cd /opt/backup/refactorBBS/jnsecBBS && chmod +x run.sh && ./run.sh",expiry-time="20270125"` + 公钥。
- `.env.example` 携带真实生产配置（run.sh 每次部署同步为 `.env`）；后续需收敛，在 VPS 轮换密码/密钥

## 同步脚本

脚本：[`scripts/sync-flarum.py`](scripts/sync-flarum.py)。仅依赖宿主机 Python 3、Docker CLI，以及现有 MySQL 容器内的 `mysql` / `mysqldump`。密码从容器已有 `MYSQL_ROOT_PASSWORD` 环境读取，不写入脚本或命令参数。源库只执行查询/快照，写入目标必须是不同数据库；拒绝系统库和非法标识符。

行为：

1. 检查源/目标表集合、列和索引结构，所有表必须为 InnoDB；结构变化时停止并保留上次成功数据，不能静默遗漏新扩展表。
2. 使用 `mysqldump --single-transaction` 获取一致性快照；快照临时文件权限受限，结束后删除。
3. 目标端在单个事务内 DELETE + INSERT；初始化不复制源触发器，避免重复执行业务逻辑。不使用 TRUNCATE、LOCK TABLES 或 `--force`。失败自动回滚，读取方看到提交前或提交后的完整版本。
4. 以旧库为准同步新增、修改和删除。**会覆盖新库论坛数据，必须保持只读模式。**
5. 保留目标 `flarum_access_tokens`，新站会话不会每五分钟消失。该表只复制结构，不复制旧站会话；拆库后的首次访问需要重新登录。
6. 其他 34 张表全部复制，包括帖子、用户、关系表、旧通知、扩展记录和已有 Go 辅助数据。保存到新库不等于新 UI 已支持每一种扩展。
7. 使用文件锁避免同一计划任务重入，在 `_bbs_sync_state` 保存最后成功时间和逐表行数；日志只记录计数与耗时，不输出帖子正文或密码。

初次使用：

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

服务器 `139.162.121.96`；Compose 目录 `/opt/backup/refactorBBS/jnsecBBS`（git 克隆，`main` 分支）。日常由 CICD 自动部署：

```sh
docker-compose build app
docker-compose up -d --no-deps app
```

`wwwdata` 给 PHP 挂到 `/var/www/bbs`，给 Go 只读挂到 `/flarum`；两站共享旧头像和附件文件，无需每五分钟重复复制资源文件。`appdata` 保留 Go 原上传文件。MySQL 外部卷为 `bbsmysql`。

nginx 源站配置 `/etc/nginx/sites-available/bbs.jnsec.org`，80 端口按 Cookie 转发 `8080` / `8081`。外部 Cloudflare 提供 HTTPS
## 备份与回滚

本次备份目录 `/opt/backup/bbs-refactor-20261025`（root 私有）：数据库备份、拆库前 `.env` / Compose、nginx 配置、校验和与发布 bundle。拆库前程序镜像 `bbs:pre-mirror-20261025`。

回到拆库前的共享库部署：停止 timer 并等待 service 退出，恢复 `env.before-mirror` 和 `compose.before-mirror.yaml` 到原 Compose 目录，执行 `docker tag bbs:pre-mirror-20261025 bbs:latest`，再 `docker-compose up -d --no-deps app`。源 `flarum` 从未被同步脚本覆盖；不要删除其资源卷。旧站仍可通过 Cookie 切回。
```

剩余边界详见 [PLAN.md](PLAN.md)。
