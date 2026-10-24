package api

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/exzco/bbs/internal/auth"
	"github.com/exzco/bbs/internal/markdown"
	"github.com/exzco/bbs/internal/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// home 首页：大版块卡片（版块 + 子版块 + 最后活跃话题）。
func (s *Server) home(c *gin.Context) {
	u := s.currentUser(c)
	var tags []models.Tag
	s.db.Order("position ASC, id ASC").Find(&tags)
	s.fillTagPerms(tags)
	tags = auth.FilterVisibleTags(u, tags)
	s.fillTagCounts(tags)

	byParent := map[uint][]models.Tag{}
	var tops []models.Tag
	for _, t := range tags {
		if t.ParentID == nil {
			tops = append(tops, t)
		} else {
			byParent[*t.ParentID] = append(byParent[*t.ParentID], t)
		}
	}
	type latestInfo struct {
		Title string    `json:"title"`
		URL   string    `json:"url"`
		At    time.Time `json:"at"`
	}
	type card struct {
		Tag      TagDTO      `json:"tag"`
		Children []TagDTO    `json:"children"`
		Latest   *latestInfo `json:"latest"`
	}
	cards := make([]card, 0, len(tops))
	for _, top := range tops {
		cd := card{Tag: s.tagDTO(&top, u), Children: []TagDTO{}}
		ids := []uint{top.ID}
		for _, ch := range byParent[top.ID] {
			cd.Children = append(cd.Children, s.tagDTO(&ch, u))
			ids = append(ids, ch.ID)
		}
		var d models.Discussion
		err := s.db.Joins("JOIN flarum_discussion_tag dt ON dt.discussion_id = flarum_discussions.id").
			Where("dt.tag_id IN ? AND flarum_discussions.hidden_at IS NULL AND flarum_discussions.is_approved = 1", ids).
			Order("flarum_discussions.last_posted_at DESC").
			Preload("User").Preload("Tags").
			First(&d).Error
		if err == nil {
			cd.Latest = &latestInfo{
				Title: d.Title, URL: discussionURL(&d), At: d.LastPostedAtOrZero(),
			}
		}
		cards = append(cards, cd)
	}
	c.JSON(http.StatusOK, gin.H{"cards": cards})
}

func discussionURL(d *models.Discussion) string {
	if s := d.Slug(); s == "" {
		return http_disc(d.ID)
	}
	return "/d/" + strconvUint(d.ID) + "-" + d.Slug()
}

func http_disc(id uint) string { return "/d/" + strconvUint(id) }

func strconvUint(id uint) string {
	if id == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for id > 0 {
		i--
		b[i] = byte('0' + id%10)
		id /= 10
	}
	return string(b[i:])
}

// fillTagCounts 实时统计各版块的讨论数（Flarum tags 无计数列）。
func (s *Server) fillTagCounts(tags []models.Tag) {
	type row struct {
		TagID uint
		N     int
	}
	var rows []row
	s.db.Table("flarum_discussion_tag").
		Select("tag_id, COUNT(DISTINCT discussion_id) as n").
		Group("tag_id").Scan(&rows)
	m := map[uint]int{}
	for _, r := range rows {
		m[r.TagID] = r.N
	}
	for i := range tags {
		tags[i].DiscussionsCount = m[tags[i].ID]
	}
}

func (s *Server) listGroups(c *gin.Context) {
	var groups []models.Group
	s.db.Where("is_hidden = ? OR id IN ?", false, []uint{models.GroupAdmin, models.GroupMod}).
		Order("id").Find(&groups)
	out := make([]GroupDTO, 0, len(groups))
	for _, g := range groups {
		out = append(out, GroupDTO{ID: g.ID, Name: g.Name(), Color: g.Color})
	}
	c.JSON(http.StatusOK, gin.H{"groups": out})
}

// loadTag 按 slug 加载版块（含可见性检查）。
func (s *Server) loadTag(c *gin.Context) (*models.Tag, []models.Tag, *models.User, bool) {
	var tag models.Tag
	if err := s.db.Where("slug = ?", c.Param("slug")).First(&tag).Error; err != nil {
		fail(c, http.StatusNotFound, "版块不存在或已被删除。")
		return nil, nil, nil, false
	}
	u := s.currentUser(c)
	if !auth.CanViewTag(u, &tag) {
		fail(c, http.StatusForbidden, "你没有查看该版块的权限。")
		return nil, nil, nil, false
	}
	var children []models.Tag
	s.db.Where("parent_id = ?", tag.ID).Order("position ASC, id ASC").Find(&children)
	s.fillTagPerms(children)
	children = auth.FilterVisibleTags(u, children)
	s.fillTagCounts(append([]models.Tag{tag}, children...))
	return &tag, children, u, true
}

func (s *Server) tagDetail(c *gin.Context) {
	tag, children, u, ok := s.loadTag(c)
	if !ok {
		return
	}
	t := s.tagDTO(tag, u)
	t.Children = make([]TagDTO, 0, len(children))
	for i := range children {
		t.Children = append(t.Children, s.tagDTO(&children[i], u))
	}
	c.JSON(http.StatusOK, gin.H{
		"tag":     t,
		"canPost": auth.CanPostTag(u, tag),
	})
}

func (s *Server) tagDiscussions(c *gin.Context) {
	tag, children, u, ok := s.loadTag(c)
	if !ok {
		return
	}
	ids := []uint{tag.ID}
	for _, ch := range children {
		ids = append(ids, ch.ID)
	}
	s.fillTagCounts(append([]models.Tag{*tag}, children...))
	base := s.db.Joins("JOIN flarum_discussion_tag dt ON dt.discussion_id = flarum_discussions.id").
		Where("dt.tag_id IN ? AND flarum_discussions.hidden_at IS NULL AND flarum_discussions.is_approved = 1", ids)
	var total int64
	base.Session(&gorm.Session{}).Distinct("flarum_discussions.id").Count(&total)

	page := pageParam(c)
	pages := paginate(int(total), page, perPageDiscussions)
	var discs []models.Discussion
	base.Session(&gorm.Session{}).
		Preload("User").Preload("Tags").Preload("LastPostUser").
		Distinct("flarum_discussions.*").
		Order("flarum_discussions.is_sticky DESC, flarum_discussions.last_posted_at DESC").
		Limit(perPageDiscussions).Offset((page - 1) * perPageDiscussions).Find(&discs)

	items := make([]DiscussionSummaryDTO, 0, len(discs))
	for i := range discs {
		items = append(items, s.discussionSummaryDTO(&discs[i], u))
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "page": PageDTO{Page: page, Pages: pages, Total: int(total)}})
}

// postForm 发帖表单：按父版块分组的可发版块。
func (s *Server) postForm(c *gin.Context) {
	u := s.currentUser(c)
	tags := s.postableTags(u)
	var tops []models.Tag
	byParent := map[uint][]models.Tag{}
	for _, t := range tags {
		if t.ParentID == nil {
			tops = append(tops, t)
		} else {
			byParent[*t.ParentID] = append(byParent[*t.ParentID], t)
		}
	}
	type group struct {
		Parent   TagDTO   `json:"parent"`
		Children []TagDTO `json:"children"`
	}
	groups := make([]group, 0, len(tops))
	for _, top := range tops {
		g := group{Parent: s.tagDTO(&top, u), Children: []TagDTO{}}
		for _, ch := range byParent[top.ID] {
			g.Children = append(g.Children, s.tagDTO(&ch, u))
		}
		if len(g.Children) == 0 {
			g.Children = append(g.Children, s.tagDTO(&top, u))
		}
		groups = append(groups, g)
	}
	c.JSON(http.StatusOK, gin.H{"groups": groups})
}

// preview 渲染 Markdown 预览（与发帖同一条消毒管线）。
func (s *Server) preview(c *gin.Context) {
	var body struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, "请求格式错误。")
		return
	}
	html, _ := markdown.Render(body.Content, nil)
	c.JSON(http.StatusOK, gin.H{"html": html})
}

// loadDiscussion 加载讨论并做可见性检查。
func (s *Server) loadDiscussion(c *gin.Context, id uint) (*models.Discussion, *models.User, bool) {
	var d models.Discussion
	if err := s.db.Preload("User").Preload("Tags").First(&d, id).Error; err != nil {
		fail(c, http.StatusNotFound, "该帖子不存在。")
		return nil, nil, false
	}
	u := s.currentUser(c)
	for _, t := range d.Tags {
		if !auth.CanViewTag(u, &t) {
			fail(c, http.StatusForbidden, "你没有查看该帖子的权限。")
			return nil, nil, false
		}
	}
	isMod := u != nil && u.IsModerator()
	if d.HiddenAt != nil && !isMod {
		fail(c, http.StatusNotFound, "该帖子不存在或已被删除。")
		return nil, nil, false
	}
	// flarum/approval：未审核帖仅版主可见
	if !d.IsApproved && !isMod {
		fail(c, http.StatusNotFound, "该帖子不存在或已被删除。")
		return nil, nil, false
	}
	return &d, u, true
}

func (s *Server) discussionDetail(c *gin.Context) {
	id, _ := parseUint(c.Param("id"))
	if id == 0 {
		fail(c, http.StatusNotFound, "该帖子不存在。")
		return
	}
	d, u, ok := s.loadDiscussion(c, id)
	if !ok {
		return
	}
	page := pageParam(c)
	pages := paginate(d.CommentCount, page, perPagePosts)

	var posts []models.Post
	s.db.Where("discussion_id = ? AND type = ? AND is_approved = ?", d.ID, "comment", true).Order("number ASC").
		Limit(perPagePosts).Offset((page - 1) * perPagePosts).Preload("User").Find(&posts)

	liked := s.loadLikes(posts, u)

	isMod := u != nil && u.IsModerator()
	views := make([]PostDTO, 0, len(posts))
	for i := range posts {
		if posts[i].HiddenAt != nil && !isMod {
			continue
		}
		dto := s.postDTO(&posts[i], liked, u)
		dto.Discussion.Slug = d.Slug()
		views = append(views, dto)
	}
	canReply := u != nil && !d.IsLocked
	for _, t := range d.Tags {
		if !auth.CanPostTag(u, &t) {
			canReply = false
		}
	}
	var editorTags []TagDTO
	if isMod {
		for _, t := range s.postableTags(u) {
			editorTags = append(editorTags, s.tagDTO(&t, u))
		}
	}
	summary := s.discussionSummaryDTO(d, u)
	c.JSON(http.StatusOK, gin.H{
		"discussion": summary,
		"posts":      views,
		"page":       PageDTO{Page: page, Pages: pages, Total: d.CommentCount},
		"canReply":   canReply,
		"isMod":      isMod,
		"editorTags": editorTags,
	})
}

func (s *Server) createDiscussion(c *gin.Context) {
	u := s.currentUser(c)
	var body struct {
		Title   string `json:"title"`
		Content string `json:"content"`
		TagIDs  []uint `json:"tagIds"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, "请求格式错误。")
		return
	}
	title := strings.TrimSpace(body.Title)
	content := strings.TrimSpace(body.Content)

	postable := s.postableTags(u)
	okIDs := map[uint]models.Tag{}
	for _, t := range postable {
		okIDs[t.ID] = t
	}
	var chosen []models.Tag
	for _, id := range body.TagIDs {
		if t, ok := okIDs[id]; ok && len(chosen) < 5 {
			chosen = append(chosen, t)
		}
	}
	if len([]rune(title)) < 3 || len([]rune(title)) > 200 {
		fail(c, http.StatusBadRequest, "标题长度需在 3-200 字之间。")
		return
	}
	if content == "" {
		fail(c, http.StatusBadRequest, "正文不能为空。")
		return
	}
	if len(chosen) == 0 {
		fail(c, http.StatusBadRequest, "请至少选择一个你有发帖权限的版块。")
		return
	}
	now := time.Now()
	one := 1
	uid := u.ID
	// 不内嵌 Tags（GORM Create 会跟随关联对 flarum_tags 做隐式 UPSERT），
	// 改为建帖后用 Association 显式写 flarum_discussion_tag。
	d := models.Discussion{
		Title: title, CommentCount: 1, ParticipantCount: 1, PostNumberIndex: 1,
		CreatedAt: now, LastPostedAt: &now, LastPostNumber: &one,
		SlugCol: slugify(title),
		UserID:  &uid, LastPostUserID: &uid, IsApproved: true,
	}
	post := models.Post{Number: &one, UserID: &uid, CreatedAt: now, Type: "comment", IsApproved: true}
	post.SetMarkdown(content)
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&d).Error; err != nil {
			return err
		}
		post.DiscussionID = d.ID
		if err := tx.Create(&post).Error; err != nil {
			return err
		}
		if err := tx.Model(&d).Association("Tags").Append(&chosen); err != nil {
			return err
		}
		if err := tx.Model(&models.Discussion{}).Where("id = ?", d.ID).
			Updates(map[string]any{"first_post_id": post.ID, "last_post_id": post.ID}).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.User{}).Where("id = ?", u.ID).
			UpdateColumn("discussion_count", gorm.Expr("discussion_count + 1")).Error; err != nil {
			return err
		}
		// 同步版块冗余计数（PHP 端语义）
		for _, t := range chosen {
			if err := tx.Model(&models.Tag{}).Where("id = ?", t.ID).Updates(map[string]any{
				"discussion_count":          gorm.Expr("discussion_count + 1"),
				"last_posted_at":            now,
				"last_posted_discussion_id": d.ID,
				"last_posted_user_id":       uid,
			}).Error; err != nil {
				return err
			}
		}
		for _, name := range markdown.MentionedUsernames(content, s.mentionResolver()) {
			var mid uint
			tx.Model(&models.User{}).Select("id").Where("lower(username) = lower(?)", name).Scan(&mid)
			notify(tx, mid, u.ID, "mention", d.ID, post.ID)
		}
		return nil
	})
	if err != nil {
		log.Printf("[api] createDiscussion 事务失败 user=%s: %v", u.Username, err)
		fail(c, http.StatusInternalServerError, "保存失败，请重试。")
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": d.ID, "slug": d.Slug(), "url": discussionURL(&d)})
}

func (s *Server) reply(c *gin.Context) {
	u := s.currentUser(c)
	id, _ := parseUint(c.Param("id"))
	var d models.Discussion
	if err := s.db.Preload("Tags").First(&d, id).Error; err != nil {
		fail(c, http.StatusNotFound, "该帖子不存在。")
		return
	}
	if d.IsLocked {
		fail(c, http.StatusBadRequest, "该帖子已被锁定，无法回复。")
		return
	}
	if d.HiddenAt != nil {
		fail(c, http.StatusBadRequest, "该帖子已被隐藏。")
		return
	}
	if !d.IsApproved {
		fail(c, http.StatusBadRequest, "该帖子尚未通过审核。")
		return
	}
	for _, t := range d.Tags {
		if !auth.CanPostTag(u, &t) {
			fail(c, http.StatusForbidden, "你没有在该版块回帖的权限。")
			return
		}
	}
	var body struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.Content) == "" {
		fail(c, http.StatusBadRequest, "回复内容不能为空。")
		return
	}
	now := time.Now()
	nextNumber := d.CommentCount + 1
	uid := u.ID
	post := models.Post{
		DiscussionID: d.ID, Number: &nextNumber, UserID: &uid,
		CreatedAt: now, Type: "comment", IsApproved: true,
	}
	post.SetMarkdown(body.Content)
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&post).Error; err != nil {
			return err
		}
		// Flarum 计数语义（原子递增，避免与 PHP 端并发写竞态）
		if err := tx.Model(&models.Discussion{}).Where("id = ?", d.ID).Updates(map[string]any{
			"comment_count":       gorm.Expr("comment_count + 1"),
			"post_number_index":   gorm.Expr("post_number_index + 1"),
			"last_posted_at":      now,
			"last_post_id":        post.ID,
			"last_post_number":    nextNumber,
			"last_posted_user_id": uid,
		}).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.User{}).Where("id = ?", u.ID).
			UpdateColumn("comment_count", gorm.Expr("comment_count + 1")).Error; err != nil {
			return err
		}
		// 同步版块冗余计数
		for _, t := range d.Tags {
			if err := tx.Model(&models.Tag{}).Where("id = ?", t.ID).Updates(map[string]any{
				"last_posted_at":            now,
				"last_posted_discussion_id": d.ID,
				"last_posted_user_id":       uid,
			}).Error; err != nil {
				return err
			}
		}
		for _, name := range markdown.MentionedUsernames(body.Content, s.mentionResolver()) {
			var mid uint
			tx.Model(&models.User{}).Select("id").Where("lower(username) = lower(?)", name).Scan(&mid)
			notify(tx, mid, u.ID, "mention", d.ID, post.ID)
		}
		notifyDiscussionParticipants(tx, &d, u.ID, "reply", post.ID)
		return nil
	})
	if err != nil {
		log.Printf("[api] reply 事务失败 discussion=%d user=%s: %v", d.ID, u.Username, err)
		fail(c, http.StatusInternalServerError, "保存失败，请重试。")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"postId": post.ID, "number": post.PostNumber(),
		"page": (post.PostNumber()-1)/perPagePosts + 1,
	})
}

// moderate 版主操作：置顶/锁定/隐藏/改标题与标签。
func (s *Server) moderate(c *gin.Context) {
	u := s.currentUser(c)
	if !u.IsModerator() {
		fail(c, http.StatusForbidden, "该操作需要版主权限。")
		return
	}
	id, _ := parseUint(c.Param("id"))
	var d models.Discussion
	if err := s.db.Preload("Tags").First(&d, id).Error; err != nil {
		fail(c, http.StatusNotFound, "该帖子不存在。")
		return
	}
	var body struct {
		Action string `json:"action"`
		Title  string `json:"title"`
		TagIDs []uint `json:"tagIds"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, "请求格式错误。")
		return
	}
	switch body.Action {
	case "toggle_sticky":
		s.db.Model(&d).Update("is_sticky", !d.IsSticky)
	case "toggle_lock":
		s.db.Model(&d).Update("is_locked", !d.IsLocked)
	case "toggle_hidden":
		if d.HiddenAt != nil {
			s.db.Model(&d).Update("hidden_at", nil)
		} else {
			s.db.Model(&d).Update("hidden_at", time.Now())
		}
	case "edit_meta":
		updates := map[string]any{}
		if title := strings.TrimSpace(body.Title); title != "" {
			updates["title"] = title
		}
		if len(body.TagIDs) > 0 {
			postable := s.postableTags(u)
			okIDs := map[uint]bool{}
			for _, t := range postable {
				okIDs[t.ID] = true
			}
			var tags []models.Tag
			for _, tid := range body.TagIDs {
				if okIDs[tid] {
					tags = append(tags, models.Tag{ID: tid})
				}
			}
			if len(tags) > 0 {
				if err := s.db.Model(&d).Association("Tags").Replace(tags); err != nil {
					fail(c, http.StatusInternalServerError, "标签更新失败。")
					return
				}
			}
		}
		if len(updates) > 0 {
			s.db.Model(&d).Updates(updates)
		}
	default:
		fail(c, http.StatusBadRequest, "未知操作。")
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ---------- 帖子级操作 ----------

func (s *Server) loadPost(c *gin.Context) (*models.Post, *models.User, bool) {
	id, _ := parseUint(c.Param("id"))
	var p models.Post
	if err := s.db.First(&p, id).Error; err != nil {
		fail(c, http.StatusNotFound, "该帖子不存在。")
		return nil, nil, false
	}
	u := s.currentUser(c)
	if u == nil {
		fail(c, http.StatusUnauthorized, "请先登录。")
		return nil, nil, false
	}
	return &p, u, true
}

func (s *Server) postEditGet(c *gin.Context) {
	p, _, ok := s.loadPost(c)
	if !ok {
		return
	}
	u := s.currentUser(c)
	if !auth.CanEditPost(u, p) {
		fail(c, http.StatusForbidden, "你只能编辑自己的回复。")
		return
	}
	var d models.Discussion
	s.db.Select("id, slug").First(&d, p.DiscussionID)
	c.JSON(http.StatusOK, gin.H{
		"id":         p.ID,
		"content":    p.Markdown(),
		"discussion": gin.H{"id": d.ID, "slug": d.Slug(), "title": d.Title},
	})
}

func (s *Server) editPost(c *gin.Context) {
	p, _, ok := s.loadPost(c)
	if !ok {
		return
	}
	u := s.currentUser(c)
	if !auth.CanEditPost(u, p) {
		fail(c, http.StatusForbidden, "你只能编辑自己的回复。")
		return
	}
	var body struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.Content) == "" {
		fail(c, http.StatusBadRequest, "内容不能为空。")
		return
	}
	now := time.Now()
	edited := now
	s.db.Model(p).Updates(map[string]any{"edited_at": edited})
	p.SetMarkdown(body.Content)
	s.db.Model(p).Update("content", p.Content)
	authorID := uint(0)
	if p.UserID != nil {
		authorID = *p.UserID
	}
	for _, name := range markdown.MentionedUsernames(body.Content, s.mentionResolver()) {
		var uid uint
		s.db.Model(&models.User{}).Select("id").Where("lower(username) = lower(?)", name).Scan(&uid)
		notify(s.db, uid, authorID, "mention", p.DiscussionID, p.ID)
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "editedAt": edited})
}

func (s *Server) hidePost(c *gin.Context) {
	p, u, ok := s.loadPost(c)
	if !ok {
		return
	}
	if (p.UserID == nil || *p.UserID != u.ID) && !u.IsModerator() {
		fail(c, http.StatusForbidden, "无权隐藏该回复。")
		return
	}
	if p.HiddenAt != nil {
		s.db.Model(p).Update("hidden_at", nil)
	} else {
		s.db.Model(p).Update("hidden_at", time.Now())
	}
	c.JSON(http.StatusOK, gin.H{"hidden": p.HiddenAt != nil})
}

func (s *Server) toggleLike(c *gin.Context) {
	p, u, ok := s.loadPost(c)
	if !ok {
		return
	}
	var like models.PostLike
	liked := false
	if err := s.db.Where("post_id = ? AND user_id = ?", p.ID, u.ID).First(&like).Error; err == nil {
		s.db.Delete(&like)
	} else {
		if err := s.db.Create(&models.PostLike{PostID: p.ID, UserID: u.ID}).Error; err != nil {
			fail(c, http.StatusInternalServerError, "操作失败。")
			return
		}
		liked = true
		if p.UserID != nil {
			notify(s.db, *p.UserID, u.ID, "like", p.DiscussionID, p.ID)
		}
	}
	var n int64
	s.db.Model(&models.PostLike{}).Where("post_id = ?", p.ID).Count(&n)
	c.JSON(http.StatusOK, gin.H{"liked": liked, "likeCount": n})
}

// ---------- 用户主页 / 搜索 ----------

func (s *Server) profile(c *gin.Context) {
	var p models.User
	if err := s.db.Preload("Groups").Where("lower(username) = lower(?)", c.Param("username")).First(&p).Error; err != nil {
		fail(c, http.StatusNotFound, "该用户不存在。")
		return
	}
	viewer := s.currentUser(c)
	canSeeHidden := viewer != nil && viewer.IsModerator()
	dQuery := s.db.Model(&models.Discussion{}).Where("flarum_discussions.user_id = ?", p.ID)
	if !canSeeHidden {
		dQuery = dQuery.Where("flarum_discussions.hidden_at IS NULL AND flarum_discussions.is_approved = 1")
	}
	var total int64
	dQuery.Session(&gorm.Session{}).Count(&total)
	var discs []models.Discussion
	dQuery.Session(&gorm.Session{}).Preload("User").Preload("Tags").
		Order("flarum_discussions.last_posted_at DESC").Limit(10).Find(&discs)

	var posts []models.Post
	s.db.Where("user_id = ? AND hidden_at IS NULL AND type = ? AND is_approved = ?", p.ID, "comment", true).
		Preload("Discussion").Preload("User").
		Order("created_at DESC").Limit(10).Find(&posts)
	type postRow struct {
		Post       PostDTO               `json:"post"`
		Discussion *DiscussionSummaryDTO `json:"discussion"`
	}
	rows := make([]postRow, 0, len(posts))
	for i := range posts {
		po := &posts[i]
		if po.Discussion == nil || po.Discussion.HiddenAt != nil {
			continue
		}
		rows = append(rows, postRow{Post: s.postDTO(po, nil, viewer), Discussion: s.discussionSummaryPTR(po.Discussion, viewer)})
	}
	c.JSON(http.StatusOK, gin.H{
		"user":        userDTO(&p),
		"discTotal":   total,
		"discussions": s.summaries(discs, viewer),
		"posts":       rows,
	})
}

func (s *Server) summaries(discs []models.Discussion, u *models.User) []DiscussionSummaryDTO {
	out := make([]DiscussionSummaryDTO, 0, len(discs))
	for i := range discs {
		out = append(out, s.discussionSummaryDTO(&discs[i], u))
	}
	return out
}

func (s *Server) discussionSummaryPTR(d *models.Discussion, u *models.User) *DiscussionSummaryDTO {
	dto := s.discussionSummaryDTO(d, u)
	return &dto
}

func (s *Server) search(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	data := gin.H{"q": q}
	if len([]rune(q)) >= 2 {
		like := "%" + q + "%"
		var discs []models.Discussion
		s.db.Where("hidden_at IS NULL AND is_approved = 1 AND title LIKE ?", like).
			Preload("User").Preload("Tags").
			Order("last_posted_at DESC").Limit(20).Find(&discs)
		var posts []models.Post
		s.db.Where("hidden_at IS NULL AND is_approved = 1 AND content LIKE ? AND type = ?", like, "comment").
			Preload("User").Preload("Discussion").
			Order("created_at DESC").Limit(20).Find(&posts)
		data["discussions"] = s.summaries(discs, s.currentUser(c))
		var rows []gin.H
		for i := range posts {
			po := &posts[i]
			if po.Discussion == nil || po.Discussion.HiddenAt != nil {
				continue
			}
			rows = append(rows, gin.H{
				"post":       s.postDTO(po, nil, s.currentUser(c)),
				"discussion": s.discussionSummaryPTR(po.Discussion, s.currentUser(c)),
			})
		}
		data["posts"] = rows
	}
	c.JSON(http.StatusOK, data)
}
