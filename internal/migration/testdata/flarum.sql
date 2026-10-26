CREATE TABLE flarum_users (
 id INT UNSIGNED PRIMARY KEY, username VARCHAR(100), email VARCHAR(150), password VARCHAR(100),
 is_email_confirmed BOOLEAN, bio TEXT, avatar_url VARCHAR(100), joined_at DATETIME(6),
 last_seen_at DATETIME(6), suspended_until DATETIME(6), preferences BLOB
) ENGINE=InnoDB;
CREATE TABLE flarum_groups (
 id INT UNSIGNED PRIMARY KEY, name_singular VARCHAR(100), name_plural VARCHAR(100),
 color VARCHAR(50), icon VARCHAR(100), is_hidden BOOLEAN
) ENGINE=InnoDB;
CREATE TABLE flarum_group_user (user_id INT UNSIGNED,group_id INT UNSIGNED,PRIMARY KEY(user_id,group_id)) ENGINE=InnoDB;
CREATE TABLE flarum_group_permission (group_id INT UNSIGNED,permission VARCHAR(100),PRIMARY KEY(group_id,permission)) ENGINE=InnoDB;
CREATE TABLE flarum_tags (
 id INT UNSIGNED PRIMARY KEY,name VARCHAR(100),slug VARCHAR(100),description TEXT,color VARCHAR(50),
 icon VARCHAR(100),parent_id INT UNSIGNED,position INT,is_restricted BOOLEAN,is_hidden BOOLEAN
) ENGINE=InnoDB;
CREATE TABLE flarum_discussions (
 id INT UNSIGNED PRIMARY KEY,title VARCHAR(200),slug VARCHAR(255),user_id INT UNSIGNED,created_at DATETIME(6),
 first_post_id INT UNSIGNED,last_post_id INT UNSIGNED,last_posted_at DATETIME(6),last_posted_user_id INT UNSIGNED,
 hidden_at DATETIME(6),hidden_user_id INT UNSIGNED,is_private BOOLEAN,is_approved BOOLEAN,
 is_locked BOOLEAN,is_sticky BOOLEAN,comment_count BIGINT
) ENGINE=InnoDB;
CREATE TABLE flarum_discussion_tag (discussion_id INT UNSIGNED,tag_id INT UNSIGNED,PRIMARY KEY(discussion_id,tag_id)) ENGINE=InnoDB;
CREATE TABLE flarum_posts (
 id INT UNSIGNED PRIMARY KEY,discussion_id INT UNSIGNED,number INT UNSIGNED,user_id INT UNSIGNED,
 type VARCHAR(100),content MEDIUMTEXT,created_at DATETIME(6),edited_at DATETIME(6),edited_user_id INT UNSIGNED,
 hidden_at DATETIME(6),hidden_user_id INT UNSIGNED,is_private BOOLEAN,is_approved BOOLEAN
) ENGINE=InnoDB;
CREATE TABLE flarum_post_likes (post_id INT UNSIGNED,user_id INT UNSIGNED,created_at DATETIME(6),PRIMARY KEY(post_id,user_id)) ENGINE=InnoDB;
CREATE TABLE flarum_discussion_user (
 user_id INT UNSIGNED,discussion_id INT UNSIGNED,last_read_at DATETIME(6),last_read_post_number INT UNSIGNED,
 subscription ENUM('follow','ignore'),PRIMARY KEY(user_id,discussion_id)
) ENGINE=InnoDB;
CREATE TABLE unknown_extension (
 id INT UNSIGNED PRIMARY KEY,payload BLOB,nullable TEXT,`empty` TEXT,
 exact_number DECIMAL(30,10),at DATETIME(6),zero_at DATETIME
) ENGINE=InnoDB;
INSERT INTO flarum_users VALUES(1,'Alice','a@example.test','$2y$10$unchanged',1,'中文签名','avatar.png','2026-01-02 03:04:05.123456',NULL,NULL,UNHEX('00FF5C'));
INSERT INTO flarum_groups VALUES(1,'Admin','Admins','#fff','icon',0);
INSERT INTO flarum_group_user VALUES(1,1);
INSERT INTO flarum_tags VALUES(10,'版块','board','描述','#fff',NULL,NULL,0,1,0);
INSERT INTO flarum_group_permission VALUES(1,'viewForum'),(1,'tag10.discussion.reply');
INSERT INTO flarum_discussions VALUES(1,'历史主题','history',1,'2026-01-02 03:04:05.123456',1,1,'2026-01-02 03:04:05.123456',1,NULL,NULL,0,1,0,1,2);
INSERT INTO flarum_discussion_tag VALUES(1,10);
INSERT INTO flarum_posts VALUES
 (1,1,1,1,'comment','<r><B><s>**</s>中文 &amp; Markdown<e>**</e></B></r>','2026-01-02 03:04:05.123456',NULL,NULL,NULL,NULL,0,1),
 (2,1,2,NULL,'comment','{"text":"JSON 正文"}','2026-01-02 04:04:05.123456',NULL,NULL,NULL,NULL,0,1),
 (3,1,NULL,1,'discussionRenamed','["old","new"]','2026-01-02 05:04:05.123456',NULL,NULL,NULL,NULL,0,1);
INSERT INTO flarum_post_likes VALUES(1,1,'2026-01-02 03:04:05.123456');
INSERT INTO flarum_discussion_user VALUES(1,1,'2026-01-02 03:04:05.123456',2,'follow');
INSERT INTO unknown_extension VALUES(1,UNHEX('00FF5C270A'),NULL,'',12345678901234567890.1234567890,'2026-01-02 03:04:05.123456','0000-00-00 00:00:00');
