package models

import "time"

// UserProfile 是 Go 专属的用户扩展资料（实名、博客链接）。
// 独立 bbs_user_profiles 表：五分钟同步只覆盖 flarum_* 表，这里的数据不会被
// 旧库快照冲掉。只读镜像阶段该表可能尚未创建，读取方必须容忍缺表并降级为空。
type UserProfile struct {
	UserID    uint      `gorm:"column:user_id;primaryKey"`
	RealName  string    `gorm:"column:real_name;type:varchar(100)"`
	BlogURL   string    `gorm:"column:blog_url;type:varchar(255)"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (UserProfile) TableName() string { return "bbs_user_profiles" }
