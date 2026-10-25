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
	case "sqlite":
		dialector = sqlite.Open(dsn)
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", driver)
	}
	return gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
}

// Migrate 确保表结构，按驱动分流：
//   - MySQL（生产）：flarum_* 表由 PHP 侧管理，**绝不使用 AutoMigrate**——
//     GORM 会递归跟随模型关联去 ALTER 被关联的 flarum 表（如把 user_id 改成
//     bigint unsigned，被外键约束拒绝）。两张 Go 专属辅助表用裸 DDL
//     CREATE TABLE IF NOT EXISTS 创建，零递归、零风险。
//   - SQLite（本地开发）：全部表按模型自动创建（首次启动即为 flarum 结构）。
func Migrate(g *gorm.DB, seed bool) error {
	if g.Dialector.Name() == "mysql" {
		if err := createBBSTablesMySQL(g); err != nil {
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
	if err := migrateModels(g); err != nil {
		return err
	}
	if seed {
		return SeedDefaults(g)
	}
	return nil
}

// createBBSTablesMySQL 用裸 DDL 创建 Go 专属辅助表（幂等）。
// 不走 AutoMigrate：其会递归跟随模型关联（Notification→Discussion/Sender）
// 去 ALTER flarum 表，已被外键约束拒绝过一次（Error 1832）。
func createBBSTablesMySQL(g *gorm.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS bbs_notifications (
			id int unsigned NOT NULL AUTO_INCREMENT,
			user_id int unsigned NOT NULL,
			sender_id int unsigned NOT NULL,
			type varchar(100) NOT NULL,
			discussion_id int unsigned NOT NULL,
			post_id int unsigned NOT NULL,
			read_at datetime DEFAULT NULL,
			created_at datetime NOT NULL,
			PRIMARY KEY (id),
			KEY idx_bbs_notif (user_id, type)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE IF NOT EXISTS bbs_invite_codes (
			code varchar(20) NOT NULL,
			created_by int unsigned NOT NULL,
			used_by int unsigned DEFAULT NULL,
			used_at datetime DEFAULT NULL,
			expires_at datetime DEFAULT NULL,
			created_at datetime NOT NULL,
			PRIMARY KEY (code)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
	}
	for _, s := range stmts {
		if err := g.Exec(s).Error; err != nil {
			return err
		}
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

	return SeedDefaultTags(g)
}

// SeedDefaultTags gives new installations a neutral board. Existing forum
// taxonomy and site title remain owned by the administrator.
func SeedDefaultTags(g *gorm.DB) error {
	var n int64
	if err := g.Model(&models.Tag{}).Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	position := 1
	return g.Create(&models.Tag{Name: "综合讨论", Slug: "general", Color: "#1e88e5", Position: &position}).Error
}

// InitializeEmpty explicitly initializes a standalone database. Never run
// AutoMigrate against an existing Flarum schema (including partial installs).
func InitializeEmpty(g *gorm.DB) error {
	tables, err := g.Migrator().GetTables()
	if err != nil {
		return err
	}
	if len(tables) != 0 {
		return fmt.Errorf("init-db requires an empty database; found %d tables", len(tables))
	}
	if err := migrateModels(g); err != nil {
		return err
	}
	return SeedDefaults(g)
}

func migrateModels(g *gorm.DB) error {
	if err := g.AutoMigrate(
		&models.User{}, &models.Group{}, &models.Discussion{}, &models.Post{},
		&models.PostLike{}, &models.Tag{}, &models.Notification{}, &models.AccessToken{},
		&models.InviteCode{}, &models.Setting{}, &models.GroupPermission{},
	); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}
