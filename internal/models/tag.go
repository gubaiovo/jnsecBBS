package models

import "time"

// 字段类型按生产库 information_schema 实测对齐（2026-10 采集）。

// Tag 映射 flarum_tags（含 discussion_count 计数列与 last_posted_* 冗余列，
// PHP 端维护；Go 端读取使用，不负责刷新）。
type Tag struct {
	ID               uint       `gorm:"column:id;primaryKey"`
	Name             string     `gorm:"column:name;type:varchar(100)"`
	Slug             string     `gorm:"column:slug;type:varchar(100);uniqueIndex"`
	Description      string     `gorm:"column:description;type:text"`
	Color            string     `gorm:"column:color;type:varchar(50)"`
	BackgroundPath   string     `gorm:"column:background_path;type:varchar(100)"`
	BackgroundMode   string     `gorm:"column:background_mode;type:varchar(100)"`
	Position         *int       `gorm:"column:position"`
	ParentID         *uint      `gorm:"column:parent_id"`
	DefaultSort      string     `gorm:"column:default_sort;type:varchar(50)"`
	IsRestricted     bool       `gorm:"column:is_restricted"`
	IsHidden         bool       `gorm:"column:is_hidden"`
	DiscussionsCount int        `gorm:"column:discussion_count;type:int unsigned"`
	LastPostedAt     *time.Time `gorm:"column:last_posted_at"`
	LastPostedDiscID *uint      `gorm:"column:last_posted_discussion_id"`
	LastPostedUserID *uint      `gorm:"column:last_posted_user_id"`
	Icon             string     `gorm:"column:icon;type:varchar(100)"`

	ViewGroups  []uint `gorm:"-"` // 可见组（来自 flarum_group_permission，查询层填充）
	PostGroups  []uint `gorm:"-"` // 可发帖组
	ReplyGroups []uint `gorm:"-"`
	Ancestors   []Tag  `gorm:"-"`
}

func (Tag) TableName() string { return "flarum_tags" }

// Notification 存储于 Go 专属表 bbs_notifications（不写 flarum_notifications——
// 其 subject/data 结构为 PHP 序列化格式，跨写有兼容风险）。
type Notification struct {
	ID           uint        `gorm:"column:id;primaryKey"`
	UserID       uint        `gorm:"column:user_id;index:idx_bbs_notif"`
	SenderID     uint        `gorm:"column:sender_id"`
	Sender       User        `gorm:"foreignKey:SenderID"`
	Type         string      `gorm:"column:type;type:varchar(100);index:idx_bbs_notif"` // reply | mention | like
	DiscussionID uint        `gorm:"column:discussion_id"`
	Discussion   *Discussion `gorm:"foreignKey:DiscussionID"`
	PostID       uint        `gorm:"column:post_id"`
	ReadAt       *time.Time  `gorm:"column:read_at"`
	CreatedAt    time.Time   `gorm:"column:created_at;autoCreateTime"`
}

func (Notification) TableName() string { return "bbs_notifications" }

// Setting 映射 flarum_settings（与 PHP 共享；forum_title 即站点名）。
type Setting struct {
	Key   string `gorm:"column:key;primaryKey;type:varchar(100)"`
	Value string `gorm:"column:value;type:text"`
}

func (Setting) TableName() string { return "flarum_settings" }

// GroupPermission 映射 flarum_group_permission（PHP 与 Go 共享版块授权）。
// Flarum/tags 的 permission 形如 tag{ID}.viewDiscussions / tag{ID}.startDiscussion。
type GroupPermission struct {
	GroupID    uint   `gorm:"column:group_id;primaryKey"`
	Permission string `gorm:"column:permission;primaryKey;type:varchar(100)"`
}

func (GroupPermission) TableName() string { return "flarum_group_permission" }

// 设置键：站点名沿用 Flarum 的 forum_title；其余为 Go 专属键（bbs_ 前缀）。
const (
	SettingKeySiteName     = "forum_title"
	SettingKeyWelcomeText  = "bbs_welcome_text"
	SettingKeyRegistration = "bbs_registration"
)
