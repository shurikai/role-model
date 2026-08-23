-- Sample data: clinical career, user, vocabularies, employers, positions, tags
--
-- FICTIONAL. Priya Raghunathan does not exist and neither does any employer
-- here. See database/sample-clinical/README.md.
--
-- PRODUCED BY THE INTAKE, not written by hand. A 640-word career narrative was
-- pasted into an account holding nothing but a user row and the shipped neutral
-- vocabularies; stage0_career_extraction.tmpl read it, internal/intake planned
-- and staged 25 drafts, and the resolver wrote them in dependency order. The
-- README records what came out of that and what a human added afterwards.
--
-- UUIDs all begin with '5b' so they cannot collide with the software sample
-- ('5a', '5c', '57'...) or a real seed set loaded into the same database.

BEGIN;

-- ============================================================
-- users
-- ============================================================
INSERT INTO users (id, email, password_hash, full_name, phone, location, linkedin_url, headline) VALUES
  ('5b000000-0000-0000-0000-000000000001', 'priya@example.com', '$2a$10$fxDaKBd7DFo7pgWitj9BKuR32oHwJVt.jCpZgOfYeGu31HOttzrKa', 'Priya Raghunathan, RN, MSN', '(555) 555-0188', 'Providence, RI', 'https://www.linkedin.com/in/example-priya-raghunathan', 'Ambulatory quality improvement -- referral flow, care coordination, and the systems clinics actually run on.')

ON CONFLICT (id) DO UPDATE SET
  email = EXCLUDED.email,
  password_hash = EXCLUDED.password_hash,
  full_name = EXCLUDED.full_name,
  phone = EXCLUDED.phone,
  location = EXCLUDED.location,
  linkedin_url = EXCLUDED.linkedin_url,
  headline = EXCLUDED.headline,
  updated_at = now();

-- ============================================================
-- career_levels
-- ============================================================
INSERT INTO career_levels (id, user_id, value, label, rank, aliases, length_budget, framing_guidance, is_fallback, source, sort_order) VALUES
  ('5b100000-0000-0000-0000-000000000001', '5b000000-0000-0000-0000-000000000001', 'new grad', 'New Graduate', 1, ARRAY['graduate nurse', 'new graduate', 'entry level'], 'Target 1 page (~8-10 bullets total across ALL positions and projects combined).', 'Lead with the concrete work and the outcome it produced.

Where the source material genuinely supports ownership or leadership scope,
say so — but do not reach for it. A bullet claiming scope the contribution data
does not support reads as padding, and costs more credibility than the framing
gains.', FALSE, 'inferred', 1),
  ('5b100000-0000-0000-0000-000000000002', '5b000000-0000-0000-0000-000000000001', 'staff', 'Staff Nurse', 2, ARRAY['staff level', 'staff rn', 'clinical nurse'], 'Target 1-2 pages (~12-15 bullets total across ALL positions and projects combined).', 'Lead with the concrete work and the outcome it produced.

Where the source material genuinely supports ownership or leadership scope,
say so — but do not reach for it. A bullet claiming scope the contribution data
does not support reads as padding, and costs more credibility than the framing
gains.', TRUE, 'inferred', 2),
  ('5b100000-0000-0000-0000-000000000003', '5b000000-0000-0000-0000-000000000001', 'coordinator', 'Coordinator', 3, ARRAY['care coordinator', 'coordinator level'], 'Target 1-2 pages (~12-15 bullets total across ALL positions and projects combined).', 'Lead with the concrete work and the outcome it produced.

Where the source material genuinely supports ownership or leadership scope,
say so — but do not reach for it. A bullet claiming scope the contribution data
does not support reads as padding, and costs more credibility than the framing
gains.', FALSE, 'inferred', 3),
  ('5b100000-0000-0000-0000-000000000004', '5b000000-0000-0000-0000-000000000001', 'charge', 'Charge Nurse', 4, ARRAY['charge', 'charge/supervisory level', 'lead nurse'], 'Target 2 pages (~15-18 bullets total across ALL positions and projects combined).', 'A reader at this level is scanning for scope and accountability, not
implementation detail alone.

On the 2-3 most JD-relevant positions, open each bullet with what was owned,
decided, or changed at the system or team level, then land the supporting
evidence — the metric, the scale, the caseload — in the same sentence.

Two hard limits on this:
  - NEVER trade the evidence for the framing. A bullet that claims ownership
    and drops the number is weaker than one that only reports the number.
  - NEVER manufacture scope the source material does not support. "Owned",
    "led", "set direction for" are factual claims and need backing in the
    contribution data like any other.', FALSE, 'inferred', 4),
  ('5b100000-0000-0000-0000-000000000005', '5b000000-0000-0000-0000-000000000001', 'manager', 'Nurse Manager', 5, ARRAY['nurse manager', 'clinical manager'], 'Target 2 pages (~15-18 bullets total across ALL positions and projects combined).', 'A reader at this level is scanning for scope and accountability, not
implementation detail alone.

On the 2-3 most JD-relevant positions, open each bullet with what was owned,
decided, or changed at the system or team level, then land the supporting
evidence — the metric, the scale, the caseload — in the same sentence.

Two hard limits on this:
  - NEVER trade the evidence for the framing. A bullet that claims ownership
    and drops the number is weaker than one that only reports the number.
  - NEVER manufacture scope the source material does not support. "Owned",
    "led", "set direction for" are factual claims and need backing in the
    contribution data like any other.', FALSE, 'inferred', 5),
  ('5b100000-0000-0000-0000-000000000006', '5b000000-0000-0000-0000-000000000001', 'director', 'Director of Nursing', 6, ARRAY['director', 'cno', 'chief nursing officer'], 'Target 2 pages (~15-18 bullets total across ALL positions and projects combined).', 'A reader at this level is scanning for scope and accountability, not
implementation detail alone.

On the 2-3 most JD-relevant positions, open each bullet with what was owned,
decided, or changed at the system or team level, then land the supporting
evidence — the metric, the scale, the caseload — in the same sentence.

Two hard limits on this:
  - NEVER trade the evidence for the framing. A bullet that claims ownership
    and drops the number is weaker than one that only reports the number.
  - NEVER manufacture scope the source material does not support. "Owned",
    "led", "set direction for" are factual claims and need backing in the
    contribution data like any other.', FALSE, 'inferred', 6)

ON CONFLICT (user_id, value) DO UPDATE SET
  id = EXCLUDED.id,
  label = EXCLUDED.label,
  rank = EXCLUDED.rank,
  aliases = EXCLUDED.aliases,
  length_budget = EXCLUDED.length_budget,
  framing_guidance = EXCLUDED.framing_guidance,
  is_fallback = EXCLUDED.is_fallback,
  source = EXCLUDED.source,
  sort_order = EXCLUDED.sort_order,
  updated_at = now();

-- ============================================================
-- proficiency_levels
-- ============================================================
INSERT INTO proficiency_levels (id, user_id, value, label, rank, aliases, source, sort_order) VALUES
  ('38dcad52-e496-4834-9b28-12bd7e47c977', '5b000000-0000-0000-0000-000000000001', 'novice', 'Novice', 1, ARRAY['beginner', 'familiarity', 'exposure', 'awareness'], 'default', 1),
  ('ab9bf734-fdaf-4e9f-8050-9c7b15b527b1', '5b000000-0000-0000-0000-000000000001', 'proficient', 'Proficient', 2, ARRAY['working knowledge', 'solid', 'competent'], 'default', 2),
  ('364ad5c3-4583-41aa-bfa0-adb831436b5e', '5b000000-0000-0000-0000-000000000001', 'expert', 'Expert', 3, ARRAY['deep expertise', 'advanced', 'mastery', 'authority'], 'default', 3)

ON CONFLICT (user_id, value) DO UPDATE SET
  id = EXCLUDED.id,
  label = EXCLUDED.label,
  rank = EXCLUDED.rank,
  aliases = EXCLUDED.aliases,
  source = EXCLUDED.source,
  sort_order = EXCLUDED.sort_order,
  updated_at = now();

-- ============================================================
-- resume_sections
-- ============================================================
INSERT INTO resume_sections (id, user_id, key, heading, sort_order, hidden, source) VALUES
  ('bf1ea518-6726-482b-8a5d-f7959994bbda', '5b000000-0000-0000-0000-000000000001', 'summary', 'SUMMARY', 1, FALSE, 'default'),
  ('24d351f2-9880-4a29-b579-e527a7cdd198', '5b000000-0000-0000-0000-000000000001', 'skills', 'SKILLS', 2, FALSE, 'default'),
  ('e35f014d-2fc3-4210-b35e-85b33e4d2ab0', '5b000000-0000-0000-0000-000000000001', 'experience', 'EXPERIENCE', 3, FALSE, 'default'),
  ('c88cf588-4eaf-454c-b3ff-e4626bc57398', '5b000000-0000-0000-0000-000000000001', 'projects', 'PROJECTS', 4, FALSE, 'default'),
  ('354f56af-4df0-4949-8781-cc2ae6609c3e', '5b000000-0000-0000-0000-000000000001', 'education', 'EDUCATION', 5, FALSE, 'default'),
  ('56af4e47-a258-4909-9b78-17ffbe3bf7c4', '5b000000-0000-0000-0000-000000000001', 'credentials', 'CREDENTIALS', 6, FALSE, 'default')

ON CONFLICT (user_id, key) DO UPDATE SET
  id = EXCLUDED.id,
  heading = EXCLUDED.heading,
  sort_order = EXCLUDED.sort_order,
  hidden = EXCLUDED.hidden,
  source = EXCLUDED.source,
  updated_at = now();

-- ============================================================
-- employers
-- ============================================================
INSERT INTO employers (id, user_id, name, industry, notes) VALUES
  ('891790d4-7896-4744-b6e6-8e85ccfb9290', '5b000000-0000-0000-0000-000000000001', 'Coastal Vascular Associates', 'Outpatient vascular specialty practice', NULL),
  ('c6f975b2-5cca-48b9-9641-99664286bea8', '5b000000-0000-0000-0000-000000000001', 'County Health Network', 'Hospital system with ambulatory network', NULL),
  ('9d5827c2-4c0c-4ea9-9561-7e6272e7f0f9', '5b000000-0000-0000-0000-000000000001', 'St. Brigid Regional Hospital', 'Acute care hospital', NULL)

ON CONFLICT (id) DO UPDATE SET
  user_id = EXCLUDED.user_id,
  name = EXCLUDED.name,
  industry = EXCLUDED.industry,
  notes = EXCLUDED.notes,
  updated_at = now();

-- ============================================================
-- positions
-- ============================================================
INSERT INTO positions (id, user_id, employer_id, title, industry_level, industry_role, level_rationale, started_on, ended_on, context_narrative, sort_order, location) VALUES
  ('9bef3db7-138f-48c1-a951-66ba338da48b', '5b000000-0000-0000-0000-000000000001', '9d5827c2-4c0c-4ea9-9561-7e6272e7f0f9', 'Staff Nurse', 'staff', 'Staff Nurse', NULL, '2011-06-01', '2015-01-01', 'Med-surg floor, 32 beds, nights.', 0, 'Providence'),
  ('19d8fe1f-2dea-45df-9d4b-3492ecc64f73', '5b000000-0000-0000-0000-000000000001', '891790d4-7896-4744-b6e6-8e85ccfb9290', 'Clinical Nurse', 'staff', 'Clinical Nurse', NULL, '2015-02-01', '2018-08-01', 'Outpatient specialty practice with five providers, about 60 patients a day.', 0, NULL),
  ('3054324f-376b-469b-bd19-9c797aea910a', '5b000000-0000-0000-0000-000000000001', 'c6f975b2-5cca-48b9-9641-99664286bea8', 'Nurse Care Coordinator', 'coordinator', 'Nurse Care Coordinator', NULL, '2018-09-01', '2021-03-01', 'Administrative role in hospital system ambulatory network.', 0, NULL),
  ('ee202c6e-8704-4384-9f74-a894888d1c97', '5b000000-0000-0000-0000-000000000001', 'c6f975b2-5cca-48b9-9641-99664286bea8', 'Charge Nurse', 'charge', 'Charge Nurse, Ambulatory Network', NULL, '2021-03-01', NULL, 'Charge Nurse over the ambulatory network, six clinic sites, roughly 400 referrals a month. Leading quality improvement work for ambulatory since 2023.', 0, NULL)

ON CONFLICT (id) DO UPDATE SET
  user_id = EXCLUDED.user_id,
  employer_id = EXCLUDED.employer_id,
  title = EXCLUDED.title,
  industry_level = EXCLUDED.industry_level,
  industry_role = EXCLUDED.industry_role,
  level_rationale = EXCLUDED.level_rationale,
  started_on = EXCLUDED.started_on,
  ended_on = EXCLUDED.ended_on,
  context_narrative = EXCLUDED.context_narrative,
  sort_order = EXCLUDED.sort_order,
  location = EXCLUDED.location,
  updated_at = now();

-- ============================================================
-- tag_categories
-- ============================================================
INSERT INTO tag_categories (id, user_id, name, aliases, sort_order) VALUES
  ('a4c4c0f3-8f2e-4b62-95d4-efe734a53835', '5b000000-0000-0000-0000-000000000001', 'Certifications', ARRAY['certification', 'licensure', 'credentialed', 'board certified'], 1),
  ('467665e2-a9ff-4c89-88c6-0bdfc039c425', '5b000000-0000-0000-0000-000000000001', 'Health Information Systems', ARRAY['ehr', 'electronic health record', 'charting', 'clinical documentation', 'health information systems'], 2),
  ('e4df9094-a0f0-4bac-9464-a1b50a458eec', '5b000000-0000-0000-0000-000000000001', 'Education and Training', ARRAY['staff education', 'training', 'preceptorship', 'patient education', 'teaching'], 3),
  ('ff3838e9-e62b-4d30-b4d6-e712af351e95', '5b000000-0000-0000-0000-000000000001', 'Languages', NULL, 4),
  ('02381c42-751a-4d27-8abb-e2c89ae5cff1', '5b000000-0000-0000-0000-000000000001', 'Clinical Specialties', NULL, 5),
  ('06b57395-4b59-404f-8530-64cde0048891', '5b000000-0000-0000-0000-000000000001', 'Quality Improvement', ARRAY['quality improvement', 'process improvement', 'performance improvement', 'continuous improvement'], 6),
  ('35b9e658-b3ee-4d55-9ac6-ce7d2df47ffc', '5b000000-0000-0000-0000-000000000001', 'Data and Analytics', ARRAY['data analysis', 'reporting', 'dashboards', 'analytics'], 7),
  ('2e231fc2-9fda-4ff8-bd82-ea48076379eb', '5b000000-0000-0000-0000-000000000001', 'Clinical Operations', ARRAY['care coordination', 'referral management', 'clinical workflow', 'patient flow'], 8)

ON CONFLICT (id) DO UPDATE SET
  name = EXCLUDED.name,
  aliases = EXCLUDED.aliases,
  sort_order = EXCLUDED.sort_order;

-- ============================================================
-- tags
-- ============================================================
INSERT INTO tags (id, user_id, name, aliases, category, sort_order) VALUES
  ('8e38a538-5ea2-48d7-9819-ff140f81e70f', '5b000000-0000-0000-0000-000000000001', 'ACLS', NULL, 'Certifications', 0),
  ('980db3a4-6582-4e20-ae9f-322d11333bb2', '5b000000-0000-0000-0000-000000000001', 'Ambulatory Care Nursing (ANCC)', NULL, 'Certifications', 0),
  ('26a07ef9-d1a6-46c3-b9a5-69eb769c4fec', '5b000000-0000-0000-0000-000000000001', 'BLS', NULL, 'Certifications', 0),
  ('5309f377-b1a9-403e-9fbf-9b429d61421e', '5b000000-0000-0000-0000-000000000001', 'pre-op teaching', NULL, 'Clinical Operations', 0),
  ('58c68b26-d4a1-4b46-bccb-49c8f541ff68', '5b000000-0000-0000-0000-000000000001', 'referral management', NULL, 'Clinical Operations', 0),
  ('dc0d4666-135f-4467-b8bf-0668b0269955', '5b000000-0000-0000-0000-000000000001', 'shift handoff', NULL, 'Clinical Operations', 0),
  ('bafcfe69-f096-496d-ad64-ee45e4f2e2e8', '5b000000-0000-0000-0000-000000000001', 'ambulatory care', NULL, 'Clinical Specialties', 0),
  ('a9926d56-d0de-43d6-98f5-fded5f7938fa', '5b000000-0000-0000-0000-000000000001', 'med-surg', NULL, 'Clinical Specialties', 0),
  ('e6a14cf6-4519-48b2-8a54-75daa45375ba', '5b000000-0000-0000-0000-000000000001', 'Tableau', NULL, 'Data and Analytics', 0),
  ('556903d3-9fe4-411e-b4b1-1fb8c59fdc39', '5b000000-0000-0000-0000-000000000001', 'anticoagulation management', NULL, 'Education and Training', 0),
  ('9eecafec-d741-46b1-b4f7-dc39b974d891', '5b000000-0000-0000-0000-000000000001', 'EHR implementation', NULL, 'Education and Training', 0),
  ('89a0e9b4-1622-40f9-858c-71983aabcc61', '5b000000-0000-0000-0000-000000000001', 'precepting', NULL, 'Education and Training', 0),
  ('a30aa7b0-4f9c-4e77-8aeb-aa686e7accca', '5b000000-0000-0000-0000-000000000001', 'QI methodology training', NULL, 'Education and Training', 0),
  ('ecfddedc-5f39-46b8-8efa-59aa712f8ce4', '5b000000-0000-0000-0000-000000000001', 'Epic', NULL, 'Health Information Systems', 0),
  ('609d0924-604a-41a6-b75f-32ad6134d1dc', '5b000000-0000-0000-0000-000000000001', 'Meditech', NULL, 'Health Information Systems', 0),
  ('0a6e6e08-f484-4ce9-ba93-b7dbff24351a', '5b000000-0000-0000-0000-000000000001', 'Spanish', NULL, 'Languages', 0),
  ('1ffe9f57-f6d5-4a1b-8cfa-c979da2af984', '5b000000-0000-0000-0000-000000000001', 'incident reduction', NULL, 'Quality Improvement', 0),
  ('a0fcb743-0222-4363-96a8-4c44c197f5a0', '5b000000-0000-0000-0000-000000000001', 'PDSA cycles', NULL, 'Quality Improvement', 0),
  ('350ec171-6e1b-4ce2-9dbf-dbabcc9a54bf', '5b000000-0000-0000-0000-000000000001', 'PDSA methodology', NULL, 'Quality Improvement', 0),
  ('84127bd0-8f93-4f75-9c71-293ad9e69bf6', '5b000000-0000-0000-0000-000000000001', 'quality dashboards', NULL, 'Quality Improvement', 0),
  ('f788fc8b-65cb-41c7-87c3-5ef4f763e612', '5b000000-0000-0000-0000-000000000001', 'workflow redesign', NULL, 'Quality Improvement', 0)

ON CONFLICT (id) DO UPDATE SET
  name = EXCLUDED.name,
  aliases = EXCLUDED.aliases,
  category = EXCLUDED.category,
  sort_order = EXCLUDED.sort_order;

COMMIT;
