-- Down Migration: Drop restrictions and tran_restrictions columns from videos table
 
ALTER TABLE videos
DROP COLUMN IF EXISTS tran_restrictions,
DROP COLUMN IF EXISTS restrictions; 