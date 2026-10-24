package models

import (
	"encoding/json"
	"time"

	"github.com/exzco/bbs/internal/slug"
)

type Discussion struct {
	ID               uint       `gorm:"column:id;primaryKey"`
	Title            string     `gorm:"column:title"`
	CommentCount     int        `gorm:"column:comment_count"`
	ReplyCount       int        `gorm:"column:reply_count"`
	ParticipantCount int        `gorm:"column:participant_count"`
	CreatedAt        time.Time  `gorm:"column:created_at"`
	UserID           uint       `gorm:"column:user_id"`
	User             User       `gorm:"foreignKey:UserID"`
	FirstPostID      *uint      `gorm:"column:first_post_id"`
	LastPostedAt     time.Time  `gorm:"column:last_posted_at"`
	LastPostID       *uint      `gorm:"column:last_post_id"`
	LastPostNumber   int        `gorm:"column:last_post_number"`
	LastPostUserID   *uint      `gorm:"column:last_posted_user_id"`
	LastPostUser     *User      `gorm:"foreignKey:LastPostUserID"`
	HiddenAt         *time.Time `gorm:"column:hidden_at"`
	IsSticky         bool       `gorm:"column:is_sticky"`
	IsLocked         bool       `gorm:"column:is_locked"`
	IsApproved       bool       `gorm:"column:is_approved"`
	IsPrivate        bool       `gorm:"column:is_private"`
	Tags             []Tag      `gorm:"many2many:flarum_discussion_tag;foreignKey:ID;joinForeignKey:DiscussionID;joinReferences:ID"`
}

func (Discussion) TableName() string { return "flarum_discussions" }

// Slug 运行时生成（Flarum 表无 slug 列；/d/{id}-{slug} 中 id 才是定位键，
// PHP 端接受的任意 slug 均可被新站按 id 解析）。
func (d *Discussion) Slug() string { return slug.Make(d.Title) }

func (d *Discussion) IsHidden() bool { return d.HiddenAt != nil }

type Post struct {
	ID           uint        `gorm:"column:id;primaryKey"`
	DiscussionID uint        `gorm:"column:discussion_id"`
	Number       int         `gorm:"column:number"`
	CreatedAt    time.Time   `gorm:"column:created_at"`
	UserID       uint        `gorm:"column:user_id"`
	User         User        `gorm:"foreignKey:UserID"`
	Type         string      `gorm:"column:type"`    // 仅 'comment' 参与内容展示
	Content      string      `gorm:"column:content"` // Flarum JSON：{"text": "..."}
	EditedAt     *time.Time  `gorm:"column:edited_at"`
	HiddenAt     *time.Time  `gorm:"column:hidden_at"`
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

type PostLike struct {
	PostID uint `gorm:"column:post_id;primaryKey"`
	UserID uint `gorm:"column:user_id;primaryKey"`
}

func (PostLike) TableName() string { return "flarum_post_likes" }
