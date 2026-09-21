-- Migration 0002: immutable paper version freezing.
--
-- Published exams freeze the exact question bank content (stem, options,
-- per-question score and reference answer) into paper_versions /
-- paper_version_questions at publish time. Later question edits or withdrawals
-- never mutate frozen versions. Re-generating a paper creates a new version
-- row; old versions stay readable. Attempts bind to one version for life.
--
-- The Go server also runs GORM AutoMigrate at startup and backfills a v1
-- snapshot for exams/attempts that predate this migration, so this file can be
-- applied manually but is normally redundant.

CREATE TABLE IF NOT EXISTS paper_versions (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    exam_id BIGINT UNSIGNED NOT NULL,
    version_no INT NOT NULL,
    title VARCHAR(128) NOT NULL,
    total_score DOUBLE NOT NULL DEFAULT 0,
    snapshot_at DATETIME(3) NOT NULL,
    created_by BIGINT UNSIGNED DEFAULT 0,
    created_at DATETIME(3) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_paper_version_no (exam_id, version_no)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS paper_version_questions (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    version_id BIGINT UNSIGNED NOT NULL,
    question_id BIGINT UNSIGNED NOT NULL,
    type VARCHAR(16) NOT NULL,
    content TEXT NOT NULL,
    options TEXT,
    answer TEXT NOT NULL,
    analysis TEXT,
    difficulty VARCHAR(16) NOT NULL,
    knowledge_point VARCHAR(128) NOT NULL,
    score DOUBLE NOT NULL,
    sort_order INT NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_version_question_sort (version_id, sort_order),
    KEY idx_pvq_version_id (version_id),
    KEY idx_pvq_question_id (question_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

ALTER TABLE exams
    ADD COLUMN current_version_id BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER status,
    ADD COLUMN revision INT UNSIGNED NOT NULL DEFAULT 0 AFTER current_version_id,
    ADD KEY idx_exams_current_version (current_version_id);

ALTER TABLE exam_attempts
    ADD COLUMN paper_version_id BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER exam_id,
    ADD KEY idx_exam_attempts_paper_version (paper_version_id);
