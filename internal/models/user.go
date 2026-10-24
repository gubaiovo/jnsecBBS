package models

import "time"

// 字段类型按生产库 information_schema 实测对齐（2026-10 采集）。

// 用户组 ID 沿用 Flarum 约定。
const (
	GroupAdmin  uint = 1
	GroupGuest  uint = 2
	GroupMember uint = 3
	GroupMod    uint = 4
)

type User struct {
	ID                  uint       `gorm:"column:id;primaryKey"`
	Username            string     `gorm:"column:username;type:varchar(100);uniqueIndex"`
	Email               string     `gorm:"column:email;type:varchar(150);uniqueIndex"`
	IsEmailConfirmed    bool       `gorm:"column:is_email_confirmed"`
	Password            string     `gorm:"column:password;type:varchar(100)"`
	Bio                 string     `gorm:"column:bio;type:text"`
	AvatarURL           string     `gorm:"column:avatar_url;type:varchar(100)"`
	Preferences         []byte     `gorm:"column:preferences;type:blob"` // PHP 序列化格式，Go 只透传不解析
	JoinedAt            *time.Time `gorm:"column:joined_at"`
	LastSeenAt          *time.Time `gorm:"column:last_seen_at"`
	MarkedAllAsReadAt   *time.Time `gorm:"column:marked_all_as_read_at"`
	ReadNotificationsAt *time.Time `gorm:"column:read_notifications_at"`
	DiscussionsCount    int        `gorm:"column:discussion_count;type:int unsigned"`
	PostsCount          int        `gorm:"column:comment_count;type:int unsigned"`
	ReadFlagsAt         *time.Time `gorm:"column:read_flags_at"`
	SuspendUntil        *time.Time `gorm:"column:suspended_until"`               // 注意：flarum/suspend 的真实列名
	InviteCode          string     `gorm:"column:invite_code;type:varchar(128)"` // fof/doorman 遗留
	Groups              []Group    `gorm:"many2many:flarum_group_user;foreignKey:ID;joinForeignKey:UserID;joinReferences:ID"`
}

func (User) TableName() string { return "flarum_users" }

// IsBanned：flarum/suspend 语义——suspended_until 在未来即封禁中。
func (u *User) IsBanned() bool {
	return u.SuspendUntil != nil && u.SuspendUntil.After(time.Now())
}

func (u *User) HasGroup(ids ...uint) bool {
	for _, g := range u.Groups {
		for _, want := range ids {
			if g.ID == want {
				return true
			}
		}
	}
	return false
}

func (u *User) IsAdmin() bool       { return u.HasGroup(GroupAdmin) }
func (u *User) IsModerator() bool   { return u.HasGroup(GroupAdmin, GroupMod) }
func (u *User) DisplayName() string { return u.Username }

// JoinedAtOrZero 供 DTO 使用：可空时间零值兜底。
func (u *User) JoinedAtOrZero() time.Time {
	if u.JoinedAt != nil {
		return *u.JoinedAt
	}
	return time.Time{}
}

// GroupIDs 返回用户所属组 ID（管理后台表单勾选用）。
func (u *User) GroupIDs() []uint {
	ids := make([]uint, 0, len(u.Groups))
	for _, g := range u.Groups {
		ids = append(ids, g.ID)
	}
	return ids
}

type Group struct {
	ID           uint   `gorm:"column:id;primaryKey"`
	NameSingular string `gorm:"column:name_singular;type:varchar(100)"`
	NamePlural   string `gorm:"column:name_plural;type:varchar(100)"`
	Color        string `gorm:"column:color;type:varchar(20)"`
	Icon         string `gorm:"column:icon;type:varchar(100)"`
	IsHidden     bool   `gorm:"column:is_hidden"`
}

// Name 展示名（Flarum 后台以 name_plural 为组名）。
func (g *Group) Name() string { return g.NamePlural }

func (Group) TableName() string { return "flarum_groups" }
