package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/exzco/bbs/internal/auth"
	"github.com/exzco/bbs/internal/config"
	"github.com/exzco/bbs/internal/db"
	"github.com/exzco/bbs/internal/models"
	"github.com/exzco/bbs/internal/web"
	"github.com/gorilla/csrf"
	"gorm.io/gorm"
)

func main() {
	log.SetFlags(log.LstdFlags)
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "serve":
			serveCmd(os.Args[2:])
			return
		case "seed-admin":
			seedAdminCmd(os.Args[2:])
			return
		default:
			usage()
			os.Exit(2)
		}
	}
	usage()
}

func usage() {
	fmt.Fprintf(os.Stderr, `JNSEC BBS (Go)

用法:
  bbs serve [-c config.yaml]              启动服务
  bbs seed-admin [-c config.yaml] -username x -email x@y -password xxx
                                          创建/提升管理员
`)
}

func loadCfg(args []string) *config.Config {
	fs := flag.NewFlagSet("bbs", flag.ContinueOnError)
	cfgPath := fs.String("c", "config.yaml", "配置文件路径")
	_ = fs.Parse(args)
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	return cfg
}

func openDB(cfg *config.Config, seed bool) *gorm.DB {
	g, err := db.Open(cfg.Database.Driver, cfg.Database.DSN)
	if err != nil {
		log.Fatalf("连接数据库失败: %v", err)
	}
	if err := db.Migrate(g, seed); err != nil {
		log.Fatalf("数据库迁移失败: %v", err)
	}
	return g
}

// csrfKey 保证 32 字节：过短时用 SHA-256 派生。
func csrfKey(secret string) []byte {
	if len(secret) >= 32 {
		return []byte(secret)
	}
	sum := sha256.Sum256([]byte("jnsec-bbs-csrf:" + secret))
	return sum[:]
}

// plaintextMark 在请求上下文中标记明文 HTTP，供 gorilla/csrf 放宽 Origin/Referer 校验。
func plaintextMark(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, csrf.PlaintextHTTPRequest(r))
	})
}

func serveCmd(args []string) {
	cfg := loadCfg(args)
	if cfg.SessionSecret == "" || cfg.SessionSecret == "CHANGE-ME-to-a-random-64-char-string" {
		log.Println("[警告] session_secret 未配置或仍为默认值，生产环境请在 config.yaml 中设置随机密钥。")
	}
	g := openDB(cfg, true)

	srv, err := web.New(cfg, g)
	if err != nil {
		log.Fatalf("初始化 Web 服务失败: %v", err)
	}
	// 明文 HTTP 部署需在 CSRF 中间件之前（外层）标记，否则按 HTTPS 比对 Origin/Referer 导致全部 403
	opts := []csrf.Option{
		csrf.Secure(cfg.CookieSecure),
		csrf.CookieName("jnsec_csrf"),
		csrf.FieldName("_csrf"),
		csrf.RequestHeader("X-CSRF-Token"),
	}
	if len(cfg.TrustedOrigins) > 0 {
		opts = append(opts, csrf.TrustedOrigins(cfg.TrustedOrigins))
	}
	opts = append(opts, csrf.ErrorHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, fmt.Sprintf("CSRF 校验失败，请刷新页面重试。（原因: %v）", csrf.FailureReason(r)), http.StatusForbidden)
	})))
	handler := csrf.Protect(csrfKey(cfg.SessionSecret), opts...)(srv.Handler())
	if !cfg.CookieSecure {
		handler = plaintextMark(handler)
	}

	httpSrv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		log.Printf("%s 启动于 http://%s", cfg.SiteName, cfg.Listen)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP 服务异常退出: %v", err)
		}
	}()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("正在关闭…")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(ctx)
}

func seedAdminCmd(args []string) {
	fs := flag.NewFlagSet("seed-admin", flag.ContinueOnError)
	cfgPath := fs.String("c", "config.yaml", "配置文件路径")
	username := fs.String("username", "", "管理员用户名")
	email := fs.String("email", "", "邮箱")
	password := fs.String("password", "", "密码（至少 8 位）")
	_ = fs.Parse(args)
	cfg := loadCfg([]string{"-c", *cfgPath})
	g := openDB(cfg, true)

	if *username == "" || *email == "" || len(*password) < 8 {
		log.Fatal("需要 -username -email -password（密码至少 8 位）")
	}
	hash, err := auth.HashPassword(*password)
	if err != nil {
		log.Fatalf("生成密码失败: %v", err)
	}
	var u models.User
	err = g.Where("lower(username) = lower(?)", *username).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		u = models.User{Username: *username, Email: *email, Password: hash}
		if err := g.Create(&u).Error; err != nil {
			log.Fatalf("创建管理员失败: %v", err)
		}
		fmt.Printf("已创建管理员 %s (ID=%d)\n", u.Username, u.ID)
	} else if err != nil {
		log.Fatalf("查询用户失败: %v", err)
	} else {
		updates := map[string]any{"password": hash, "email": *email, "suspended_until": nil}
		if err := g.Model(&u).Updates(updates).Error; err != nil {
			log.Fatalf("更新管理员失败: %v", err)
		}
		fmt.Printf("已将现有用户 %s (ID=%d) 提升为管理员并重置密码\n", u.Username, u.ID)
	}
	admin := models.Group{ID: models.GroupAdmin}
	if err := g.Model(&u).Association("Groups").Append(&admin); err != nil {
		log.Fatalf("设置管理员组失败: %v", err)
	}
}
