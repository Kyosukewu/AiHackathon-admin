-- Up Migration: Update ENUM definition for analysis_status in videos table
ALTER TABLE videos
MODIFY COLUMN analysis_status ENUM(
    'pending',
    'metadata_extracting',
    'metadata_extracted',
    'txt_analysis_failed',
    'processing',
    'video_analysis_failed',
    'completed',
    'failed' -- 保留舊的 'failed' 以防萬一，或者可以考慮移除它如果 'txt_analysis_failed' 和 'video_analysis_failed' 足夠
) NOT NULL DEFAULT 'pending';