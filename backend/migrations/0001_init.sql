-- Migration 0001: initial schema for the online exam platform.
-- The Go server also runs GORM AutoMigrate at startup, so this file documents
-- the canonical schema and can be applied manually if needed.

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
    PRIMARY KEY (id),
    KEY idx_questions_type (type),
    KEY idx_questions_difficulty (difficulty),
    KEY idx_questions_knowledge_point (knowledge_point)
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
    created_by BIGINT UNSIGNED DEFAULT 0,
    created_at DATETIME(3) NULL,
    updated_at DATETIME(3) NULL,
    PRIMARY KEY (id),
    KEY idx_exams_status (status),
    KEY idx_exams_created_by (created_by)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS exam_questions (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    exam_id BIGINT UNSIGNED NOT NULL,
    question_id BIGINT UNSIGNED NOT NULL,
    score DOUBLE NOT NULL,
    sort_order INT NOT NULL,
    PRIMARY KEY (id),
    KEY idx_exam_questions_exam_id (exam_id),
    KEY idx_exam_questions_question_id (question_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS exam_attempts (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    exam_id BIGINT UNSIGNED NOT NULL,
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
