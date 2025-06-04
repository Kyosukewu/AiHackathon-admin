-- Down Migration: Remove performance optimization indexes

-- 移除標題全文索引
ALTER TABLE videos
DROP INDEX IF EXISTS idx_title_shotlist;

-- 移除獲取時間索引
ALTER TABLE videos
DROP INDEX IF EXISTS idx_fetched_at;

-- 移除分析狀態和發布時間的複合索引
ALTER TABLE videos
DROP INDEX IF EXISTS idx_analysis_status_published_at;

-- 移除發布時間索引
ALTER TABLE videos
DROP INDEX IF EXISTS idx_published_at; 