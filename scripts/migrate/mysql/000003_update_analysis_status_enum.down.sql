-- Down Migration: Revert ENUM definition for analysis_status
ALTER TABLE videos
MODIFY COLUMN analysis_status ENUM(
    'pending',
    'processing',
    'completed',
    'failed'
) NOT NULL DEFAULT 'pending';