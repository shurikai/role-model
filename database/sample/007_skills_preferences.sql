-- Sample data: skills and preferences
--
-- FICTIONAL -- see database/sample/README.md.
--
-- Neither of these tables is populated by any file in the private seed set.
--
-- skills: migration 008 backfills this from contribution_tags, but on a fresh
-- clone that migration runs against empty tables, so without this file the
-- skills table stays empty after seeding and generation loses its skill
-- signal entirely.
--
-- Proficiency here is deliberately VARIED. The real dataset was backfilled at
-- a uniform 'proficient' / NULL years, which means a weekend prototype and a
-- decade of production work look identical to generation. This sample is the
-- one place the pipeline can be exercised against a dataset where skill depth
-- actually differs -- including the case that matters most, a skill with many
-- years but only moderate depth (SQL, REST) versus fewer years and real depth.
--
-- is_active = FALSE marks skills that are real history but no longer current
-- (Spring Boot, Jenkins), so the active-skill filter has something to filter.
--
-- preferences: internal/fitgate scores preference fit against this table.
-- With no rows the fit gate only ever produces its technical half. The set
-- below covers both sentiments, gate and non-gate rows, and all four
-- preference types, and is
-- built to pair with the JD fixtures in tests/fixtures/ -- see
-- database/sample/README.md for which fixture exercises which path.

BEGIN;

-- ============================================================
-- Skills
-- ============================================================
INSERT INTO skills (id, user_id, tag_id, proficiency, years_experience, is_active) VALUES
  -- ---------- expert ----------
  ('55000000-0000-0000-0000-000000000001', '5a000000-0000-0000-0000-000000000001', '57000000-0000-0000-0000-000000000001', 'expert',     9.0,  TRUE),  -- Go
  ('55000000-0000-0000-0000-000000000002', '5a000000-0000-0000-0000-000000000001', '57000000-0000-0000-0000-000000000030', 'expert',     11.0, TRUE),  -- PostgreSQL
  ('55000000-0000-0000-0000-000000000003', '5a000000-0000-0000-0000-000000000001', '57000000-0000-0000-0000-000000000040', 'expert',     9.5,  TRUE),  -- Kafka
  ('55000000-0000-0000-0000-000000000004', '5a000000-0000-0000-0000-000000000001', '57000000-0000-0000-0000-000000000070', 'expert',     10.0, TRUE),  -- Distributed Systems
  ('55000000-0000-0000-0000-000000000005', '5a000000-0000-0000-0000-000000000001', '57000000-0000-0000-0000-000000000020', 'expert',     9.0,  TRUE),  -- AWS
  ('55000000-0000-0000-0000-000000000006', '5a000000-0000-0000-0000-000000000001', '57000000-0000-0000-0000-000000000071', 'expert',     9.5,  TRUE),  -- Event-Driven Architecture
  ('55000000-0000-0000-0000-000000000007', '5a000000-0000-0000-0000-000000000001', '57000000-0000-0000-0000-000000000075', 'expert',     8.0,  TRUE),  -- Technical Leadership
  ('55000000-0000-0000-0000-000000000008', '5a000000-0000-0000-0000-000000000001', '57000000-0000-0000-0000-000000000073', 'expert',     8.5,  TRUE),  -- Incident Response

  -- ---------- proficient ----------
  -- Note SQL and REST: 13 years each, but breadth of use rather than depth.
  -- Years and proficiency are independent signals and this dataset says so.
  ('55000000-0000-0000-0000-000000000020', '5a000000-0000-0000-0000-000000000001', '57000000-0000-0000-0000-000000000004', 'proficient', 13.0, TRUE),  -- SQL
  ('55000000-0000-0000-0000-000000000021', '5a000000-0000-0000-0000-000000000001', '57000000-0000-0000-0000-000000000042', 'proficient', 13.0, TRUE),  -- REST
  ('55000000-0000-0000-0000-000000000022', '5a000000-0000-0000-0000-000000000001', '57000000-0000-0000-0000-000000000050', 'proficient', 12.0, TRUE),  -- Integration Testing
  ('55000000-0000-0000-0000-000000000023', '5a000000-0000-0000-0000-000000000001', '57000000-0000-0000-0000-000000000003', 'proficient', 6.5,  TRUE),  -- Java
  ('55000000-0000-0000-0000-000000000024', '5a000000-0000-0000-0000-000000000001', '57000000-0000-0000-0000-000000000002', 'proficient', 5.0,  TRUE),  -- Python
  ('55000000-0000-0000-0000-000000000025', '5a000000-0000-0000-0000-000000000001', '57000000-0000-0000-0000-000000000021', 'proficient', 6.5,  TRUE),  -- Kubernetes
  ('55000000-0000-0000-0000-000000000026', '5a000000-0000-0000-0000-000000000001', '57000000-0000-0000-0000-000000000022', 'proficient', 6.0,  TRUE),  -- Terraform
  ('55000000-0000-0000-0000-000000000027', '5a000000-0000-0000-0000-000000000001', '57000000-0000-0000-0000-000000000023', 'proficient', 8.0,  TRUE),  -- Docker
  ('55000000-0000-0000-0000-000000000028', '5a000000-0000-0000-0000-000000000001', '57000000-0000-0000-0000-000000000031', 'proficient', 8.0,  TRUE),  -- Redis
  ('55000000-0000-0000-0000-000000000029', '5a000000-0000-0000-0000-000000000001', '57000000-0000-0000-0000-000000000041', 'proficient', 6.0,  TRUE),  -- gRPC
  ('55000000-0000-0000-0000-000000000030', '5a000000-0000-0000-0000-000000000001', '57000000-0000-0000-0000-000000000074', 'proficient', 8.0,  TRUE),  -- Mentoring
  ('55000000-0000-0000-0000-000000000031', '5a000000-0000-0000-0000-000000000001', '57000000-0000-0000-0000-000000000060', 'proficient', 7.0,  TRUE),  -- Prometheus
  ('55000000-0000-0000-0000-000000000032', '5a000000-0000-0000-0000-000000000001', '57000000-0000-0000-0000-000000000061', 'proficient', 7.0,  TRUE),  -- Grafana
  ('55000000-0000-0000-0000-000000000033', '5a000000-0000-0000-0000-000000000001', '57000000-0000-0000-0000-000000000062', 'proficient', 5.0,  TRUE),  -- OpenTelemetry
  ('55000000-0000-0000-0000-000000000034', '5a000000-0000-0000-0000-000000000001', '57000000-0000-0000-0000-000000000051', 'proficient', 6.0,  TRUE),  -- Load Testing
  ('55000000-0000-0000-0000-000000000035', '5a000000-0000-0000-0000-000000000001', '57000000-0000-0000-0000-000000000052', 'proficient', 5.0,  TRUE),  -- Contract Testing
  ('55000000-0000-0000-0000-000000000036', '5a000000-0000-0000-0000-000000000001', '57000000-0000-0000-0000-000000000072', 'proficient', 7.0,  TRUE),  -- Domain-Driven Design
  ('55000000-0000-0000-0000-000000000037', '5a000000-0000-0000-0000-000000000001', '57000000-0000-0000-0000-000000000080', 'proficient', 5.5,  TRUE),  -- GitHub Actions
  ('55000000-0000-0000-0000-000000000038', '5a000000-0000-0000-0000-000000000001', '57000000-0000-0000-0000-000000000081', 'proficient', 5.0,  TRUE),  -- ArgoCD
  ('55000000-0000-0000-0000-000000000039', '5a000000-0000-0000-0000-000000000001', '57000000-0000-0000-0000-000000000034', 'proficient', 4.5,  TRUE),  -- TimescaleDB
  ('55000000-0000-0000-0000-000000000040', '5a000000-0000-0000-0000-000000000001', '57000000-0000-0000-0000-000000000043', 'proficient', 6.0,  TRUE),  -- EDI
  ('55000000-0000-0000-0000-000000000041', '5a000000-0000-0000-0000-000000000001', '57000000-0000-0000-0000-000000000010', 'proficient', 4.0,  TRUE),  -- chi

  -- ---------- novice ----------
  ('55000000-0000-0000-0000-000000000060', '5a000000-0000-0000-0000-000000000001', '57000000-0000-0000-0000-000000000033', 'novice',     3.5,  TRUE),  -- Elasticsearch
  ('55000000-0000-0000-0000-000000000061', '5a000000-0000-0000-0000-000000000001', '57000000-0000-0000-0000-000000000005', 'novice',     2.0,  TRUE),  -- TypeScript
  ('55000000-0000-0000-0000-000000000062', '5a000000-0000-0000-0000-000000000001', '57000000-0000-0000-0000-000000000013', 'novice',     1.5,  TRUE),  -- React
  ('55000000-0000-0000-0000-000000000063', '5a000000-0000-0000-0000-000000000001', '57000000-0000-0000-0000-000000000011', 'novice',     1.5,  TRUE),  -- FastAPI
  ('55000000-0000-0000-0000-000000000064', '5a000000-0000-0000-0000-000000000001', '57000000-0000-0000-0000-000000000032', 'novice',     1.0,  TRUE),  -- DynamoDB
  ('55000000-0000-0000-0000-000000000065', '5a000000-0000-0000-0000-000000000001', '57000000-0000-0000-0000-000000000044', 'novice',     1.5,  TRUE),  -- WebSockets

  -- ---------- real history, no longer current ----------
  ('55000000-0000-0000-0000-000000000080', '5a000000-0000-0000-0000-000000000001', '57000000-0000-0000-0000-000000000012', 'proficient', 4.5,  FALSE), -- Spring Boot
  ('55000000-0000-0000-0000-000000000081', '5a000000-0000-0000-0000-000000000001', '57000000-0000-0000-0000-000000000082', 'proficient', 5.0,  FALSE)  -- Jenkins

ON CONFLICT (id) DO UPDATE SET
  tag_id           = EXCLUDED.tag_id,
  proficiency      = EXCLUDED.proficiency,
  years_experience = EXCLUDED.years_experience,
  is_active        = EXCLUDED.is_active,
  updated_at       = now();

-- ============================================================
-- Preferences
--
-- Matching in internal/fitgate is a case-insensitive substring check in
-- BOTH directions against the JD's domain, work_type, seniority, and
-- culture_signals fields, so labels are kept short enough to match a
-- canonical JD token.
--
-- Every row carries a weight. A hard exclude is a heavy negative that also
-- sets is_hard_gate: it costs its weight like any negative and additionally
-- caps the preference score. There is no separate hard_exclude sentiment.
-- ============================================================
INSERT INTO preferences (
  id, user_id, preference_type, label, sentiment, weight, is_hard_gate, context_type, notes
) VALUES
  -- ---------- positive ----------
  ('56000000-0000-0000-0000-000000000001', '5a000000-0000-0000-0000-000000000001',
   'domain', 'logistics', 'positive', 9, FALSE, 'permanent',
   'Thirteen years in freight. Domain knowledge compounds and the persona does not want to restart it elsewhere.'),

  ('56000000-0000-0000-0000-000000000002', '5a000000-0000-0000-0000-000000000001',
   'domain', 'supply chain', 'positive', 8, FALSE, NULL,
   'Adjacent to logistics and draws on the same domain knowledge.'),

  ('56000000-0000-0000-0000-000000000003', '5a000000-0000-0000-0000-000000000001',
   'domain', 'developer tooling', 'positive', 6, FALSE, NULL,
   'Secondary interest, from the platform team years.'),

  ('56000000-0000-0000-0000-000000000004', '5a000000-0000-0000-0000-000000000001',
   'work_type', 'backend', 'positive', 9, FALSE, 'permanent',
   'Core of the career and where the persona is strongest.'),

  ('56000000-0000-0000-0000-000000000005', '5a000000-0000-0000-0000-000000000001',
   'work_type', 'distributed systems', 'positive', 9, FALSE, NULL,
   'The problems the persona finds most interesting.'),

  ('56000000-0000-0000-0000-000000000006', '5a000000-0000-0000-0000-000000000001',
   'work_type', 'platform engineering', 'positive', 8, FALSE, NULL,
   'Founded one platform team and would do it again.'),

  ('56000000-0000-0000-0000-000000000007', '5a000000-0000-0000-0000-000000000001',
   'culture', 'remote', 'positive', 8, FALSE, NULL,
   'Remote since 2017 and not returning to an office full time.'),

  ('56000000-0000-0000-0000-000000000008', '5a000000-0000-0000-0000-000000000001',
   'culture', 'mentoring', 'positive', 7, FALSE, NULL,
   'Wants a role where growing other engineers is part of the job, not a side activity.'),

  ('56000000-0000-0000-0000-000000000009', '5a000000-0000-0000-0000-000000000001',
   'culture', 'blameless postmortems', 'positive', 5, FALSE, NULL,
   'A reliable proxy for whether an engineering org is honest about failure.'),

  -- ---------- negative ----------
  ('56000000-0000-0000-0000-000000000020', '5a000000-0000-0000-0000-000000000001',
   'work_type', 'frontend', 'negative', 6, FALSE, NULL,
   'Can do it, does not want it as the majority of the role. See the novice React and TypeScript entries in skills.'),

  ('56000000-0000-0000-0000-000000000021', '5a000000-0000-0000-0000-000000000001',
   'work_type', 'people management', 'negative', 7, FALSE, NULL,
   'Chose the IC track deliberately at the staff level and intends to stay on it.'),

  ('56000000-0000-0000-0000-000000000022', '5a000000-0000-0000-0000-000000000001',
   'culture', 'on-call heavy', 'negative', 5, FALSE, NULL,
   'Has built two on-call practices and will carry a pager, but not one that dominates the role.'),

  ('56000000-0000-0000-0000-000000000023', '5a000000-0000-0000-0000-000000000001',
   'culture', 'mandatory overtime', 'negative', 7, FALSE, NULL,
   'Non-negotiable enough to be worth flagging, but not an automatic disqualification.'),

  -- ---------- hard gates (heavy negatives that also cap the score) ----------
  -- The one skills-shaped gate. The other three are industry-shaped, and an
  -- industry is not currently matchable (see #48) -- so without this row no
  -- fixture could exercise a gate firing end to end.
  ('56000000-0000-0000-0000-000000000043', '5a000000-0000-0000-0000-000000000001',
   'anti_pattern', '.NET', 'negative', 10, TRUE, NULL,
   'Will not move to the Microsoft stack. Go/Java/Python is the career direction.'),

  -- The one prominence-shaped gate. Every other row here objects to a
  -- technology or industry being present at all; this one objects only to
  -- TypeScript being what the role is BUILT ON, which is a different question
  -- and is why it is typed primary_stack. The persona is a novice at
  -- TypeScript and React and would still take a backend role that touches
  -- them. Without a row of this shape no fixture exercises the distinction
  -- #68 turns on.
  ('56000000-0000-0000-0000-000000000044', '5a000000-0000-0000-0000-000000000001',
   'primary_stack', 'TypeScript / Node.js as a primary language', 'negative', 10, TRUE, NULL,
   'Backend is the career direction. A TypeScript-primary role is a different job, not a stretch assignment.'),

  ('56000000-0000-0000-0000-000000000040', '5a000000-0000-0000-0000-000000000001',
   'anti_pattern', 'adtech', 'negative', 10, TRUE, NULL,
   'Will not work on advertising technology.'),

  ('56000000-0000-0000-0000-000000000041', '5a000000-0000-0000-0000-000000000001',
   'anti_pattern', 'gambling', 'negative', 10, TRUE, NULL,
   'Will not work on gambling or sports betting.'),

  ('56000000-0000-0000-0000-000000000042', '5a000000-0000-0000-0000-000000000001',
   'anti_pattern', 'surveillance', 'negative', 10, TRUE, NULL,
   'Will not work on consumer surveillance products.')

ON CONFLICT (id) DO UPDATE SET
  preference_type = EXCLUDED.preference_type,
  label           = EXCLUDED.label,
  sentiment       = EXCLUDED.sentiment,
  weight          = EXCLUDED.weight,
  is_hard_gate    = EXCLUDED.is_hard_gate,
  context_type    = EXCLUDED.context_type,
  notes           = EXCLUDED.notes,
  updated_at      = now();

COMMIT;
