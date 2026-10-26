# JNSEC BBS（Go + React）

Go + React + Flarum 兼容

## 当前状态

| 服务 | 数据与入口 |
|---|---|
|  PHP Flarum | MySQL `flarum` 库，`bbs-php` 容器，源站 8081 |
|  Go  | 独立 MySQL `bbs` 库，`bbs-app` 容器，仅绑定 `127.0.0.1:8080` |
| 数据库 | 复用 `bbs-mysql` / MySQL 5.7 实例，两个独立数据库；没有新增 MySQL 容器 |

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

## 配置与部署
Compose 目录 `/opt/backup/refactorBBS/jnsecBBS`（git 克隆，`main` 分支）。日常由 CICD 自动部署：

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
