# JNSEC BBS — Go 重写计划

原站基于 Flarum（PHP + Mithril SPA），存在性能（PHP-FPM 每请求全量引导）、可维护性（框架 + 多扩展耦合）与安全性（PHP 攻击面、XSS 面）等问题。本项目使用 Go 从零重写后端，前端采用 Bun + React（shadcn/ui 组件库 + lucide-react 图标）构建现代 SPA，并保证**数据可迁移、旧链接可访问**。

## 一、总体架构决策

| 维度 | Flarum（旧） | 新版 |
|---|---|---|
| 后端 | PHP 8 + Composer 多包 | Go 1.25，单静态二进制（Gin + GORM） |
| 前端 | Mithril SPA（Node 构建） | **Bun + Vite + React 19 + TypeScript**，shadcn/ui 组件库（Radix + Tailwind CSS v4），lucide-react 图标；SPA 产物 go:embed 进二进制 |
| API | JSON:API（内部） | REST JSON（/api/*），会话 Cookie + X-CSRF-Token 头 |
| 数据库 | MySQL | MySQL（生产）/ SQLite（本地开发），GORM |
| 构建 | Composer + Node | `bun run build`（前端）→ `go build`（后端），两步出单文件 |
| 密码 | bcrypt（PHP `password_hash`） | bcrypt（`$2y$` 前缀自动兼容，老用户密码直接沿用） |
| 会话 | access_tokens 表 | 同为服务端随机 token 表，HttpOnly Cookie |
| 富文本 | Markdown（前端渲染） | goldmark 渲染 + bluemonday 白名单消毒后入库，前端直接展示消毒 HTML（编辑器预览走同一条管线） |

## 二、功能范围

### 第一阶段（已交付，Flarum 核心对等 + 现代 SPA 前端）
- **版块（Tags）**：两级版块（大版块/子版块）、颜色、描述、排序、URL 与 Flarum 一致 `/t/{slug}`；按版块设置"谁可看 / 谁可发帖"（内部版块权限）。
- **帖子（Discussions）**：标题 + Markdown 正文、回复、分页、置顶、锁定、隐藏（软删除）、编辑（显示已编辑）。
- **用户（Users）**：注册（可配置开放 / 邀请码制）、登录/登出、头像上传（自动缩放，缺省生成字环头像）、签名档（bio）、修改密码/邮箱、个人主页 `/u/{username}`、用户组（沿用 Flarum 组 ID：1=admin 2=guest 3=member 4=moderator）。
- **通知（Notifications）**：回复、@提及、点赞，铃铛未读数（轮询）+ 通知页。
- **点赞**：帖子点赞（对应 fof/likes 扩展）。
- **搜索**：标题 + 正文检索（LIKE；MySQL 生产可平滑升级 ngram 全文索引）。
- **管理后台 `/admin`**：版块管理（增删改/排序/颜色/权限）、用户管理（禁言/封禁/改组）、邀请码、站点设置。
- **Flarum 数据导入**：`bbs import-flarum` 从原 Flarum 库导入 users/groups/discussions/posts/tags/点赞等，密码哈希原样保留；帖子 `content` JSON 自动解码为 Markdown。
- **链接兼容**：`/d/{id}`、`/d/{id}-{slug}`、`/t/{slug}` 与 Flarum 完全一致，SPA 路由直接接管，SEO 收录与旧书签不断链。

### 第二阶段（后续迭代）
- 未读标记（discussion_user 已读位点，"2 天前"新帖高亮）
- 邮件通知/找回密码（SMTP）
- 附件上传（对应 fof/upload）、代码高亮（chroma）
- MySQL ngram 全文检索、评论历史版本
- 只读 REST API（为未来小程序/客户端预留）

### 明确不做
- Mithril SPA 前端兼容层（服务端渲染替代）
- Flarum 扩展运行时（所需能力直接内置）

## 三、安全设计
1. **XSS**：Markdown → goldmark 渲染 → bluemonday `UGCPolicy` 白名单消毒后才入库/输出；模板自动转义兜底。
2. **CSRF**：gorilla/csrf 全站中间件，所有 POST 表单带 token。
3. **会话**：服务端 token（128bit 随机），HttpOnly + SameSite=Lax（可配 Secure），无前端可读凭据。
4. **密码**：bcrypt cost 12；登录限速（按 IP 令牌桶）。
5. **注入**：GORM 参数化查询，杜绝拼接 SQL。
6. **上传**：头像仅 jpg/png，限 2MB，重采样为 128×128，随机文件名，独立数据目录不执行。
7. **权限**：服务端逐请求鉴权（版块可见性、发帖/回帖、管理操作），前端仅展示不授权。
8. **限速**：登录/注册/发帖按 IP + 用户双维度限速。

## 四、目录结构
```
cmd/bbs/              入口：serve / import-flarum / seed-admin
frontend/             Bun + Vite + React + shadcn/ui SPA 源码
internal/api/         REST JSON 接口（SPA 数据源）
internal/frontend/    go:embed 内嵌 SPA 构建产物
internal/config/      配置（YAML）
internal/models/      GORM 模型（User/Discussion/Post/Tag/Group/Notification/...）
internal/db/          连接、自动迁移、默认数据
internal/auth/        密码、会话、权限
internal/markdown/    goldmark + bluemonday + @提及
internal/slug/        URL slug
internal/web/         会话中间件、静态端点（头像/验证码）、SPA 伺服
internal/importer/    Flarum 数据导入
```

## 五、迁移与上线步骤
1. 部署新服务（配置 MySQL 指向**新库**），`bbs seed-admin` 创建管理员。
2. 维护窗口内执行 `bbs import-flarum --dsn "user:pass@tcp(旧库)/flarum"`，校验讨论/帖子/用户计数。
3. nginx 将 `bbs.jnsec.org` 反代切换到新服务；旧 `/t`、`/d` 路由天然兼容。
4. 观察期保留旧 PHP 目录随时回切（数据不互相写入即无漂移）。

## 六、性能目标
- 首页/版块页：服务端直出 HTML，无 SPA 二次 API 往返；分类卡片最后活跃帖用单查询聚合。
- 单二进制 + 嵌入资源，冷启动 < 100ms；MySQL 连接池复用。
- 列表页全部走索引（时间线 last_posted_at、版块计数冗余列），渲染结果可用内存缓存进一步加速（预留）。
