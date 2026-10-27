package main

import (
	"log"

	"github.com/exzco/bbs/internal/db"
)

func migrateDBCmd(args []string) {
	cfg := loadCfg(args)
	g, err := db.Open(cfg.Database.Driver, cfg.Database.DSN)
	if err != nil {
		log.Fatalf("连接数据库失败: %v", err)
	}
	if err := db.Migrate(g); err != nil {
		log.Fatalf("数据库迁移失败: %v", err)
	}
	log.Println("Go 辅助表迁移完成。")
}
