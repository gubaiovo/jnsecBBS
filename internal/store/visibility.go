// Package store contains shared database queries used by the forum API.
package store

import (
	"github.com/exzco/bbs/internal/auth"
	"github.com/exzco/bbs/internal/models"
	"gorm.io/gorm"
)

// VisibleDiscussions applies visibility before pagination and aggregation, so
// multi-tag discussions cannot leak via a public tag, search, or user profile.
func VisibleDiscussions(g *gorm.DB, u *models.User) *gorm.DB {
	query := g.Model(&models.Discussion{}).Where("flarum_discussions.is_private = ?", false)
	var tags []models.Tag
	if err := g.Find(&tags).Error; err != nil {
		return query.Where("1 = 0")
	}
	tp := auth.LoadTagPerms(g)
	auth.FillTagPerms(g, tags, tp)
	if tp.Failed {
		return query.Where("1 = 0")
	}
	denied := []uint{}
	for i := range tags {
		if !auth.CanViewTag(u, &tags[i]) {
			denied = append(denied, tags[i].ID)
		}
	}
	if len(denied) > 0 {
		query = query.Where("NOT EXISTS (SELECT 1 FROM flarum_discussion_tag access_tag WHERE access_tag.discussion_id = flarum_discussions.id AND access_tag.tag_id IN ?)", denied)
	}
	return query
}

func VisiblePosts(g *gorm.DB, u *models.User) *gorm.DB {
	ids := VisibleDiscussions(g, u).Select("flarum_discussions.id").Where("flarum_discussions.hidden_at IS NULL AND flarum_discussions.is_approved = ?", true)
	return g.Model(&models.Post{}).Where("discussion_id IN (?) AND is_private = ?", ids, false)
}
