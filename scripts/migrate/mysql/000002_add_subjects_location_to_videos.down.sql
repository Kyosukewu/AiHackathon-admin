    -- Down Migration: Drop subjects and location columns from videos table

    ALTER TABLE videos
    DROP COLUMN IF EXISTS location,
    DROP COLUMN IF EXISTS subjects;
    