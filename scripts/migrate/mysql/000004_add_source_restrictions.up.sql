-- Up Migration: Add restrictions and tran_restrictions columns to videos table
 
ALTER TABLE videos
ADD COLUMN restrictions VARCHAR(255) NULL DEFAULT NULL COMMENT '來源 (來自 TXT 文件分析)' AFTER location,
ADD COLUMN tran_restrictions VARCHAR(255) NULL DEFAULT NULL COMMENT '來源翻譯 (來自 TXT 文件分析)' AFTER restrictions; 