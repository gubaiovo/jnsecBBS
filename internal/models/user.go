package models

import "time"

// 全部模型直接映射 Flarum 1.x 原表（表名 flarum_*，由 PHP 侧负责建表/迁移）。
// Go 端与 PHP 端共享同一套表与数据，实时一致；写入路径须维护 Flarum 的计数语义。
// bbs_* 前缀的表为 Go 专属辅助表（Flarum 不感知）。

// 用户组 ID 沿用 Flarum 约定。
const (
	GroupAdmin  uint = 1
	GroupGuest  uint = 2
	GroupMember uint = 3
	GroupMod    uint = 4
)

type User struct {
	ID                  uint       `gorm:"column:id;primaryKey"`
	Username            string     `gorm:"column:username"`
	Email               string     `gorm:"column:email"`
	Password            string     `gorm:"column:password"`
	Bio                 string     `gorm:"column:bio"`
	AvatarURL           string     `gorm:"column:avatar_url"`
	Preferences         string     `gorm:"column:preferences"`
	IsEmailConfirmed    bool       `gorm:"column:is_email_confirmed"`
	DiscussionsCount    int        `gorm:"column:discussions_count"`
	PostsCount          int        `gorm:"column:comment_count"`
	JoinedAt            time.Time  `gorm:"column:joined_at;autoCreateTime"`
	LastSeenAt          *time.Time `gorm:"column:last_seen_at"`
	ReadNotificationsAt *time.Time `gorm:"column:read_notifications_at"`
	SuspendUntil        *time.Time `gorm:"column:suspend_until"`
	SuspendMessage      string     `gorm:"column:suspend_message"`
	Groups              []Group    `gorm:"many2many:flarum_group_user;foreignKey:ID;joinForeignKey:UserID;joinReferences:ID"`
}

func (User) TableName() string { return "flarum_users" }

// IsBanned：flarum/suspend 语义——suspend_until 在未来即封禁中。
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
	Name         string `gorm:"column:name"`
	NameSingular string `gorm:"column:name_singular"`
	Color        string `gorm:"column:color"`
	Icon         string `gorm:"column:icon"`
	IsHidden     bool   `gorm:"column:is_hidden"`
}

func (Group) TableName() string { return "flarum_groups" }
