# 独立 schema 与影子转换

## 本轮边界

`bbs native-import` 将 Flarum 快照转换到独立 schema，目标建议使用 `bbs_v2`。
它是迁移工具，不是运行时数据库适配器。当前线上 Go API 仍访问 `bbs` 兼容库，
不能将 `APP_DB_DSN` 直接改成 `bbs_v2`。不创建掩盖旧表依赖的兼容视图。

两个流程并存：

```text
旧站 flarum ──镜像同步──> bbs ──> 当前只读 Go 站
          └──转换同步──> bbs_v2（隔离影子库，验收用途）
```

页面、路由和 API 响应保持不变。影子库不授予在线应用访问权限；切换存储须另行完成
运行时适配、权限与扩展语义验收。

## 数据模型

| 目标表 | 语义 |
|---|---|
| `users`、`groups`、`user_groups` | 保留旧 ID、密码哈希、用户组成员关系；用户列采用 `password_hash` 等明确名称 |
| `permissions` | 将权限拆分为用户组、范围、范围 ID 和动作；未支持的扩展权限仍保存，不因此授权 |
| `tags`、`discussion_tags` | 标签层级及多标签关联 |
| `discussions` | 主题元数据、首末帖、作者、隐藏及审核状态 |
| `posts` | 正文 `body_markdown`、帖子类型、楼层、作者、编辑及隐藏信息 |
| `post_likes` | 点赞关系及时间 |
| `discussion_reads`、`discussion_subscriptions` | 阅读位点与订阅状态分别存储 |
| `legacy_post_contents` | 旧正文原始字节及原文、转换后 Markdown 的 SHA-256 |
| `legacy_tables`、`legacy_rows` | 完整源表定义、列顺序与原始行归档，保留未映射字段和扩展 |
| `sessions`、`user_profiles` | 目标本地表，刷新不覆盖；当前尚未接入在线应用 |
| `migration_state` | 版本、源身份、结构指纹、成功时间及计数 |

原始行归档使用按列顺序排列的数组，非 NULL 值以 Base64 保存原始读取字节，SQL NULL
单独表示；列元数据与建表语句保存在 `legacy_tables`。不要将其当成可公开查询的 API：
里面包含邮箱、密码哈希、会话和私密扩展数据，需要与数据库备份相同的访问控制。

原有头像、附件文件继续保存在原资源卷；影子转换保留引用和扩展记录，不复制文件，
也不宣称已完成附件、旧通知和第三方插件的业务转换。

## 导入与刷新

使用专用数据库账号：源账号只授予 `flarum.*` SELECT；目标账号仅有 `bbs_v2.*`
建表和导入所需权限。不要给目标账号授予源库写权限。

连接参数只通过环境变量提供，不传入命令行，不提交实际密码：

```sh
# 将 deploy/native-sync.conf.example 填写后安装为 root:root、0600 的配置文件。
# 下例由管理员在服务器执行，配置文件必须可信。
set -a
. /etc/jnsec-bbs-native-sync.conf
set +a
/opt/bbs-native/bbs native-import -initialize -read-only-target

# 后续刷新：不加 -initialize
/opt/bbs-native/bbs native-import -read-only-target
```

首次初始化要求目标库为空；后续只允许刷新该工具管理的、绑定同一源身份和结构的目标。
全量转换在源一致性快照中读取；目标管理数据事务替换，失败保留上次完整数据。
结构变化必须人工审查，不通过忽略校验来继续覆盖。

仓库提供 `deploy/systemd/jnsec-bbs-native-sync.{service,timer}`，默认不会被应用部署脚本
自动启用。管理员先完成首次导入与核对，再安装并启用 timer。它每五分钟运行一次，
相对于原镜像任务错开两分钟，减少同时读取源库的负载。

```sh
sudo systemctl daemon-reload
sudo systemctl enable --now jnsec-bbs-native-sync.timer
sudo journalctl -u jnsec-bbs-native-sync.service -n 20 --no-pager
```

## 验收与回退

验收需要同时覆盖：

- 核心记录数量、ID 和关系；全部源表归档行数及原文校验和。
- 修改、删除和新增后的刷新；目标本地会话及资料保留。
- 无效关系、导入失败、源/目标结构漂移时拒绝或回滚。
- 私密、隐藏、审核状态保持；保存权限不等于已实现其授权语义。

影子转换失败不影响当前 Go 站和旧站。停止 `jnsec-bbs-native-sync.timer` 并等待运行中
service 退出即可暂停；保留影子库供排查，不需要切换线上 DSN。

下一步才是将 `internal/store` 扩展为 native 查询实现、改造登录和写入事务、接入必要
扩展、做 API 等价验收。最终切换前需暂停旧站写入、执行最后一次转换、关闭两条同步
任务，再开放新站写入。开放写入后不能直接恢复旧快照覆盖。
