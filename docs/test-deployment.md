# 独立可写测试环境

2026-10-26 在 `10.1.63.202` 部署，访问地址 `http://10.1.63.202:8080`。
SSH：`ssh -i ~/.ssh/jnsecbbs-test root@10.1.63.202`。

## 与生产隔离

- 应用代码版本：`d76cfd51c1fec92e2f6446caf6df4c0a0258cc70`。
- 项目目录 `/opt/jnsec-bbs-test`，独立 Compose 项目 `jnsec-bbs-test`。
- 仅部署 Go 应用和 MySQL 5.7；使用兼容结构的 `bbs` 库，不使用 `bbs_v2`。
- `JNSEC_READ_ONLY=false`，测试环境允许业务写入；未安装旧站周期同步任务，测试修改不会回传生产。
- 应用仅绑定测试机私网地址的 8080 端口；MySQL 不向宿主机发布端口。
- 数据库和会话密钥重新生成，仅保存在服务器 root 私有 `.env`，不使用生产配置模板。
- 使用内网 HTTP，Cookie Secure 为 false，可信来源为测试地址；迁移到 HTTPS 时应同步修改。
- 注册策略为 invite；旧用户密码哈希保留，旧登录会话在测试副本清除。

## 构建与启动

测试虚拟 CPU 缺少 SSE4.2，`oven/bun:1.4.4-alpine` 无法在该机器启动。
测试部署使用上述提交中已跟踪、此前验证过的 `internal/frontend/dist`，
通过测试机源码副本中的 `Dockerfile.test` 编译 Go 并打包，不修改生产 Dockerfile。
因此本次成功部署不代表测试机能执行原 Dockerfile 的完整 Bun 构建阶段。
后续前端变更需要在支持 Bun 的机器上从冻结锁文件重新构建，再更新测试镜像。

Docker Hub 访问超时时通过 `docker.m.daocloud.io` 获取基础镜像；系统软件使用
机器配置的清华 Ubuntu 镜像，Go 依赖使用 `goproxy.cn`。

```sh
cd /opt/jnsec-bbs-test
docker compose ps
docker compose logs --tail=100 app
curl --fail http://10.1.63.202:8080/api/health
# 重启现有部署，不重新导入或覆盖测试数据
docker compose up -d
```

不要在测试机直接运行生产 `run.sh`，它带有生产配置同步逻辑。
测试机维护细节另见 `/opt/jnsec-bbs-test/DEPLOYMENT.md`。

## 数据与资源

旧站数据使用只读一致性快照导出，包含所有 35 张源表。原始导入文件保存在测试机
`/root/jnsec-test-import`，仅 root 可读；这些文件包含私人数据，不应提交或公开。
数据库及资源压缩包传输后通过 SHA-256 和 gzip 完整性检查。

本次快照为 147 个用户、907 个主题、12,618 条帖子，旧资源包含 2,045 个文件，
另有 1 个 Go 上传头像。目标库显式执行辅助表迁移；在线应用账号仅有
`bbs.*` 的 SELECT、INSERT、UPDATE、DELETE 权限，没有建表或修改表结构权限。

旧资源解压到项目 `assets` 目录并只读挂载；Go 上传目录使用独立数据卷。
生产资源与测试资源不共享存储。后续重新导入会覆盖测试内容，应另行备份并明确安排，
不要启动每五分钟镜像任务。

## 本次验收

- 健康检查和首页返回 200，`/api/me` 返回 `readOnly=false`。
- 使用临时普通成员账号完成注册、退出、重新登录、发主题、回复、编辑及回读。
- 测试账号、主题、帖子、邀请码、会话及关联行已清理；受测试影响的版块汇总字段
  恢复为快照值，最终仍为 147 个用户、907 个主题、12,618 条帖子、25 个标签。
- Go 上传头像 HTTP 访问正常；旧资源完整保留。
- 应用和 MySQL 健康，未安装论坛数据同步 timer。

该验收不代表所有旧插件功能已经适配；生产仍保持旧站可写、新 Go 站只读的迁移策略。
