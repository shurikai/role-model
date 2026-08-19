-- Drops the column and, with it, the seeded competency vocabulary and any
-- aliases curated since. There is nowhere else to keep them, so this is
-- lossy by nature rather than by oversight.

ALTER TABLE tag_categories DROP COLUMN aliases;
