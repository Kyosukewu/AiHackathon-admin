-- Up Migration: Add prompt_version column to videos table
 
ALTER TABLE videos
ADD COLUMN prompt_version VARCHAR(50) NULL DEFAULT NULL COMMENT '文本分析使用的 Prompt 版本' AFTER tran_restrictions; 