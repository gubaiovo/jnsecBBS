package migration

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/exzco/bbs/internal/flarum"
)

type mapping struct {
	target          string
	columns, source []string
}

var mappings = map[string]mapping{
	"flarum_users": {"users",
		[]string{"id", "username", "email", "password_hash", "email_confirmed", "bio", "avatar_path", "joined_at", "last_seen_at", "suspended_until"},
		[]string{"id", "username", "email", "password", "is_email_confirmed", "bio", "avatar_url", "joined_at", "last_seen_at", "suspended_until"}},
	"flarum_groups": {"groups",
		[]string{"id", "name_singular", "name_plural", "color", "icon", "is_hidden"},
		[]string{"id", "name_singular", "name_plural", "color", "icon", "is_hidden"}},
	"flarum_group_user": {"user_groups", []string{"user_id", "group_id"}, []string{"user_id", "group_id"}},
	"flarum_tags": {"tags",
		[]string{"id", "name", "slug", "description", "color", "icon", "parent_id", "position", "is_restricted", "is_hidden"},
		[]string{"id", "name", "slug", "description", "color", "icon", "parent_id", "position", "is_restricted", "is_hidden"}},
	"flarum_discussion_tag": {"discussion_tags", []string{"discussion_id", "tag_id"}, []string{"discussion_id", "tag_id"}},
	"flarum_discussions": {"discussions",
		[]string{"id", "title", "slug", "author_id", "created_at", "first_post_id", "last_post_id", "last_posted_at", "last_posted_user_id", "hidden_at", "hidden_by", "is_private", "is_approved", "is_locked", "is_sticky", "comment_count"},
		[]string{"id", "title", "slug", "user_id", "created_at", "first_post_id", "last_post_id", "last_posted_at", "last_posted_user_id", "hidden_at", "hidden_user_id", "is_private", "is_approved", "is_locked", "is_sticky", "comment_count"}},
	"flarum_post_likes":      {"post_likes", []string{"post_id", "user_id", "created_at"}, []string{"post_id", "user_id", "created_at"}},
	"flarum_discussion_user": {"discussion_reads", []string{"user_id", "discussion_id", "last_read_at", "last_read_number"}, []string{"user_id", "discussion_id", "last_read_at", "last_read_post_number"}},
}

var postColumns = []string{"id", "discussion_id", "number", "user_id", "type", "content", "created_at", "edited_at", "edited_user_id", "hidden_at", "hidden_user_id", "is_private", "is_approved"}

func requiredColumns(table string, columns []string) error {
	var required []string
	if m, ok := mappings[table]; ok {
		required = m.source
	}
	if table == "flarum_posts" {
		required = postColumns
	}
	if table == "flarum_group_permission" {
		required = []string{"group_id", "permission"}
	}
	if table == "flarum_discussion_user" {
		required = append(append([]string(nil), required...), "subscription")
	}
	present := make(map[string]bool, len(columns))
	for _, c := range columns {
		present[c] = true
	}
	for _, c := range required {
		if !present[c] {
			return errors.New("native conversion requires a missing core source column; review mapping")
		}
	}
	return nil
}

func insert(ctx context.Context, tx *sql.Tx, table string, columns []string, values []any) error {
	name, _ := quoted(table)
	parts := make([]string, len(columns))
	slots := make([]string, len(columns))
	for i, c := range columns {
		parts[i], _ = quoted(c)
		slots[i] = "?"
	}
	_, err := tx.ExecContext(ctx, "INSERT INTO "+name+" ("+strings.Join(parts, ",")+") VALUES ("+strings.Join(slots, ",")+")", values...)
	if err != nil {
		return failed("core row conversion")
	}
	return nil
}

var tagPermission = regexp.MustCompile(`^tag([0-9]+)\.(.+)$`)

func convert(ctx context.Context, tx *sql.Tx, table string, row map[string]any) error {
	if m, ok := mappings[table]; ok {
		values := make([]any, len(m.source))
		for i, c := range m.source {
			values[i] = row[c]
		}
		if err := insert(ctx, tx, m.target, m.columns, values); err != nil {
			return err
		}
	}
	switch table {
	case "flarum_group_permission":
		action, ok := row["permission"].(string)
		if !ok || action == "" {
			return errors.New("native conversion found an empty permission")
		}
		scope := "global"
		var id uint64
		if match := tagPermission.FindStringSubmatch(action); match != nil {
			var err error
			id, err = strconv.ParseUint(match[1], 10, 64)
			if err != nil {
				return errors.New("native conversion found an invalid tag permission")
			}
			scope, action = "tag", match[2]
		}
		return insert(ctx, tx, "permissions", []string{"group_id", "scope", "scope_id", "action"}, []any{row["group_id"], scope, id, action})
	case "flarum_posts":
		var markdown any
		var markdownHash any
		content, _ := row["content"].(string)
		if row["type"] == "comment" && row["content"] != nil {
			text := flarum.DecodeContent(content)
			markdown = text
			hash := sha256.Sum256([]byte(text))
			markdownHash = hex.EncodeToString(hash[:])
		}
		if err := insert(ctx, tx, "posts", []string{"id", "discussion_id", "number", "author_id", "kind", "body_markdown", "created_at", "edited_at", "edited_by", "hidden_at", "hidden_by", "is_private", "is_approved"},
			[]any{row["id"], row["discussion_id"], row["number"], row["user_id"], row["type"], markdown, row["created_at"], row["edited_at"], row["edited_user_id"], row["hidden_at"], row["hidden_user_id"], row["is_private"], row["is_approved"]}); err != nil {
			return err
		}
		sourceHash := sha256.Sum256([]byte(content))
		return insert(ctx, tx, "legacy_post_contents", []string{"post_id", "raw_content", "source_sha256", "markdown_sha256"}, []any{row["id"], row["content"], hex.EncodeToString(sourceHash[:]), markdownHash})
	case "flarum_discussion_user":
		if row["subscription"] != nil {
			if row["subscription"] != "follow" && row["subscription"] != "ignore" {
				return errors.New("native conversion found an unsupported subscription mode")
			}
			return insert(ctx, tx, "discussion_subscriptions", []string{"user_id", "discussion_id", "mode"}, []any{row["user_id"], row["discussion_id"], row["subscription"]})
		}
	}
	return nil
}

func validateCounts(result *Result) error {
	var archived int64
	for _, count := range result.SourceRows {
		archived += count
	}
	if result.NativeRows["legacy_rows"] != archived || result.NativeRows["legacy_tables"] != int64(len(result.SourceRows)) {
		return errors.New("native archive counts differ from source snapshot")
	}
	for source, m := range mappings {
		if result.NativeRows[m.target] != result.SourceRows[source] {
			return errors.New("native converted row counts differ from source snapshot")
		}
	}
	for source, target := range map[string]string{"flarum_posts": "posts", "flarum_group_permission": "permissions"} {
		if result.NativeRows[target] != result.SourceRows[source] {
			return errors.New("native converted row counts differ from source snapshot")
		}
	}
	if result.NativeRows["legacy_post_contents"] != result.SourceRows["flarum_posts"] {
		return errors.New("native post archive count differs")
	}
	return nil
}

func validate(ctx context.Context, tx *sql.Tx) error {
	// Preserve NULL authors, but reject dangling relationships rather than
	// silently discard data or expose private content under a broken ACL.
	queries := []string{
		"SELECT COUNT(*) FROM posts p LEFT JOIN discussions d ON d.id=p.discussion_id WHERE d.id IS NULL",
		"SELECT COUNT(*) FROM posts p LEFT JOIN users u ON u.id=p.author_id WHERE p.author_id IS NOT NULL AND u.id IS NULL",
		"SELECT COUNT(*) FROM discussions d LEFT JOIN users u ON u.id=d.author_id WHERE d.author_id IS NOT NULL AND u.id IS NULL",
		"SELECT COUNT(*) FROM user_groups x LEFT JOIN users u ON u.id=x.user_id LEFT JOIN `groups` g ON g.id=x.group_id WHERE u.id IS NULL OR g.id IS NULL",
		"SELECT COUNT(*) FROM permissions p LEFT JOIN `groups` g ON g.id=p.group_id WHERE g.id IS NULL",
		"SELECT COUNT(*) FROM permissions p LEFT JOIN tags t ON p.scope='tag' AND t.id=p.scope_id WHERE p.scope='tag' AND t.id IS NULL",
		"SELECT COUNT(*) FROM tags t LEFT JOIN tags parent ON parent.id=t.parent_id WHERE t.parent_id IS NOT NULL AND parent.id IS NULL",
		"SELECT COUNT(*) FROM discussion_tags x LEFT JOIN discussions d ON d.id=x.discussion_id LEFT JOIN tags t ON t.id=x.tag_id WHERE d.id IS NULL OR t.id IS NULL",
		"SELECT COUNT(*) FROM post_likes x LEFT JOIN posts p ON p.id=x.post_id LEFT JOIN users u ON u.id=x.user_id WHERE p.id IS NULL OR u.id IS NULL",
		"SELECT COUNT(*) FROM discussion_reads x LEFT JOIN discussions d ON d.id=x.discussion_id LEFT JOIN users u ON u.id=x.user_id WHERE d.id IS NULL OR u.id IS NULL",
		"SELECT COUNT(*) FROM discussion_subscriptions x LEFT JOIN discussions d ON d.id=x.discussion_id LEFT JOIN users u ON u.id=x.user_id WHERE d.id IS NULL OR u.id IS NULL",
		"SELECT COUNT(*) FROM discussions d LEFT JOIN posts p ON p.id=d.first_post_id WHERE d.first_post_id IS NOT NULL AND (p.id IS NULL OR p.discussion_id<>d.id)",
		"SELECT COUNT(*) FROM discussions d LEFT JOIN posts p ON p.id=d.last_post_id WHERE d.last_post_id IS NOT NULL AND (p.id IS NULL OR p.discussion_id<>d.id)",
		"SELECT COUNT(*) FROM posts p JOIN legacy_post_contents l ON l.post_id=p.id WHERE NOT (SHA2(l.raw_content,256)<=>IF(l.raw_content IS NULL,NULL,l.source_sha256)) OR NOT (SHA2(p.body_markdown,256)<=>l.markdown_sha256)",
	}
	for _, query := range queries {
		var invalid int64
		if err := tx.QueryRowContext(ctx, query).Scan(&invalid); err != nil {
			return failed("native relationship/content validation")
		}
		if invalid != 0 {
			return errors.New("native relationship/content validation found invalid records; target unchanged")
		}
	}
	return nil
}
