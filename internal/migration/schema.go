// Package migration imports a legacy Flarum snapshot into an independent native
// schema. The native schema is a shadow target; the serving application does not
// yet use it. No compatibility views or automatic runtime DDL are installed.
package migration

const schemaVersion = 1

// There are deliberately no automatic cascades. Legacy relationships are checked
// explicitly before commit, and rows that cannot be represented cause rollback.
// Optional extension fields remain recoverable in the complete source archive.
var schemaStatements = []string{
	`CREATE TABLE users (
		id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, username VARCHAR(100), email VARCHAR(150),
		password_hash VARCHAR(255), email_confirmed BOOLEAN, bio TEXT, avatar_path TEXT,
		joined_at DATETIME(6), last_seen_at DATETIME(6), suspended_until DATETIME(6),
		UNIQUE KEY users_username (username), UNIQUE KEY users_email (email)
	)`,
	"CREATE TABLE `groups` (" + `
		id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, name_singular VARCHAR(100) NOT NULL,
		name_plural VARCHAR(100) NOT NULL, color VARCHAR(50), icon VARCHAR(100),
		is_hidden BOOLEAN NOT NULL
	)`,
	`CREATE TABLE user_groups (
		user_id BIGINT UNSIGNED NOT NULL, group_id BIGINT UNSIGNED NOT NULL,
		PRIMARY KEY (user_id,group_id), KEY user_groups_group (group_id,user_id)
	)`,
	`CREATE TABLE permissions (
		group_id BIGINT UNSIGNED NOT NULL, scope VARCHAR(8) NOT NULL,
		scope_id BIGINT UNSIGNED NOT NULL, action VARCHAR(100) NOT NULL,
		PRIMARY KEY (group_id,scope,scope_id,action)
	)`,
	`CREATE TABLE tags (
		id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, name VARCHAR(100) NOT NULL, slug VARCHAR(100) NOT NULL,
		description TEXT, color VARCHAR(50), icon VARCHAR(100), parent_id BIGINT UNSIGNED,
		position INT, is_restricted BOOLEAN NOT NULL, is_hidden BOOLEAN NOT NULL,
		KEY tags_parent (parent_id,position)
	)`,
	`CREATE TABLE discussion_tags (
		discussion_id BIGINT UNSIGNED NOT NULL, tag_id BIGINT UNSIGNED NOT NULL,
		PRIMARY KEY (discussion_id,tag_id), KEY discussion_tags_tag (tag_id,discussion_id)
	)`,
	`CREATE TABLE discussions (
		id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, title VARCHAR(200), slug VARCHAR(255) NOT NULL,
		author_id BIGINT UNSIGNED, created_at DATETIME(6), first_post_id BIGINT UNSIGNED,
		last_post_id BIGINT UNSIGNED, last_posted_at DATETIME(6), last_posted_user_id BIGINT UNSIGNED,
		hidden_at DATETIME(6), hidden_by BIGINT UNSIGNED, is_private BOOLEAN NOT NULL,
		is_approved BOOLEAN NOT NULL, is_locked BOOLEAN NOT NULL, is_sticky BOOLEAN NOT NULL,
		comment_count BIGINT, KEY discussions_recent (last_posted_at,id),
		KEY discussions_author (author_id,created_at,id)
	)`,
	`CREATE TABLE posts (
		id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, discussion_id BIGINT UNSIGNED NOT NULL,
		number BIGINT UNSIGNED, author_id BIGINT UNSIGNED, kind VARCHAR(100), body_markdown LONGTEXT,
		created_at DATETIME(6) NOT NULL, edited_at DATETIME(6), edited_by BIGINT UNSIGNED,
		hidden_at DATETIME(6), hidden_by BIGINT UNSIGNED, is_private BOOLEAN NOT NULL,
		is_approved BOOLEAN NOT NULL, UNIQUE KEY posts_number (discussion_id,number),
		KEY posts_author (author_id,created_at,id)
	)`,
	`CREATE TABLE post_likes (
		post_id BIGINT UNSIGNED NOT NULL, user_id BIGINT UNSIGNED NOT NULL, created_at DATETIME(6) NOT NULL,
		PRIMARY KEY (post_id,user_id), KEY post_likes_user (user_id,post_id)
	)`,
	`CREATE TABLE discussion_reads (
		user_id BIGINT UNSIGNED NOT NULL, discussion_id BIGINT UNSIGNED NOT NULL,
		last_read_at DATETIME(6), last_read_number BIGINT UNSIGNED,
		PRIMARY KEY (user_id,discussion_id)
	)`,
	`CREATE TABLE discussion_subscriptions (
		user_id BIGINT UNSIGNED NOT NULL, discussion_id BIGINT UNSIGNED NOT NULL,
		mode VARCHAR(16) NOT NULL, PRIMARY KEY (user_id,discussion_id)
	)`,
	`CREATE TABLE legacy_post_contents (
		post_id BIGINT UNSIGNED PRIMARY KEY, raw_content LONGBLOB,
		source_sha256 CHAR(64) NOT NULL, markdown_sha256 CHAR(64)
	)`,
	`CREATE TABLE legacy_tables (
		name VARCHAR(64) PRIMARY KEY, create_sql LONGTEXT NOT NULL, columns_json LONGTEXT NOT NULL,
		row_count BIGINT UNSIGNED NOT NULL
	)`,
	`CREATE TABLE legacy_rows (
		source_table VARCHAR(64) NOT NULL, row_ordinal BIGINT UNSIGNED NOT NULL,
		values_json LONGTEXT NOT NULL, PRIMARY KEY (source_table,row_ordinal)
	)`,
	`CREATE TABLE sessions (
		token_hash BINARY(32) PRIMARY KEY, user_id BIGINT UNSIGNED NOT NULL, expires_at DATETIME(6) NOT NULL,
		KEY sessions_user (user_id), KEY sessions_expiry (expires_at)
	)`,
	`CREATE TABLE user_profiles (
		user_id BIGINT UNSIGNED PRIMARY KEY, real_name VARCHAR(100) NOT NULL DEFAULT '',
		blog_url VARCHAR(255) NOT NULL DEFAULT '', updated_at DATETIME(6) NOT NULL
	)`,
	`CREATE TABLE migration_state (
		id INT PRIMARY KEY, schema_version INT NOT NULL, source_uuid VARCHAR(64) NOT NULL,
		source_database VARCHAR(64) NOT NULL, source_schema_hash CHAR(64) NOT NULL,
		target_schema_hash CHAR(64) NOT NULL, completed_at DATETIME(6), counts_json LONGTEXT
	)`,
}

var managedTables = []string{
	"users", "groups", "user_groups", "permissions", "tags", "discussion_tags", "discussions",
	"posts", "post_likes", "discussion_reads", "discussion_subscriptions", "legacy_post_contents",
	"legacy_tables", "legacy_rows",
}

var localTables = []string{"sessions", "user_profiles", "migration_state"}
