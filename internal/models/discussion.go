package models

import (
	"encoding/json"
	"time"

	"github.com/exzco/bbs/internal/slug"
)

// 字段类型按生产库 information_schema 实测对齐（2026-10 采集），
// 见 user.go 注释——保证与 PHP 端共享表时的读写安全。

type Discussion struct {
	ID               uint       `gorm:"column:id;primaryKey"`
	Title            string     `gorm:"column:title;type:varchar(200)"`
	CommentCount     int        `gorm:"column:comment_count"`
	ParticipantCount int        `gorm:"column:participant_count;type:int unsigned"`
	PostNumberIndex  int        `gorm:"column:post_number_index;type:int unsigned"`
	CreatedAt        time.Time  `gorm:"column:created_at"`
	UserID           *uint      `gorm:"column:user_id"`
	User             User       `gorm:"foreignKey:UserID"`
	FirstPostID      *uint      `gorm:"column:first_post_id"`
	LastPostedAt     *time.Time `gorm:"column:last_posted_at"`
	LastPostID       *uint      `gorm:"column:last_post_id"`
	LastPostNumber   *int       `gorm:"column:last_post_number;type:int unsigned"`
	LastPostUserID   *uint      `gorm:"column:last_posted_user_id"`
	LastPostUser     *User      `gorm:"foreignKey:LastPostUserID"`
	HiddenAt         *time.Time `gorm:"column:hidden_at"`
	HiddenUserID     *uint      `gorm:"column:hidden_user_id"`
	SlugCol          string     `gorm:"column:slug;type:varchar(255)"`
	IsPrivate        bool       `gorm:"column:is_private"`
	IsApproved       bool       `gorm:"column:is_approved"`
	IsLocked         bool       `gorm:"column:is_locked"`
	IsSticky         bool       `gorm:"column:is_sticky"`
	ViewCount        int        `gorm:"column:view_count"`
	Tags             []Tag      `gorm:"many2many:flarum_discussion_tag;foreignKey:ID;joinForeignKey:DiscussionID;joinReferences:ID"`
}

func (Discussion) TableName() string { return "flarum_discussions" }

// Slug 优先用库内 slug 列（PHP 端生成的历史值，保持旧链接完全一致）；
// 空则按标题运行时生成。
func (d *Discussion) Slug() string {
	if d.SlugCol != "" {
		return d.SlugCol
	}
	return slug.Make(d.Title)
}

func (d *Discussion) IsHidden() bool { return d.HiddenAt != nil }

// LastPostedAtOrZero 供 DTO 使用：可空时间零值兜底。
func (d *Discussion) LastPostedAtOrZero() time.Time {
	if d.LastPostedAt != nil {
		return *d.LastPostedAt
	}
	return d.CreatedAt
}

type Post struct {
	ID           uint        `gorm:"column:id;primaryKey"`
	DiscussionID uint        `gorm:"column:discussion_id"`
	Number       *int        `gorm:"column:number;type:int unsigned"`
	CreatedAt    time.Time   `gorm:"column:created_at"`
	UserID       *uint       `gorm:"column:user_id"`
	User         User        `gorm:"foreignKey:UserID"`
	Type         string      `gorm:"column:type;type:varchar(100)"`  // 仅 'comment' 参与内容展示
	Content      string      `gorm:"column:content;type:mediumtext"` // Flarum JSON：{"text": "..."}
	EditedAt     *time.Time  `gorm:"column:edited_at"`
	EditedUserID *uint       `gorm:"column:edited_user_id"`
	HiddenAt     *time.Time  `gorm:"column:hidden_at"`
	HiddenUserID *uint       `gorm:"column:hidden_user_id"`
	IPAddress    string      `gorm:"column:ip_address;type:varchar(45)"`
	IsPrivate    bool        `gorm:"column:is_private"`
	IsApproved   bool        `gorm:"column:is_approved"`
	LikeCount    int         `gorm:"-"` // 实时 COUNT（flarum_post_likes），不落库
	Discussion   *Discussion `gorm:"foreignKey:DiscussionID"`
}

func (Post) TableName() string { return "flarum_posts" }

// Markdown 解出正文（type=comment 的 content JSON 中取 text）。
func (p *Post) Markdown() string {
	if p.Type != "comment" {
		return ""
	}
	var m struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(p.Content), &m); err == nil {
		return m.Text
	}
	return p.Content // 兼容裸文本
}

// SetMarkdown 把 Markdown 包装为 Flarum content JSON。
func (p *Post) SetMarkdown(md string) {
	b, _ := json.Marshal(map[string]string{"text": md})
	p.Content = string(b)
}

// PostNumber 返回楼层号（0 表示无楼层，如 discussionRenamed 等系统帖）。
func (p *Post) PostNumber() int {
	if p.Number == nil {
		return 0
	}
	return *p.Number
}

type PostLike struct {
	PostID    uint      `gorm:"column:post_id;primaryKey"`
	UserID    uint      `gorm:"column:user_id;primaryKey"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (PostLike) TableName() string { return "flarum_post_likes" }
