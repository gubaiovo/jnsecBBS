-- For legacy Flarum installations with the best-answer extension only.
-- Inspect SHOW COLUMNS FROM flarum_discussions LIKE 'best_answer_notified'
-- first. The column must already exist; standalone Go databases do not need it.
-- Sets the default for future inserts without rewriting historical values.
ALTER TABLE flarum_discussions ALTER COLUMN best_answer_notified SET DEFAULT 0;
-- Optional schema rollback (old PHP explicitly writes this field):
-- ALTER TABLE flarum_discussions ALTER COLUMN best_answer_notified DROP DEFAULT;
