package main

import (
	"crypto/sha256"
	"fmt"
	"log"
	"strings"

	"github.com/exzco/bbs/internal/db"
	"github.com/exzco/bbs/internal/flarum"
	"github.com/exzco/bbs/internal/models"
)

// auditContentCmd is read-only: hashes can be compared to s9e's PHP Unparser
// without exporting private post text or mutating the shared legacy tables.
func auditContentCmd(args []string) {
	cfg := loadCfg(args)
	g, err := db.Open(cfg.Database.Driver, cfg.Database.DSN)
	if err != nil {
		log.Fatal(err)
	}
	rows, err := g.Model(&models.Post{}).Select("id, content").Where("type = ?", "comment").Order("id").Rows()
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id uint
		var content string
		if err := rows.Scan(&id, &content); err != nil {
			log.Fatal(err)
		}
		source := flarum.DecodeContent(content)
		if (strings.HasPrefix(content, "<r>") || strings.HasPrefix(content, "<t>")) && source == content {
			log.Fatalf("invalid legacy XML in post %d", id)
		}
		fmt.Printf("%d\t%x\n", id, sha256.Sum256([]byte(source)))
	}
	if err := rows.Err(); err != nil {
		log.Fatal(err)
	}
}
