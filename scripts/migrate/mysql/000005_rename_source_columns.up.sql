-- Up Migration: Rename source and source_translation columns to restrictions and tran_restrictions
 
ALTER TABLE videos
CHANGE COLUMN source restrictions VARCHAR(255) NULL DEFAULT NULL COMMENT '來源 (來自 TXT 文件分析)',
CHANGE COLUMN source_translation tran_restrictions VARCHAR(255) NULL DEFAULT NULL COMMENT '來源翻譯 (來自 TXT 文件分析)'; 