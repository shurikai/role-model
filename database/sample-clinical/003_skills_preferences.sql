-- Sample data: clinical career, skills, education, credentials, preferences
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
-- skills
-- ============================================================
INSERT INTO skills (id, user_id, tag_id, proficiency, years_experience, is_active) VALUES
  ('74200eee-9842-4ffb-8ac2-23753455cad1', '5b000000-0000-0000-0000-000000000001', '26a07ef9-d1a6-46c3-b9a5-69eb769c4fec', 'proficient', NULL, TRUE),
  ('471d3579-4bd0-4f7a-b362-90d58018f2ac', '5b000000-0000-0000-0000-000000000001', '609d0924-604a-41a6-b75f-32ad6134d1dc', 'proficient', 3.5, TRUE),
  ('6834a53f-02ef-4b0c-9616-a982ea7fa188', '5b000000-0000-0000-0000-000000000001', '8e38a538-5ea2-48d7-9819-ff140f81e70f', 'proficient', 12, TRUE),
  ('4fecd426-095f-4559-8593-8e172118e22c', '5b000000-0000-0000-0000-000000000001', '980db3a4-6582-4e20-ae9f-322d11333bb2', 'proficient', 5, TRUE),
  ('ec935cf2-1785-459d-bcaa-cbabed3e3dfe', '5b000000-0000-0000-0000-000000000001', '0a6e6e08-f484-4ce9-ba93-b7dbff24351a', 'expert', NULL, TRUE),
  ('3129cb68-5748-417b-bdc4-e5bb3a4836f3', '5b000000-0000-0000-0000-000000000001', 'a9926d56-d0de-43d6-98f5-fded5f7938fa', 'proficient', 3.5, TRUE),
  ('2da72984-3194-424c-aad2-93627a5e6200', '5b000000-0000-0000-0000-000000000001', '89a0e9b4-1622-40f9-858c-71983aabcc61', 'proficient', NULL, TRUE),
  ('f033a48d-338c-4867-a644-2428fdffe978', '5b000000-0000-0000-0000-000000000001', '350ec171-6e1b-4ce2-9dbf-dbabcc9a54bf', 'expert', NULL, TRUE),
  ('2dc568ce-bd3c-496c-b8df-4eca9055ded4', '5b000000-0000-0000-0000-000000000001', 'e6a14cf6-4519-48b2-8a54-75daa45375ba', 'proficient', NULL, TRUE),
  ('c7c35266-564d-4bfb-bb99-edf44fc617bc', '5b000000-0000-0000-0000-000000000001', 'bafcfe69-f096-496d-ad64-ee45e4f2e2e8', 'expert', 6, TRUE),
  ('3417936a-15c2-4b58-a4a0-09c5d8467ef0', '5b000000-0000-0000-0000-000000000001', 'ecfddedc-5f39-46b8-8efa-59aa712f8ce4', 'expert', 7, TRUE)

ON CONFLICT (user_id, tag_id) DO UPDATE SET
  proficiency = EXCLUDED.proficiency,
  years_experience = EXCLUDED.years_experience,
  is_active = EXCLUDED.is_active,
  updated_at = now();

-- ============================================================
-- education
-- ============================================================
INSERT INTO education (id, user_id, institution, degree, field_of_study, started_on, ended_on, notes) VALUES
  ('5b200000-0000-0000-0000-000000000001', '5b000000-0000-0000-0000-000000000001', 'University of Massachusetts Boston', 'MSN', 'Nursing Leadership', '2018-09-01', '2020-05-01', NULL),
  ('5b200000-0000-0000-0000-000000000002', '5b000000-0000-0000-0000-000000000001', 'Rhode Island College', 'BSN', 'Nursing', '2007-09-01', '2011-05-01', NULL)

ON CONFLICT (id) DO UPDATE SET
  user_id = EXCLUDED.user_id,
  institution = EXCLUDED.institution,
  degree = EXCLUDED.degree,
  field_of_study = EXCLUDED.field_of_study,
  started_on = EXCLUDED.started_on,
  ended_on = EXCLUDED.ended_on,
  notes = EXCLUDED.notes,
  updated_at = now();

-- ============================================================
-- credentials
-- ============================================================
INSERT INTO credentials (id, user_id, name, issuer, issued_on, expires_on) VALUES
  ('5b300000-0000-0000-0000-000000000003', '5b000000-0000-0000-0000-000000000001', 'Basic Life Support (BLS)', 'American Heart Association', '2011-05-01', '2026-05-01'),
  ('5b300000-0000-0000-0000-000000000004', '5b000000-0000-0000-0000-000000000001', 'Registered Nurse (RN), Rhode Island', 'RI Department of Health', '2011-07-01', NULL),
  ('5b300000-0000-0000-0000-000000000001', '5b000000-0000-0000-0000-000000000001', 'Advanced Cardiovascular Life Support (ACLS)', 'American Heart Association', '2012-04-01', '2026-04-01'),
  ('5b300000-0000-0000-0000-000000000002', '5b000000-0000-0000-0000-000000000001', 'Ambulatory Care Nursing Certification (RN-BC)', 'ANCC', '2019-09-01', '2029-09-01')

ON CONFLICT (id) DO UPDATE SET
  user_id = EXCLUDED.user_id,
  name = EXCLUDED.name,
  issuer = EXCLUDED.issuer,
  issued_on = EXCLUDED.issued_on,
  expires_on = EXCLUDED.expires_on,
  updated_at = now();

-- ============================================================
-- preferences
-- ============================================================
INSERT INTO preferences (id, user_id, preference_type, label, aliases, sentiment, weight, is_hard_gate, notes) VALUES
  ('5b400000-0000-0000-0000-000000000005', '5b000000-0000-0000-0000-000000000001', 'core_practice', 'Epic', NULL, 'positive', 6, FALSE, 'Seven years and an implementation lead. Typed core_practice because the claim is about prominence, not presence.'),
  ('5b400000-0000-0000-0000-000000000008', '5b000000-0000-0000-0000-000000000001', 'culture', 'interdisciplinary collaboration', ARRAY['cross-functional', 'team-based care'], 'positive', 6, FALSE, NULL),
  ('5b400000-0000-0000-0000-000000000007', '5b000000-0000-0000-0000-000000000001', 'dealbreaker', 'accreditation readiness as the job', ARRAY['Joint Commission readiness', 'survey readiness', 'regulatory compliance focus'], 'negative', 9, TRUE, 'Quality improvement as a compliance exercise rather than improving care.'),
  ('5b400000-0000-0000-0000-000000000006', '5b000000-0000-0000-0000-000000000001', 'dealbreaker', 'inpatient nights', ARRAY['night shift', 'overnight rotation', 'night float'], 'negative', 10, TRUE, 'Four years of nights early on and will not go back.'),
  ('5b400000-0000-0000-0000-000000000001', '5b000000-0000-0000-0000-000000000001', 'domain', 'ambulatory care', ARRAY['outpatient', 'clinic-based', 'ambulatory network'], 'positive', 9, FALSE, 'Seven years in ambulatory and wants to stay.'),
  ('5b400000-0000-0000-0000-000000000002', '5b000000-0000-0000-0000-000000000001', 'domain', 'health equity', ARRAY['language access', 'health disparities', 'underserved populations'], 'positive', 8, FALSE, 'Stated as something she wants IN the job rather than beside it.'),
  ('5b400000-0000-0000-0000-000000000004', '5b000000-0000-0000-0000-000000000001', 'role_shape', 'direct patient contact', ARRAY['clinical contact', 'patient facing', 'bedside component'], 'positive', 7, FALSE, 'Explicitly does not want a role that is purely analysis.'),
  ('5b400000-0000-0000-0000-000000000003', '5b000000-0000-0000-0000-000000000001', 'role_shape', 'quality improvement', ARRAY['performance improvement', 'process improvement', 'clinical quality'], 'positive', 9, FALSE, 'The through-line of the last three years.')

ON CONFLICT (user_id, preference_type, label) DO UPDATE SET
  aliases = EXCLUDED.aliases,
  sentiment = EXCLUDED.sentiment,
  weight = EXCLUDED.weight,
  is_hard_gate = EXCLUDED.is_hard_gate,
  notes = EXCLUDED.notes,
  updated_at = now();

COMMIT;
