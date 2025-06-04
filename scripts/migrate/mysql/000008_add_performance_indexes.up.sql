-- Up Migration: Add performance optimization indexes

-- 添加發布時間索引
ALTER TABLE videos
ADD INDEX idx_published_at (published_at);

-- 添加分析狀態和發布時間的複合索引
ALTER TABLE videos
ADD INDEX idx_analysis_status_published_at (analysis_status, published_at);

-- 添加獲取時間索引
ALTER TABLE videos
ADD INDEX idx_fetched_at (fetched_at);

-- 添加標題索引（用於搜尋）
ALTER TABLE videos
ADD FULLTEXT INDEX idx_title_shotlist (title, shotlist_content); 