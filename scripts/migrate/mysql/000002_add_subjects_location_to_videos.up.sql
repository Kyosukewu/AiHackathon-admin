    -- Up Migration: Add subjects and location columns to videos table

    ALTER TABLE videos
    ADD COLUMN subjects JSON NULL DEFAULT NULL COMMENT '分類 (來自 TXT 文件分析)' AFTER view_link,
    ADD COLUMN location VARCHAR(255) NULL DEFAULT NULL COMMENT '地點 (來自 TXT 文件分析)' AFTER subjects;
    