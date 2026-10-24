package models

import "time"

// AccessToken 映射 flarum_access_tokens（与 PHP 共享；Go 写入 session 类型 token，
// 理论上 Flarum 的 API 客户端同样可使用）。
type AccessToken struct {
	ID             uint      `gorm:"column:id;primaryKey"`
	Token          string    `gorm:"column:token"`
	UserID         uint      `gorm:"column:user_id"`
	CreatedAt      time.Time `gorm:"column:created_at;autoCreateTime"`
	LastActivityAt time.Time `gorm:"column:last_activity_at;autoCreateTime"`
	Type           string    `gorm:"column:type"`
}

func (AccessToken) TableName() string { return "flarum_access_tokens" }

// InviteCode 存储于 Go 专属表 bbs_invite_codes（替代 fof/doorman 的 doorkeys）。
type InviteCode struct {
	Code      string     `gorm:"column:code;primaryKey"`
	CreatedBy uint       `gorm:"column:created_by"`
	UsedBy    *uint      `gorm:"column:used_by"`
	UsedAt    *time.Time `gorm:"column:used_at"`
	ExpiresAt *time.Time `gorm:"column:expires_at"`
	CreatedAt time.Time  `gorm:"column:created_at;autoCreateTime"`
}

func (InviteCode) TableName() string { return "bbs_invite_codes" }
