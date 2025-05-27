-- Down Migration: Drop prompt_version column from analysis_results table
ALTER TABLE analysis_results
DROP COLUMN prompt_version;