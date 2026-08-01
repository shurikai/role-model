-- Sample data: education and credentials
--
-- FICTIONAL -- see database/sample/README.md.
--
-- Note: this file populates credential_tags, which no seed file in the
-- private set does. Without it the credential -> tag path is unexercised.

BEGIN;

-- ============================================================
-- Education
-- ============================================================
INSERT INTO education (
  id, user_id, institution, degree, field_of_study, started_on, ended_on, notes
) VALUES
  ('58000000-0000-0000-0000-000000000001', '5a000000-0000-0000-0000-000000000001',
   'Ohio State University', 'B.S.', 'Computer Science and Engineering',
   '2009-08-01', '2013-05-01',
   'Senior capstone was a discrete-event simulation of intermodal terminal throughput, which is how the persona ended up in freight rather than out of it.')

ON CONFLICT (id) DO UPDATE SET
  institution    = EXCLUDED.institution,
  degree         = EXCLUDED.degree,
  field_of_study = EXCLUDED.field_of_study,
  started_on     = EXCLUDED.started_on,
  ended_on       = EXCLUDED.ended_on,
  notes          = EXCLUDED.notes,
  updated_at     = now();

INSERT INTO education_tags (education_id, tag_id) VALUES
  ('58000000-0000-0000-0000-000000000001', '57000000-0000-0000-0000-000000000003'), -- Java
  ('58000000-0000-0000-0000-000000000001', '57000000-0000-0000-0000-000000000004'), -- SQL
  ('58000000-0000-0000-0000-000000000001', '57000000-0000-0000-0000-000000000070')  -- Distributed Systems
ON CONFLICT (education_id, tag_id) DO NOTHING;

-- ============================================================
-- Credentials
-- ============================================================
INSERT INTO credentials (
  id, user_id, name, issuer, issued_on, expires_on, credential_url
) VALUES
  ('59000000-0000-0000-0000-000000000001', '5a000000-0000-0000-0000-000000000001',
   'AWS Certified Solutions Architect - Professional', 'Amazon Web Services',
   '2021-04-01', '2027-04-01',
   'https://example.com/credentials/aws-sap-morgan-reyes'),

  ('59000000-0000-0000-0000-000000000002', '5a000000-0000-0000-0000-000000000001',
   'Certified Kubernetes Administrator', 'Cloud Native Computing Foundation',
   '2020-09-01', '2026-09-01',
   'https://example.com/credentials/cka-morgan-reyes'),

  ('59000000-0000-0000-0000-000000000003', '5a000000-0000-0000-0000-000000000001',
   'Confluent Certified Developer for Apache Kafka', 'Confluent',
   '2019-02-01', NULL,
   'https://example.com/credentials/ccdak-morgan-reyes')

ON CONFLICT (id) DO UPDATE SET
  name           = EXCLUDED.name,
  issuer         = EXCLUDED.issuer,
  issued_on      = EXCLUDED.issued_on,
  expires_on     = EXCLUDED.expires_on,
  credential_url = EXCLUDED.credential_url,
  updated_at     = now();

INSERT INTO credential_tags (credential_id, tag_id) VALUES
  -- AWS SA Pro: AWS, Terraform, Distributed Systems
  ('59000000-0000-0000-0000-000000000001', '57000000-0000-0000-0000-000000000020'),
  ('59000000-0000-0000-0000-000000000001', '57000000-0000-0000-0000-000000000022'),
  ('59000000-0000-0000-0000-000000000001', '57000000-0000-0000-0000-000000000070'),

  -- CKA: Kubernetes, Docker
  ('59000000-0000-0000-0000-000000000002', '57000000-0000-0000-0000-000000000021'),
  ('59000000-0000-0000-0000-000000000002', '57000000-0000-0000-0000-000000000023'),

  -- CCDAK: Kafka, Event-Driven Architecture
  ('59000000-0000-0000-0000-000000000003', '57000000-0000-0000-0000-000000000040'),
  ('59000000-0000-0000-0000-000000000003', '57000000-0000-0000-0000-000000000071')

ON CONFLICT (credential_id, tag_id) DO NOTHING;

COMMIT;
