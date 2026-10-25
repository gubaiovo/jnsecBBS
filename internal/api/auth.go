package api

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"image/png"
	"io"
	"log"
	"net/http"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dchest/captcha"
	"github.com/disintegration/imaging"
	"github.com/exzco/bbs/internal/auth"
	"github.com/exzco/bbs/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/csrf"
	"gorm.io/gorm"
)

// me 当前会话信息：用户 + CSRF token + 站点设置。
func (s *Server) me(c *gin.Context) {
	u := s.currentUser(c)
	resp := gin.H{
		"user":         userDTO(u),
		"csrfToken":    csrf.Token(c.Request),
		"siteName":     s.settingValue(models.SettingKeySiteName),
		"registration": s.settingValue(models.SettingKeyRegistration),
		"welcomeText":  s.settingValue(models.SettingKeyWelcomeText),
	}
	if u != nil {
		var unread int64
		s.db.Model(&models.Notification{}).Where("user_id = ? AND read_at IS NULL", u.ID).Count(&unread)
		resp["unread"] = unread
	}
	c.JSON(http.StatusOK, resp)
}

func (s *Server) captchaNew(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"id": captcha.New()})
}

func (s *Server) login(c *gin.Context) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Username == "" || body.Password == "" {
		fail(c, http.StatusBadRequest, "请输入用户名和密码。")
		return
	}
	var u models.User
	// Flarum 语义：登录标识可以是用户名或邮箱
	if err := s.db.Preload("Groups").
		Where("lower(username) = lower(?) OR lower(email) = lower(?)", body.Username, body.Username).
		First(&u).Error; err != nil ||
		!auth.CheckPassword(u.Password, body.Password) {
		fail(c, http.StatusBadRequest, "用户名或密码错误。")
		return
	}
	if u.IsBanned() {
		fail(c, http.StatusForbidden, "该账号已被封禁，如有疑问请联系管理员。")
		return
	}
	s.startSession(c, &u)
}

func (s *Server) startSession(c *gin.Context, u *models.User) {
	token, err := auth.CreateSession(s.db, u.ID, c.Request.UserAgent(), s.cfg.CookieSecure)
	if err != nil {
		log.Printf("[api] CreateSession 失败 user=%s: %v", u.Username, err)
		fail(c, http.StatusInternalServerError, "登录失败，请重试。")
		return
	}
	auth.SetSessionCookie(c.Writer, token, s.cfg.CookieSecure)
	now := time.Now()
	s.db.Model(u).Update("last_seen_at", &now)
	var unread int64
	s.db.Model(&models.Notification{}).Where("user_id = ? AND read_at IS NULL", u.ID).Count(&unread)
	c.JSON(http.StatusOK, gin.H{"user": userDTO(u), "unread": unread, "csrfToken": csrf.Token(c.Request)})
}

func (s *Server) register(c *gin.Context) {
	var body struct {
		Username        string `json:"username"`
		Email           string `json:"email"`
		Password        string `json:"password"`
		PasswordConfirm string `json:"passwordConfirm"`
		Invite          string `json:"invite"`
		CaptchaID       string `json:"captchaId"`
		CaptchaValue    string `json:"captchaValue"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, "请求格式错误。")
		return
	}
	username := strings.TrimSpace(body.Username)
	email := strings.TrimSpace(strings.ToLower(body.Email))

	if s.settingValue(models.SettingKeyRegistration) == "invite" {
		var inv models.InviteCode
		if err := s.db.Where("code = ?", strings.TrimSpace(body.Invite)).First(&inv).Error; err != nil {
			fail(c, http.StatusBadRequest, "邀请码无效。")
			return
		}
		if inv.UsedBy != nil || (inv.ExpiresAt != nil && inv.ExpiresAt.Before(time.Now())) {
			fail(c, http.StatusBadRequest, "邀请码已被使用或已过期。")
			return
		}
	} else if !captcha.VerifyString(body.CaptchaID, body.CaptchaValue) {
		fail(c, http.StatusBadRequest, "验证码错误。")
		return
	}
	if !usernameRe.MatchString(username) {
		fail(c, http.StatusBadRequest, "用户名需为 2-20 位字母、数字、下划线或连字符。")
		return
	}
	if _, err := mail.ParseAddress(email); err != nil {
		fail(c, http.StatusBadRequest, "邮箱格式不正确。")
		return
	}
	if len(body.Password) < 8 {
		fail(c, http.StatusBadRequest, "密码至少 8 位。")
		return
	}
	if body.Password != body.PasswordConfirm {
		fail(c, http.StatusBadRequest, "两次输入的密码不一致。")
		return
	}
	var cnt int64
	s.db.Model(&models.User{}).Where("lower(username) = lower(?)", username).Count(&cnt)
	if cnt > 0 {
		fail(c, http.StatusBadRequest, "用户名已被占用。")
		return
	}
	s.db.Model(&models.User{}).Where("lower(email) = lower(?)", email).Count(&cnt)
	if cnt > 0 {
		fail(c, http.StatusBadRequest, "邮箱已被注册。")
		return
	}
	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		fail(c, http.StatusInternalServerError, "注册失败，请重试。")
		return
	}
	u := models.User{
		Username: username, Email: email, Password: hash,
		IsEmailConfirmed: true, Preferences: []byte("a:0:{}"),
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&u).Error; err != nil {
			return err
		}
		member := models.Group{ID: models.GroupMember}
		return tx.Model(&u).Association("Groups").Append(&member)
	})
	if err != nil {
		log.Printf("[api] register 事务失败 username=%s: %v", username, err)
		fail(c, http.StatusInternalServerError, "注册失败，请重试。")
		return
	}
	if s.settingValue(models.SettingKeyRegistration) == "invite" {
		s.db.Model(&models.InviteCode{}).Where("code = ?", strings.TrimSpace(body.Invite)).
			Updates(map[string]any{"used_by": u.ID, "used_at": time.Now()})
	}
	s.startSession(c, &u)
}

func (s *Server) logout(c *gin.Context) {
	if v, ok := c.Get("token"); ok {
		if t, ok := v.(string); ok {
			auth.DestroySession(s.db, t)
		}
	}
	auth.ClearSessionCookie(c.Writer, s.cfg.CookieSecure)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ---------- 个人设置 ----------

func (s *Server) updateProfile(c *gin.Context) {
	u := s.currentUser(c)
	var body struct {
		Bio string `json:"bio"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, "请求格式错误。")
		return
	}
	bio := strings.TrimSpace(body.Bio)
	if len([]rune(bio)) > 500 {
		bio = string([]rune(bio)[:500])
	}
	s.db.Model(u).Update("bio", bio)
	c.JSON(http.StatusOK, gin.H{"user": userDTO(u)})
}

func (s *Server) updatePassword(c *gin.Context) {
	u := s.currentUser(c)
	var body struct {
		Current string `json:"current"`
		New     string `json:"new"`
		Confirm string `json:"confirm"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, "请求格式错误。")
		return
	}
	if !auth.CheckPassword(u.Password, body.Current) {
		fail(c, http.StatusBadRequest, "当前密码不正确。")
		return
	}
	if len(body.New) < 8 {
		fail(c, http.StatusBadRequest, "新密码至少 8 位。")
		return
	}
	if body.New != body.Confirm {
		fail(c, http.StatusBadRequest, "两次输入的新密码不一致。")
		return
	}
	hash, err := auth.HashPassword(body.New)
	if err != nil {
		fail(c, http.StatusInternalServerError, "修改失败，请重试。")
		return
	}
	s.db.Model(u).Update("password", hash)
	// 改密后踢掉其他设备的会话
	current := ""
	if v, ok := c.Get("token"); ok {
		if t, ok := v.(string); ok {
			current = t
		}
	}
	s.db.Where("user_id = ? AND token <> ?", u.ID, current).Delete(&models.AccessToken{})
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) updateAvatar(c *gin.Context) {
	u := s.currentUser(c)
	if c.PostForm("action") == "remove" {
		old := u.AvatarURL
		if err := s.db.Model(u).Update("avatar_url", "").Error; err != nil {
			fail(c, http.StatusInternalServerError, "删除头像失败。")
			return
		}
		s.removeAvatarFile(old)
		c.JSON(http.StatusOK, gin.H{"avatar": avatarURL(u)})
		return
	}
	fh, err := c.FormFile("file")
	if err != nil {
		fail(c, http.StatusBadRequest, "请选择头像文件。")
		return
	}
	if fh.Size > 2<<20 {
		fail(c, http.StatusBadRequest, "头像文件不能超过 2MB。")
		return
	}
	f, err := fh.Open()
	if err != nil {
		fail(c, http.StatusBadRequest, "读取文件失败。")
		return
	}
	defer f.Close()
	head := make([]byte, 512)
	n, _ := f.Read(head)
	ct := http.DetectContentType(head[:n])
	if ct != "image/png" && ct != "image/jpeg" {
		fail(c, http.StatusBadRequest, "仅支持 PNG / JPG 图片。")
		return
	}
	if sk, ok := f.(io.Seeker); ok {
		if _, err := sk.Seek(0, io.SeekStart); err != nil {
			fail(c, http.StatusBadRequest, "读取文件失败。")
			return
		}
	}
	img, err := imaging.Decode(f)
	if err != nil {
		fail(c, http.StatusBadRequest, "图片解析失败。")
		return
	}
	thumb := imaging.Thumbnail(img, 128, 128, imaging.Lanczos)
	name, _ := randHex(16)
	dest := filepath.Join(s.cfg.DataDir, "avatars", name+".png")
	if err := imaging.Save(thumb, dest, imaging.PNGCompressionLevel(png.DefaultCompression)); err != nil {
		fail(c, http.StatusInternalServerError, "保存头像失败。")
		return
	}
	old := u.AvatarURL
	// 写入 flarum 的 avatar_url：指向新站自己的头像路由（data/avatars 由 app 伺服）
	url := "/avatars/" + name + ".png"
	if err := s.db.Model(u).Update("avatar_url", url).Error; err != nil {
		_ = os.Remove(dest)
		fail(c, http.StatusInternalServerError, "保存头像失败。")
		return
	}
	s.removeAvatarFile(old)
	c.JSON(http.StatusOK, gin.H{"avatar": url})
}

// removeAvatarFile 删除 data/avatars 下的头像文件（flarum /assets 路径不归 Go 管，跳过）。
func (s *Server) removeAvatarFile(url string) {
	if url == "" || !strings.HasPrefix(url, "/avatars/") {
		return
	}
	_ = os.Remove(filepath.Join(s.cfg.DataDir, "avatars", filepath.Base(url)))
}

func randHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// ---------- 通知 ----------

type NotificationDTO struct {
	ID         uint                  `json:"id"`
	Type       string                `json:"type"`
	Sender     *UserDTO              `json:"sender"`
	Discussion *DiscussionSummaryDTO `json:"discussion,omitempty"`
	PostID     uint                  `json:"postId"`
	Read       bool                  `json:"read"`
	CreatedAt  time.Time             `json:"createdAt"`
}

func (s *Server) notifications(c *gin.Context) {
	u := s.currentUser(c)
	var notifs []models.Notification
	s.db.Where("user_id = ?", u.ID).
		Preload("Sender").Preload("Discussion").
		Order("created_at DESC").Limit(50).Find(&notifs)
	items := make([]NotificationDTO, 0, len(notifs))
	for _, n := range notifs {
		item := NotificationDTO{
			ID: n.ID, Type: n.Type, Sender: userDTO(&n.Sender),
			PostID: n.PostID, Read: n.ReadAt != nil, CreatedAt: n.CreatedAt,
		}
		if n.Discussion != nil {
			d := s.discussionSummaryDTO(n.Discussion, u)
			item.Discussion = &d
		}
		items = append(items, item)
	}
	var unread int64
	s.db.Model(&models.Notification{}).Where("user_id = ? AND read_at IS NULL", u.ID).Count(&unread)
	c.JSON(http.StatusOK, gin.H{"items": items, "unread": unread})
}

func (s *Server) notificationsRead(c *gin.Context) {
	u := s.currentUser(c)
	now := time.Now()
	s.db.Model(&models.Notification{}).
		Where("user_id = ? AND read_at IS NULL", u.ID).Update("read_at", now)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

var _ = fmt.Sprintf
