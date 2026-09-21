-- Migration 0002: frozen exam paper versions.
--
-- Every paper composition is now stored as an immutable version with its own
-- question snapshots (exam_version_questions). Published versions never
-- change; regenerating a paper creates a new version while old versions stay
-- readable. Attempts pin the exact version they took.

CREATE TABLE IF NOT EXISTS exam_versions (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    exam_id BIGINT UNSIGNED NOT NULL,
    version_no INT NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'draft',
    total_score DOUBLE NOT NULL DEFAULT 0,
    duration_minutes INT NOT NULL DEFAULT 60,
    published_at DATETIME(3) NULL,
    created_by BIGINT UNSIGNED DEFAULT 0,
    created_at DATETIME(3) NULL,
    updated_at DATETIME(3) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_exam_version_no (exam_id, version_no),
    KEY idx_version_status (status),
    KEY idx_version_created_by (created_by)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS exam_version_questions (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    version_id BIGINT UNSIGNED NOT NULL,
    exam_id BIGINT UNSIGNED NOT NULL,
    question_id BIGINT UNSIGNED NOT NULL,
    sort_order INT NOT NULL,
    score DOUBLE NOT NULL,
    type VARCHAR(16) NOT NULL,
    content TEXT NOT NULL,
    options TEXT,
    answer TEXT NOT NULL,
    analysis TEXT,
    difficulty VARCHAR(16) NOT NULL DEFAULT '',
    knowledge_point VARCHAR(128) NOT NULL DEFAULT '',
    PRIMARY KEY (id),
    UNIQUE KEY uk_version_sort (version_id, sort_order),
    KEY idx_version_questions_version_id (version_id),
    KEY idx_version_questions_exam_id (exam_id),
    KEY idx_version_questions_question_id (question_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Exams point at the version currently served to new attempts.
ALTER TABLE exams
    ADD COLUMN current_version_id BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER status;

-- Each attempt pins the frozen version it took.
ALTER TABLE exam_attempts
    ADD COLUMN version_id BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER exam_id,
    ADD KEY idx_exam_attempts_version_id (version_id);

-- Questions become soft-deletable ("withdrawn"): withdrawal hides them from
-- the bank but can never touch frozen snapshots.
ALTER TABLE questions
    ADD COLUMN deleted_at DATETIME(3) NULL,
    ADD KEY idx_questions_deleted_at (deleted_at);

-- One-time backfill: turn each legacy exam_questions composition into version
-- 1 of the exam, copying the question bank content as it stands today. The
-- Go server performs the same backfill idempotently at startup.

INSERT INTO exam_versions (exam_id, version_no, status, total_score, duration_minutes, created_by, created_at, updated_at, published_at)
SELECT
    e.id,
    1,
    CASE WHEN e.status IN ('published', 'closed') THEN 'published' ELSE 'draft' END,
    e.total_score,
    e.duration_minutes,
    e.created_by,
    NOW(3),
    NOW(3),
    CASE WHEN e.status IN ('published', 'closed') THEN NOW(3) ELSE NULL END
FROM exams e
WHERE NOT EXISTS (SELECT 1 FROM exam_versions v WHERE v.exam_id = e.id);

INSERT INTO exam_version_questions
    (version_id, exam_id, question_id, sort_order, score, type, content, options, answer, analysis, difficulty, knowledge_point)
SELECT
    v.id,
    e.id,
    q.id,
    eq.sort_order,
    eq.score,
    COALESCE(q.type, ''),
    COALESCE(q.content, '（题目已撤回）'),
    q.options,
    COALESCE(q.answer, ''),
    q.analysis,
    COALESCE(q.difficulty, ''),
    COALESCE(q.knowledge_point, '')
FROM exam_questions eq
JOIN exams e ON e.id = eq.exam_id
JOIN exam_versions v ON v.exam_id = e.id AND v.version_no = 1
LEFT JOIN questions q ON q.id = eq.question_id;

UPDATE exams e
JOIN exam_versions v ON v.exam_id = e.id AND v.status = 'published' AND v.version_no = 1
SET e.current_version_id = v.id;

UPDATE exam_attempts a
JOIN exam_versions v ON v.exam_id = a.exam_id AND v.version_no = 1
SET a.version_id = v.id
WHERE a.version_id = 0 OR a.version_id IS NULL;

-- Legacy table retained for rollback safety; no longer used by application code.
