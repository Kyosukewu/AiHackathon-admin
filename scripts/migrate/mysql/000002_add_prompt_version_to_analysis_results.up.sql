-- Up Migration: Add prompt_version column to analysis_results table
ALTER TABLE analysis_results
ADD COLUMN prompt_version VARCHAR(50) NULL DEFAULT NULL COMMENT '用於分析的 Prompt 版本' AFTER error_message;