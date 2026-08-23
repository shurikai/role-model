-- Sample data: clinical career, contributions and their tags
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
-- contributions
-- ============================================================
INSERT INTO contributions (id, user_id, position_id, summary, full_description, outcomes, scale_context, is_active) VALUES
  ('7ce9fca2-b9a8-4496-a661-27ec5fb41c12', '5b000000-0000-0000-0000-000000000001', '9bef3db7-138f-48c1-a951-66ba338da48b', 'Precepted new graduate nurses', 'Precepted new grads, six of them over those years, and two of them are charge nurses now.', 'Two former preceptees became charge nurses.', 'Six new graduate nurses over approximately 3.5 years', TRUE),
  ('8ad15896-dc45-4794-81b8-4e266b367e2d', '5b000000-0000-0000-0000-000000000001', '9bef3db7-138f-48c1-a951-66ba338da48b', 'Built shift handoff sheet to improve continuity of care', 'Built the shift handoff sheet the floor still uses — before that, handoff was verbal and we were losing things between shifts, mostly pending labs and pain-management plans. Put together a one-page structured sheet with the charge nurse.', 'Reduced handoff-related incident reports from about four per month to under one per month.', '32-bed med-surg floor', TRUE),
  ('cafc03f0-f5a9-4c5b-a966-737191a57c33', '5b000000-0000-0000-0000-000000000001', '19d8fe1f-2dea-45df-9d4b-3492ecc64f73', 'Led Epic EHR implementation as clinical lead', 'Ran the practice''s transition to Epic in 2017 — was the clinical lead for it, worked with the vendor team, trained all fourteen clinical staff, and we went live on schedule with no downtime beyond the planned weekend.', 'Went live on schedule with no downtime beyond the planned weekend.', 'Fourteen clinical staff trained; five-provider practice', TRUE),
  ('0a07e1d8-cb6a-4055-81c2-fa7160aaa41a', '5b000000-0000-0000-0000-000000000001', '19d8fe1f-2dea-45df-9d4b-3492ecc64f73', 'Rebuilt pre-op teaching program to reduce procedure cancellations', 'Rebuilt how pre-op teaching worked. Patients were showing up for vascular procedures without having stopped their anticoagulants, and we were cancelling maybe three cases a week. Wrote the teaching materials, set up a call-back protocol two days out.', 'Got the cancellation rate down to under one a month within about six months.', 'Practice seeing about 60 patients a day; reduced cancellations from approximately 12 per month to under 1 per month', TRUE),
  ('bdcd81f7-2a4f-4439-a72f-a89a0e03f839', '5b000000-0000-0000-0000-000000000001', 'ee202c6e-8704-4384-9f74-a894888d1c97', 'Lead quality improvement work for ambulatory network', 'Running quality improvement work for ambulatory since 2023. Lead the PDSA cycles, sit on the network quality committee, and have trained about thirty staff on the improvement methodology.', NULL, 'Thirty staff trained; six clinic sites', TRUE),
  ('51e50d00-0202-43cb-a58d-3379a89025f5', '5b000000-0000-0000-0000-000000000001', 'ee202c6e-8704-4384-9f74-a894888d1c97', 'Rebuilt referral intake system to reduce wait times', 'Referral intake rebuild in 2022 — referrals were sitting an average of three weeks before anyone triaged them, and some were falling off entirely. Mapped the whole thing, found that everything was queuing at a single fax intake point, and rebuilt it as a shared work queue with triage criteria the nurses could apply themselves.', 'Average wait went from three weeks to four days. Also stopped losing referrals.', 'Six clinic sites, roughly 400 referrals a month', TRUE),
  ('7e008499-cff1-425a-a0cf-50a3f8cf721f', '5b000000-0000-0000-0000-000000000001', 'ee202c6e-8704-4384-9f74-a894888d1c97', 'Built quality dashboards for ambulatory network', 'Know my way around Tableau well enough to build the quality dashboards we use.', NULL, 'Six clinic sites in ambulatory network', TRUE)

ON CONFLICT (id) DO UPDATE SET
  user_id = EXCLUDED.user_id,
  position_id = EXCLUDED.position_id,
  summary = EXCLUDED.summary,
  full_description = EXCLUDED.full_description,
  outcomes = EXCLUDED.outcomes,
  scale_context = EXCLUDED.scale_context,
  is_active = EXCLUDED.is_active,
  updated_at = now();

-- ============================================================
-- contribution_tags
-- ============================================================
INSERT INTO contribution_tags (contribution_id, tag_id) VALUES
  ('7ce9fca2-b9a8-4496-a661-27ec5fb41c12', '89a0e9b4-1622-40f9-858c-71983aabcc61'),
  ('8ad15896-dc45-4794-81b8-4e266b367e2d', 'dc0d4666-135f-4467-b8bf-0668b0269955'),
  ('8ad15896-dc45-4794-81b8-4e266b367e2d', '1ffe9f57-f6d5-4a1b-8cfa-c979da2af984'),
  ('cafc03f0-f5a9-4c5b-a966-737191a57c33', 'ecfddedc-5f39-46b8-8efa-59aa712f8ce4'),
  ('cafc03f0-f5a9-4c5b-a966-737191a57c33', '9eecafec-d741-46b1-b4f7-dc39b974d891'),
  ('0a07e1d8-cb6a-4055-81c2-fa7160aaa41a', '5309f377-b1a9-403e-9fbf-9b429d61421e'),
  ('0a07e1d8-cb6a-4055-81c2-fa7160aaa41a', '556903d3-9fe4-411e-b4b1-1fb8c59fdc39'),
  ('bdcd81f7-2a4f-4439-a72f-a89a0e03f839', 'a0fcb743-0222-4363-96a8-4c44c197f5a0'),
  ('bdcd81f7-2a4f-4439-a72f-a89a0e03f839', 'a30aa7b0-4f9c-4e77-8aeb-aa686e7accca'),
  ('51e50d00-0202-43cb-a58d-3379a89025f5', 'f788fc8b-65cb-41c7-87c3-5ef4f763e612'),
  ('51e50d00-0202-43cb-a58d-3379a89025f5', '58c68b26-d4a1-4b46-bccb-49c8f541ff68'),
  ('7e008499-cff1-425a-a0cf-50a3f8cf721f', 'e6a14cf6-4519-48b2-8a54-75daa45375ba'),
  ('7e008499-cff1-425a-a0cf-50a3f8cf721f', '84127bd0-8f93-4f75-9c71-293ad9e69bf6')
ON CONFLICT (contribution_id, tag_id) DO NOTHING;

COMMIT;
