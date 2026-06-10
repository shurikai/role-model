-- Drop indexes first (implicit with table drops, but explicit is cleaner)
-- Drop tables in reverse dependency order

DROP TABLE IF EXISTS contribution_feedback;
DROP TABLE IF EXISTS resume_versions;
DROP TABLE IF EXISTS applications;

DROP TABLE IF EXISTS project_tags;
DROP TABLE IF EXISTS project_contributions;
DROP TABLE IF EXISTS projects;

DROP TABLE IF EXISTS credential_tags;
DROP TABLE IF EXISTS education_tags;
DROP TABLE IF EXISTS contribution_tags;

DROP TABLE IF EXISTS tags;
DROP TABLE IF EXISTS tag_categories;

DROP TABLE IF EXISTS credentials;
DROP TABLE IF EXISTS education;
DROP TABLE IF EXISTS contributions;
DROP TABLE IF EXISTS positions;
DROP TABLE IF EXISTS employers;
DROP TABLE IF EXISTS users;
