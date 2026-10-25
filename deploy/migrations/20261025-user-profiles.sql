-- 实名与博客链接的 Go 专属扩展表。
-- 只读镜像阶段应用启动不执行 DDL，需在 bbs 库手工执行一次；
-- 读取方在缺表时降级为空，未执行不影响现有功能。
-- 五分钟同步只覆盖 flarum_* 表，本表数据不会被旧库快照冲掉。
CREATE TABLE IF NOT EXISTS bbs_user_profiles (
  user_id int unsigned NOT NULL,
  real_name varchar(100) NOT NULL DEFAULT '',
  blog_url varchar(255) NOT NULL DEFAULT '',
  updated_at datetime NOT NULL,
  PRIMARY KEY (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
