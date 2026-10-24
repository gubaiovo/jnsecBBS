package auth

import (
	"regexp"
	"strconv"

	"github.com/exzco/bbs/internal/models"
	"gorm.io/gorm"
)

// 版块权限：PHP 与 Go 共享 flarum_group_permission 表。
// Flarum/tags 的 permission 格式为 tag{ID}.viewDiscussions / tag{ID}.startDiscussion
// / tag{ID}.replyToDiscussions，每行一个 (group_id, permission)。
// 语义与旧实现一致：无任何授权行 = 公开（所有人可见/任何登录者可发帖）。

var tagPermRe = regexp.MustCompile(`^tag(\d+)\.(viewDiscussions|startDiscussion|replyToDiscussions)$`)

type TagPerms struct {
	View map[uint][]uint // tagID -> 允许查看的组 ID 列表
	Post map[uint][]uint // tagID -> 允许发帖/回帖的组 ID 列表
}

// LoadTagPerms 一次性加载全部版块权限（每请求调用一次，数据量小）。
func LoadTagPerms(g *gorm.DB) TagPerms {
	tp := TagPerms{View: map[uint][]uint{}, Post: map[uint][]uint{}}
	var rows []models.GroupPermission
	if err := g.Find(&rows).Error; err != nil {
		return tp
	}
	for _, r := range rows {
		m := tagPermRe.FindStringSubmatch(r.Permission)
		if m == nil {
			continue // 非版块权限（其他扩展的全局权限），跳过
		}
		tagID, _ := strconv.ParseUint(m[1], 10, 32)
		switch m[2] {
		case "viewDiscussions":
			tp.View[uint(tagID)] = append(tp.View[uint(tagID)], r.GroupID)
		case "startDiscussion", "replyToDiscussions":
			tp.Post[uint(tagID)] = append(tp.Post[uint(tagID)], r.GroupID)
		}
	}
	return tp
}

// FillTagPerm 把权限填充到单个版块。
func FillTagPerm(g *gorm.DB, tag *models.Tag, tp TagPerms) {
	tag.ViewGroups = tp.View[tag.ID]
	tag.PostGroups = tp.Post[tag.ID]
}

// FillTagPerms 批量填充。
func FillTagPerms(g *gorm.DB, tags []models.Tag, tp TagPerms) {
	for i := range tags {
		tags[i].ViewGroups = tp.View[tags[i].ID]
		tags[i].PostGroups = tp.Post[tags[i].ID]
	}
}

// CanViewTag 版块可见性：无授权行 = 任何人（含游客）可见。
func CanViewTag(u *models.User, tag *models.Tag) bool {
	if len(tag.ViewGroups) == 0 {
		return true
	}
	if u == nil {
		return false
	}
	return u.HasGroup(tag.ViewGroups...)
}

// CanPostTag 发帖/回帖权限：无授权行 = 任何登录用户。
func CanPostTag(u *models.User, tag *models.Tag) bool {
	if u == nil || u.IsBanned() {
		return false
	}
	if len(tag.PostGroups) == 0 {
		return true
	}
	return u.HasGroup(tag.PostGroups...)
}

// SaveTagPerms 把管理后台设置的可见/发帖组写入 flarum_group_permission
// （先清除该版块的相关行，再插入；与 PHP 端格式一致）。
func SaveTagPerms(g *gorm.DB, tagID uint, viewGroups, postGroups []uint) error {
	verbs := []string{
		"tag" + strconv.FormatUint(uint64(tagID), 10) + ".viewDiscussions",
		"tag" + strconv.FormatUint(uint64(tagID), 10) + ".startDiscussion",
		"tag" + strconv.FormatUint(uint64(tagID), 10) + ".replyToDiscussions",
	}
	if err := g.Where("permission IN ?", verbs).Delete(&models.GroupPermission{}).Error; err != nil {
		return err
	}
	rows := make([]models.GroupPermission, 0, len(viewGroups)+len(postGroups)*2)
	for _, gid := range viewGroups {
		if gid == models.GroupGuest {
			continue // 游客为默认公开语义，不落行
		}
		rows = append(rows, models.GroupPermission{GroupID: gid, Permission: verbs[0]})
	}
	for _, gid := range postGroups {
		if gid == models.GroupGuest {
			continue
		}
		rows = append(rows, models.GroupPermission{GroupID: gid, Permission: verbs[1]})
		rows = append(rows, models.GroupPermission{GroupID: gid, Permission: verbs[2]})
	}
	if len(rows) > 0 {
		return g.Create(&rows).Error
	}
	return nil
}

// FilterVisibleTags 返回用户可见的版块。
func FilterVisibleTags(u *models.User, tags []models.Tag) []models.Tag {
	out := tags[:0:0]
	for i := range tags {
		if CanViewTag(u, &tags[i]) {
			out = append(out, tags[i])
		}
	}
	return out
}

// CanEditPost 本人、版主可编辑。
func CanEditPost(u *models.User, p *models.Post) bool {
	if u == nil {
		return false
	}
	return u.IsModerator() || p.UserID == u.ID
}
