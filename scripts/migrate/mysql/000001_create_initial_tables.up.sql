            -- Up Migration: Create final schema for videos and analysis_results tables

            CREATE TABLE videos (
                id BIGINT AUTO_INCREMENT PRIMARY KEY,
                source_name VARCHAR(50) NOT NULL,
                source_id VARCHAR(255) NOT NULL,
                nas_path VARCHAR(1024) NOT NULL,
                title VARCHAR(512) NULL,
                fetched_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
                published_at TIMESTAMP NULL DEFAULT NULL COMMENT '發布時間 (文件提供)',
                duration_secs INT NULL DEFAULT NULL COMMENT '長度 (秒) (文件提供)',
                shotlist_content TEXT NULL DEFAULT NULL COMMENT 'SHOTLIST內容 (文件提供)',
                view_link VARCHAR(2048) NULL DEFAULT NULL COMMENT '一鍵看帶連結',
                analysis_status ENUM('pending', 'processing', 'completed', 'failed') NOT NULL DEFAULT 'pending',
                analyzed_at TIMESTAMP NULL,
                source_metadata JSON NULL,
                UNIQUE KEY idx_source (source_name, source_id),
                INDEX idx_nas_path (nas_path(255)),
                INDEX idx_analysis_status (analysis_status)
            ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

            CREATE TABLE analysis_results (
                video_id BIGINT PRIMARY KEY,
                transcript TEXT NULL DEFAULT NULL, -- 改為 TEXT 以匹配 JsonNullString 的底層 String
                translation TEXT NULL DEFAULT NULL, -- 改為 TEXT
                short_summary TEXT NULL DEFAULT NULL COMMENT '短摘要 (Gemini分析)',
                bulleted_summary TEXT NULL DEFAULT NULL COMMENT '列點摘要 (Gemini分析)',
                bites JSON NULL DEFAULT NULL COMMENT 'BITE (講者：「說了什麼」) (Gemini分析)',
                mentioned_locations JSON NULL DEFAULT NULL COMMENT '地點 (文稿內) (Gemini分析)',
                importance_score JSON NULL DEFAULT NULL COMMENT '重要性評分 (Gemini分析)',
                material_type VARCHAR(100) NULL DEFAULT NULL COMMENT '素材類型 (Gemini分析)',
                related_news JSON NULL DEFAULT NULL COMMENT '相關新聞 (Gemini分析)',
                visual_description TEXT NULL DEFAULT NULL, -- 改為 TEXT
                topics JSON NULL DEFAULT NULL,
                keywords JSON NULL DEFAULT NULL,
                error_message TEXT NULL DEFAULT NULL,    -- 改為 TEXT
                prompt_version VARCHAR(50) NULL DEFAULT NULL COMMENT '用於分析的 Prompt 版本',
                created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
                updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
                FOREIGN KEY (video_id) REFERENCES videos(id) ON DELETE CASCADE
            ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
            