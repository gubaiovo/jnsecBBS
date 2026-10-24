package auth

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/exzco/bbs/internal/models"
	"gorm.io/gorm"
)

const SessionCookie = "jnsec_session"

// NewToken 生成随机 token（20 字节 = 40 个 hex 字符——必须 ≤ flarum_access_tokens.token
// 的 varchar(40)，32 字节会触发 Data too long）。
func NewToken() (string, error) {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// CreateSession 写入 flarum_access_tokens（与 PHP 共享 token 存储）。
func CreateSession(g *gorm.DB, userID uint, ua string, secure bool) (string, error) {
	token, err := NewToken()
	if err != nil {
		return "", err
	}
	now := time.Now()
	at := models.AccessToken{
		Token: token, UserID: userID,
		CreatedAt: now, LastActivityAt: now, Type: "session",
	}
	if err := g.Create(&at).Error; err != nil {
		return "", err
	}
	return token, nil
}

func SetSessionCookie(w http.ResponseWriter, token string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name: SessionCookie, Value: token, Path: "/",
		HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode,
		MaxAge: 60 * 60 * 24 * 30,
	})
}

func ClearSessionCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name: SessionCookie, Value: "", Path: "/",
		HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: -1,
	})
}

// UserFromRequest 依据 Cookie 中的 token 加载当前用户（含用户组）。
func UserFromRequest(g *gorm.DB, r *http.Request) (*models.User, *models.AccessToken) {
	c, err := r.Cookie(SessionCookie)
	if err != nil || c.Value == "" {
		return nil, nil
	}
	var at models.AccessToken
	if err := g.Where("token = ?", c.Value).First(&at).Error; err != nil {
		return nil, nil
	}
	var u models.User
	if err := g.Preload("Groups").First(&u, at.UserID).Error; err != nil {
		return nil, nil
	}
	// 活跃度更新（每小时一次，避免每请求写库）
	if time.Since(at.LastActivityAt) > time.Hour {
		g.Model(&models.AccessToken{}).Where("id = ?", at.ID).
			Update("last_activity_at", time.Now())
	}
	return &u, &at
}

func DestroySession(g *gorm.DB, token string) {
	if token != "" {
		g.Where("token = ?", token).Delete(&models.AccessToken{})
	}
}
