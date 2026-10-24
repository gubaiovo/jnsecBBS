package auth

import (
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// CheckPassword 校验密码。兼容从 Flarum 导入的 PHP bcrypt 哈希：
// PHP password_hash 产出 "$2y$" 前缀，Go 的 bcrypt 只认 "$2a$/2b$"，这里做前缀归一。
func CheckPassword(hash, plain string) bool {
	hash = strings.TrimSpace(hash)
	if strings.HasPrefix(hash, "$2y$") {
		hash = "$2a$" + hash[4:]
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

const HashCost = 12

func HashPassword(plain string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(plain), HashCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(h), nil
}
