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

// Migrate 确保表结构：
//   - flarum_* 表：生产环境由 PHP 侧管理（已存在，AutoMigrate 幂等无操作）；
//     本地开发（SQLite）首次启动时按模型自动建表。
//   - bbs_* 表：Go 专属辅助表，自动创建。
//
// AutoMigrate 的模型列集均为 Flarum 标准列（或 flarum/suspend 扩展列），
// 不会改动 PHP 端的既有结构。
func Migrate(g *gorm.DB, seed bool) error {
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

// SeedDefaults 首次启动（空库）时预置用户组与默认版块结构。
func SeedDefaults(g *gorm.DB) error {
	var n int64
	g.Model(&models.Group{}).Count(&n)
	if n == 0 {
		groups := []models.Group{
			{ID: models.GroupAdmin, Name: "Admin", NameSingular: "Admin", Color: "#e53935", Icon: "fas fa-wrench"},
			{ID: models.GroupGuest, Name: "Guest", NameSingular: "Guest", Color: "#9e9e9e", Icon: "fas fa-reply", IsHidden: true},
			{ID: models.GroupMember, Name: "正式成员", NameSingular: "成员", Color: "#1e88e5", Icon: "fas fa-user"},
			{ID: models.GroupMod, Name: "版主", NameSingular: "版主", Color: "#43a047", Icon: "fas fa-bolt"},
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
	tags := []models.Tag{
		{Name: "网络安全", Slug: "security", Description: "方向各异的安全技术", Color: "#1e88e5", Position: 1},
		{Name: "比赛相关", Slug: "CTF", Description: "报名通知 / WriteUp", Color: "#9e9d24", Position: 2},
		{Name: "技术专精", Slug: "tech", Description: "编程语言/机器学习/运维/框架/玄学/…", Color: "#2e7d32", Position: 3},
		{Name: "工作&读研", Slug: "career", Description: "公司高校评价/内推&社招信息/面试经验", Color: "#7b1fa2", Position: 4},
		{Name: "建设&发展", Slug: "dev", Description: "传承", Color: "#c62828", Position: 5},
		{Name: "悠哉日常", Slug: "daily", Description: "未来展望/哲学思考/新闻/团队娱乐/…", Color: "#ef6c00", Position: 6},
		{Name: "通知公告", Slug: "announcements", Description: "活动通知/事件公告/运维变动", Color: "#ff7043", Position: 7},
		{Name: "平台运维", Slug: "ops", Description: "平台运维记录", Color: "#26c6da", Position: 8},
		{Name: "周报", Slug: "weekly", Description: "勤能补拙", Color: "#5c6bc0", Position: 9},
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
