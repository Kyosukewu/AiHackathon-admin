-- Down Migration: Rename restrictions and tran_restrictions columns back to source and source_translation
 
ALTER TABLE videos
CHANGE COLUMN restrictions source VARCHAR(255) NULL DEFAULT NULL COMMENT '來源 (來自 TXT 文件分析)',
CHANGE COLUMN tran_restrictions source_translation VARCHAR(255) NULL DEFAULT NULL COMMENT '來源翻譯 (來自 TXT 文件分析)'; 