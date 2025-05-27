-- Up Migration: Create videos and analysis_results tables

CREATE TABLE IF NOT EXISTS videos (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    source_name VARCHAR(50) NOT NULL,
    source_id VARCHAR(255) NOT NULL,
    nas_path VARCHAR(1024) NOT NULL,
    title VARCHAR(512) NULL,
    fetched_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    analysis_status ENUM('pending', 'processing', 'completed', 'failed') NOT NULL DEFAULT 'pending',
    analyzed_at TIMESTAMP NULL,
    source_metadata JSON NULL,
    UNIQUE KEY idx_source (source_name, source_id),
    INDEX idx_nas_path (nas_path(255)), -- 建議為 nas_path 加索引，指定長度
    INDEX idx_analysis_status (analysis_status) -- 為分析狀態加索引
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS analysis_results (
    video_id BIGINT PRIMARY KEY,
    transcript LONGTEXT NULL,
    translation LONGTEXT NULL,
    summary TEXT NULL,
    visual_description TEXT NULL,
    topics JSON NULL,
    keywords JSON NULL,
    error_message TEXT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (video_id) REFERENCES videos(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;