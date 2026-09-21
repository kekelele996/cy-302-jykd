-- Canonical schema for the online exam platform (frozen paper versions).
-- The Go server also runs GORM AutoMigrate at startup; this file is used by
-- docker-compose MySQL initialization on a fresh data volume.

CREATE TABLE IF NOT EXISTS users (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    username VARCHAR(64) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    name VARCHAR(64) DEFAULT '',
    role VARCHAR(16) NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'active',
    created_at DATETIME(3) NULL,
    updated_at DATETIME(3) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY idx_users_username (username),
    KEY idx_users_role (role)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS questions (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    type VARCHAR(16) NOT NULL,
    content TEXT NOT NULL,
    options TEXT,
    answer TEXT NOT NULL,
    analysis TEXT,
    difficulty VARCHAR(16) NOT NULL,
    knowledge_point VARCHAR(128) NOT NULL,
    score DOUBLE NOT NULL DEFAULT 1,
    created_by BIGINT UNSIGNED DEFAULT 0,
    created_at DATETIME(3) NULL,
    updated_at DATETIME(3) NULL,
    deleted_at DATETIME(3) NULL,
    PRIMARY KEY (id),
    KEY idx_questions_type (type),
    KEY idx_questions_difficulty (difficulty),
    KEY idx_questions_knowledge_point (knowledge_point),
    KEY idx_questions_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS exams (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    title VARCHAR(128) NOT NULL,
    description TEXT,
    total_score DOUBLE NOT NULL DEFAULT 0,
    duration_minutes INT NOT NULL DEFAULT 60,
    start_time DATETIME(3) NULL,
    end_time DATETIME(3) NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'draft',
    current_version_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
    created_by BIGINT UNSIGNED DEFAULT 0,
    created_at DATETIME(3) NULL,
    updated_at DATETIME(3) NULL,
    PRIMARY KEY (id),
    KEY idx_exams_status (status),
    KEY idx_exams_created_by (created_by)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Immutable paper versions: draft -> published -> archived (per exam, one
-- published at a time). Version rows and their snapshots are never mutated
-- after publication, so in-progress exams and historical results stay stable.
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

-- Frozen per-version question snapshots: stem, options, score and standard
-- answer captured at composition time, immune to later edits/withdrawal.
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

CREATE TABLE IF NOT EXISTS exam_attempts (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    exam_id BIGINT UNSIGNED NOT NULL,
    version_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
    student_id BIGINT UNSIGNED NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'in_progress',
    started_at DATETIME(3) NULL,
    submitted_at DATETIME(3) NULL,
    deadline DATETIME(3) NULL,
    question_order TEXT,
    option_order TEXT,
    objective_score DOUBLE NOT NULL DEFAULT 0,
    total_score DOUBLE NOT NULL DEFAULT 0,
    created_at DATETIME(3) NULL,
    updated_at DATETIME(3) NULL,
    PRIMARY KEY (id),
    KEY idx_exam_attempts_exam_id (exam_id),
    KEY idx_exam_attempts_version_id (version_id),
    KEY idx_exam_attempts_student_id (student_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS answers (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    attempt_id BIGINT UNSIGNED NOT NULL,
    exam_question_id BIGINT UNSIGNED NOT NULL,
    question_id BIGINT UNSIGNED NOT NULL,
    answer_text TEXT,
    is_correct TINYINT(1) NULL,
    score DOUBLE NOT NULL DEFAULT 0,
    marked TINYINT(1) NOT NULL DEFAULT 0,
    graded_by BIGINT UNSIGNED DEFAULT 0,
    created_at DATETIME(3) NULL,
    updated_at DATETIME(3) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY idx_attempt_question (attempt_id, exam_question_id),
    KEY idx_answers_question_id (question_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS wrong_questions (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    student_id BIGINT UNSIGNED NOT NULL,
    question_id BIGINT UNSIGNED NOT NULL,
    knowledge_point VARCHAR(128) DEFAULT '',
    wrong_count INT NOT NULL DEFAULT 1,
    last_wrong_at DATETIME(3) NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'unresolved',
    created_at DATETIME(3) NULL,
    updated_at DATETIME(3) NULL,
    PRIMARY KEY (id),
    KEY idx_wrong_questions_student_id (student_id),
    KEY idx_wrong_questions_question_id (question_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
