package models

import "time"

// Tag 映射 flarum_tags。Flarum 无讨论计数列，DiscussionsCount 由查询层实时填充。
type Tag struct {
	ID           uint   `gorm:"column:id;primaryKey"`
	Name         string `gorm:"column:name"`
	Slug         string `gorm:"column:slug"`
	Description  string `gorm:"column:description"`
	Color        string `gorm:"column:color"`
	Position     int    `gorm:"column:position"`
	ParentID     *uint  `gorm:"column:parent_id"`
	IsRestricted bool   `gorm:"column:is_restricted"`

	ViewGroups       []uint `gorm:"-"` // 可见组（来自 flarum_group_permission，查询层填充）
	PostGroups       []uint `gorm:"-"` // 可发帖组
	DiscussionsCount int    `gorm:"-"` // 实时 COUNT，不落库
}

func (Tag) TableName() string { return "flarum_tags" }

// Notification 存储于 Go 专属表 bbs_notifications（不写 flarum_notifications——
// 其 subject/data 结构为 PHP 序列化格式，跨写有兼容风险）。
type Notification struct {
	ID           uint        `gorm:"column:id;primaryKey"`
	UserID       uint        `gorm:"column:user_id;index:idx_bbs_notif"`
	SenderID     uint        `gorm:"column:sender_id"`
	Sender       User        `gorm:"foreignKey:SenderID"`
	Type         string      `gorm:"column:type;index:idx_bbs_notif"` // reply | mention | like
	DiscussionID uint        `gorm:"column:discussion_id"`
	Discussion   *Discussion `gorm:"foreignKey:DiscussionID"`
	PostID       uint        `gorm:"column:post_id"`
	ReadAt       *time.Time  `gorm:"column:read_at"`
	CreatedAt    time.Time   `gorm:"column:created_at;autoCreateTime"`
}

func (Notification) TableName() string { return "bbs_notifications" }

// Setting 映射 flarum_settings（与 PHP 共享；forum_title 即站点名）。
type Setting struct {
	Key   string `gorm:"column:key;primaryKey"`
	Value string `gorm:"column:value"`
}

func (Setting) TableName() string { return "flarum_settings" }

// GroupPermission 映射 flarum_group_permission（PHP 与 Go 共享版块授权）。
// Flarum/tags 的 permission 形如 tag{ID}.viewDiscussions / tag{ID}.startDiscussion。
type GroupPermission struct {
	GroupID    uint   `gorm:"column:group_id;primaryKey"`
	Permission string `gorm:"column:permission;primaryKey"`
}

func (GroupPermission) TableName() string { return "flarum_group_permission" }

// 设置键：站点名沿用 Flarum 的 forum_title；其余为 Go 专属键（bbs_ 前缀）。
const (
	SettingKeySiteName     = "forum_title"
	SettingKeyWelcomeText  = "bbs_welcome_text"
	SettingKeyRegistration = "bbs_registration"
)
