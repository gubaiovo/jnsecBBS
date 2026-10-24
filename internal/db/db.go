package db

import (
	"fmt"

	"github.com/exzco/bbs/internal/models"
	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Open 按配置打开数据库（生产 MySQL 指向 flarum 库；本地 SQLite 自动建 flarum 结构）。
func Open(driver, dsn string) (*gorm.DB, error) {
	var dialector gorm.Dialector
	switch driver {
	case "mysql":
		dialector = mysql.Open(dsn)
	default: // sqlite
		dialector = sqlite.Open(dsn)
	}
	return gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
}

// Migrate 确保表结构，按驱动分流：
//   - MySQL（生产）：flarum_* 表由 PHP 侧管理，**绝不 AutoMigrate**（防止对线上表
//     产生任何 ALTER——类型/索引差异都可能改动或破坏 PHP 的结构）；仅确保两张
//     Go 专属辅助表 bbs_* 存在（CREATE TABLE IF NOT EXISTS 语义）。
//   - SQLite（本地开发）：全部表按模型自动创建（首次启动即为 flarum 结构）。
func Migrate(g *gorm.DB, seed bool) error {
	if g.Dialector.Name() == "mysql" {
		if err := g.AutoMigrate(&models.Notification{}, &models.InviteCode{}); err != nil {
			return fmt.Errorf("migrate bbs tables: %w", err)
		}
		if err := ensureMySQLCompatibility(g); err != nil {
			return err
		}
		if seed {
			return SeedDefaults(g)
		}
		return nil
	}
	if err := g.AutoMigrate(
		&models.User{}, &models.Group{},
		&models.Discussion{}, &models.Post{}, &models.PostLike{},
		&models.Tag{}, &models.Notification{},
		&models.AccessToken{}, &models.InviteCode{}, &models.Setting{},
	); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	if seed {
		return SeedDefaults(g)
	}
	return nil
}

// ensureMySQLCompatibility 生产启动自检：确认 Go 依赖的 flarum_* 表全部存在。
// 缺表（如 DSN 指到空库、或扩展未安装）时给出可读的报错，而不是在请求半路崩。
func ensureMySQLCompatibility(g *gorm.DB) error {
	required := []string{
		"flarum_users", "flarum_groups", "flarum_group_user",
		"flarum_discussions", "flarum_posts", "flarum_tags",
		"flarum_discussion_tag", "flarum_post_likes",
		"flarum_access_tokens", "flarum_settings", "flarum_group_permission",
	}
	var missing []string
	for _, t := range required {
		if err := g.Exec("SELECT 1 FROM `" + t + "` LIMIT 1").Error; err != nil {
			missing = append(missing, t)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("数据库缺少 Flarum 表（DSN 是否指向了 flarum 库？）: %v", missing)
	}
	return nil
}

// SeedDefaults 首次启动（空库，仅本地 SQLite 路径）时预置用户组与默认版块结构。
func SeedDefaults(g *gorm.DB) error {
	var n int64
	g.Model(&models.Group{}).Count(&n)
	if n == 0 {
		groups := []models.Group{
			{ID: models.GroupAdmin, NameSingular: "Admin", NamePlural: "Admin", Color: "#e53935", Icon: "fas fa-wrench"},
			{ID: models.GroupGuest, NameSingular: "Guest", NamePlural: "Guest", Color: "#9e9e9e", Icon: "fas fa-reply", IsHidden: true},
			{ID: models.GroupMember, NameSingular: "成员", NamePlural: "正式成员", Color: "#1e88e5", Icon: "fas fa-user"},
			{ID: models.GroupMod, NameSingular: "版主", NamePlural: "版主", Color: "#43a047", Icon: "fas fa-bolt"},
		}
		if err := g.Create(&groups).Error; err != nil {
			return err
		}
	}

	g.Where("`key` = ?", models.SettingKeySiteName).FirstOrInit(&models.Setting{Key: models.SettingKeySiteName, Value: "JNSEC BBS"})

	return SeedDefaultTags(g)
}

// SeedDefaultTags 空库时按原站结构预置版块（可在管理后台修改）。
func SeedDefaultTags(g *gorm.DB) error {
	var n int64
	g.Model(&models.Tag{}).Count(&n)
	if n > 0 {
		return nil
	}
	pos := func(i int) *int { return &i }
	tags := []models.Tag{
		{Name: "网络安全", Slug: "security", Description: "方向各异的安全技术", Color: "#1e88e5", Position: pos(1)},
		{Name: "比赛相关", Slug: "CTF", Description: "报名通知 / WriteUp", Color: "#9e9d24", Position: pos(2)},
		{Name: "技术专精", Slug: "tech", Description: "编程语言/机器学习/运维/框架/玄学/…", Color: "#2e7d32", Position: pos(3)},
		{Name: "工作&读研", Slug: "career", Description: "公司高校评价/内推&社招信息/面试经验", Color: "#7b1fa2", Position: pos(4)},
		{Name: "建设&发展", Slug: "dev", Description: "传承", Color: "#c62828", Position: pos(5)},
		{Name: "悠哉日常", Slug: "daily", Description: "未来展望/哲学思考/新闻/团队娱乐/…", Color: "#ef6c00", Position: pos(6)},
		{Name: "通知公告", Slug: "announcements", Description: "活动通知/事件公告/运维变动", Color: "#ff7043", Position: pos(7)},
		{Name: "平台运维", Slug: "ops", Description: "平台运维记录", Color: "#26c6da", Position: pos(8)},
		{Name: "周报", Slug: "weekly", Description: "勤能补拙", Color: "#5c6bc0", Position: pos(9)},
	}
	if err := g.Create(&tags).Error; err != nil {
		return err
	}
	bySlug := map[string]uint{}
	for _, t := range tags {
		bySlug[t.Slug] = t.ID
	}
	child := func(parent string, name, slugName string) models.Tag {
		pid := bySlug[parent]
		return models.Tag{Name: name, Slug: slugName, ParentID: &pid}
	}
	children := []models.Tag{
		child("security", "Web", "web"),
		child("security", "硬件与物联网", "hardware"),
		child("security", "网络协议", "protocol"),
		child("security", "逆向", "reverse"),
		child("security", "Pwn", "pwn"),
		child("security", "移动安全", "mobile"),
		child("security", "密码学", "crypto"),
		child("security", "Misc", "misc"),
		child("security", "AWD专区", "awd"),
		child("CTF", "报名通知", "notice"),
		child("CTF", "WriteUp", "writeup"),
		child("tech", "环境配置", "env"),
		child("tech", "通用技术", "general"),
		child("career", "公司感受", "company"),
		child("career", "内推专栏", "referral"),
		child("career", "读研专栏", "grad"),
	}
	return g.Create(&children).Error
}
