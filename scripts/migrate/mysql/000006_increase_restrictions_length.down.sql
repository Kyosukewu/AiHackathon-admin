-- Down Migration: Revert restrictions and tran_restrictions columns length
 
ALTER TABLE videos
MODIFY COLUMN restrictions VARCHAR(255) NULL DEFAULT NULL COMMENT '來源 (來自 TXT 文件分析)',
MODIFY COLUMN tran_restrictions VARCHAR(255) NULL DEFAULT NULL COMMENT '來源翻譯 (來自 TXT 文件分析)'; 