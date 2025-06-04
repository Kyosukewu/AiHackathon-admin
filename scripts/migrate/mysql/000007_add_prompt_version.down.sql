-- Down Migration: Remove prompt_version column from videos table
 
ALTER TABLE videos
DROP COLUMN prompt_version; 