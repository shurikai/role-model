-- Sample data: foundation layer
--
-- FICTIONAL. Every person, employer, and accomplishment in database/sample/
-- is invented. This dataset exists so someone can clone the repo, run the
-- pipeline end to end, and see real output without access to a private
-- career-history seed set. See database/sample/README.md.
--
-- Scope: users, employers, positions, tag_categories, tags.
-- Contributions are seeded employer-by-employer in follow-on files
-- (002_continental_freightways.sql, etc.), mirroring the structure of the
-- private seed set.
--
-- ALL rows use explicit, stable UUIDs and ON CONFLICT (id) DO UPDATE, so
-- every file here is safe to re-run. Re-running converges existing rows
-- without touching FK relationships held by contribution_tags, skills,
-- resume_versions, etc.
--
-- UUIDs in this dataset all begin with '5' (for "sample") so they can never
-- collide with a real seed set loaded into the same database.

BEGIN;

-- ============================================================
-- User
--
-- Password is the literal string 'sample-password' (bcrypt, default cost).
-- DEMO CREDENTIALS -- do not reuse this hash for anything real.
-- ============================================================
INSERT INTO users (
  id, email, password_hash, full_name, phone, location,
  linkedin_url, github_url, site_url, headline
) VALUES
  ('5a000000-0000-0000-0000-000000000001',
   'sample@example.com',
   '$2a$10$fxDaKBd7DFo7pgWitj9BKuR32oHwJVt.jCpZgOfYeGu31HOttzrKa',
   'Morgan Reyes',
   '(555) 555-0142',
   'Columbus, OH',
   'https://www.linkedin.com/in/example-morgan-reyes',
   'https://github.com/example-morgan-reyes',
   'https://example.com/morgan-reyes',
   'Backend and platform engineer -- freight, logistics, and the systems that move physical goods.')

ON CONFLICT (id) DO UPDATE SET
  email         = EXCLUDED.email,
  password_hash = EXCLUDED.password_hash,
  full_name     = EXCLUDED.full_name,
  phone         = EXCLUDED.phone,
  location      = EXCLUDED.location,
  linkedin_url  = EXCLUDED.linkedin_url,
  github_url    = EXCLUDED.github_url,
  site_url      = EXCLUDED.site_url,
  headline      = EXCLUDED.headline,
  updated_at    = now();

-- ============================================================
-- Level vocabularies
--
-- The seniority ladder and the depth scale are user-owned rows, not enums, so
-- whoever creates the user creates these too. Migration 020 backfills accounts
-- that already existed and internal/vocabulary installs a neutral set at
-- signup; neither of those reaches a user this file invents, which is exactly
-- the shape of the failure migration 018 was written to repair: a column
-- populated by a migration for rows the seed had not created yet.
--
-- This ladder is software's because Morgan Reyes is a software engineer. It is
-- not the default set -- a new account starts on the neutral three-band ladder
-- and infers its real rungs during import.
--
-- length_budget and framing_guidance are the two levers a rung drives: how
-- much gets written, and at what altitude. senior carries is_fallback, so a
-- posting whose seniority reads "unknown" is written at senior length in the
-- plain framing rather than inheriting a claim of ownership nobody made.
--
-- The two framing paragraphs are lifted out into a CTE so each is written
-- once. They are ordinary column data -- a user rewriting one in place is the
-- point of the column existing.
-- ============================================================
WITH framings AS (
  SELECT
    $ownership$This role is pitched at the top of the ladder. A reader at this level is
scanning for scope and accountability, not implementation detail alone.

On the 2-3 most JD-relevant positions, open each bullet with what was owned,
decided, or changed at the system or team level, then land the supporting
evidence — the metric, the scale, the team size — in the same sentence.

  Weaker: "Rebuilt the referral intake process, cutting average wait from
    three weeks to four days."
  Stronger: "Owned referral intake across a six-site region, cutting average
    wait from three weeks to four days by rebuilding how referrals were
    routed and triaged."

Both sentences carry the same fact. The second also says what the candidate
was responsible for. This holds in every field; nothing about it is specific
to any one kind of work.

Two hard limits on this:
  - NEVER trade the evidence for the framing. A bullet that claims ownership
    and drops the number is weaker than one that only reports the number.
    Both together, or the number alone — never the claim alone.
  - NEVER manufacture scope the source material does not support. "Owned",
    "led", "set direction for" are factual claims and need backing in the
    contribution data like any other. If the data shows the work but not the
    ownership, write the work.$ownership$ AS ownership,
    $plain$This role is pitched below the top of the ladder. Lead with the concrete work
and the outcome it produced.

Where the source material genuinely supports ownership or leadership scope,
say so — but do not reach for it. A bullet claiming ownership or scope the
contribution data does not support reads as padding, and costs more
credibility than the framing gains.$plain$ AS plain
), rungs AS (
  SELECT * FROM (VALUES
    ('50000000-0000-0000-0000-000000000001'::uuid, 'junior',    'Junior',    1, ARRAY['entry level', 'entry-level', 'associate'], 'short',  'plain',     FALSE),
    ('50000000-0000-0000-0000-000000000002'::uuid, 'mid',       'Mid-level', 2, ARRAY['mid level', 'mid-level', 'intermediate'],  'short',  'plain',     FALSE),
    ('50000000-0000-0000-0000-000000000003'::uuid, 'senior',    'Senior',    3, ARRAY['senior level']::TEXT[],                    'medium', 'plain',     TRUE),
    ('50000000-0000-0000-0000-000000000004'::uuid, 'lead',      'Lead',      4, ARRAY['tech lead', 'team lead']::TEXT[],          'long',   'ownership', FALSE),
    ('50000000-0000-0000-0000-000000000005'::uuid, 'staff',     'Staff',     5, ARRAY['staff level']::TEXT[],                     'long',   'ownership', FALSE),
    ('50000000-0000-0000-0000-000000000006'::uuid, 'principal', 'Principal', 6, ARRAY['distinguished']::TEXT[],                   'long',   'ownership', FALSE)
  ) AS t(id, value, label, rank, aliases, budget, framing, is_fallback)
)
INSERT INTO career_levels (
  id, user_id, value, label, rank, aliases,
  length_budget, framing_guidance, is_fallback, source, sort_order
)
SELECT
  r.id,
  '5a000000-0000-0000-0000-000000000001',
  r.value,
  r.label,
  r.rank,
  r.aliases,
  CASE r.budget
    WHEN 'short'  THEN 'Target 1 page (~8-10 bullets total across ALL positions and projects combined).'
    WHEN 'long'   THEN 'Target 2 pages (~15-18 bullets total across ALL positions and projects combined).'
    ELSE               'Target 1-2 pages (~12-15 bullets total across ALL positions and projects combined).'
  END,
  CASE r.framing WHEN 'ownership' THEN f.ownership ELSE f.plain END,
  r.is_fallback,
  'default',
  r.rank
FROM rungs r
CROSS JOIN framings f

-- Conflict target is (user_id, value), not (id): migration 020 backfills any
-- account that exists when it runs, with UUIDs of its own, so a seed re-run
-- after a migrate-down/up cycle would otherwise try to insert duplicates of
-- rows already present under different ids.
ON CONFLICT (user_id, value) DO UPDATE SET
  label            = EXCLUDED.label,
  rank             = EXCLUDED.rank,
  aliases          = EXCLUDED.aliases,
  length_budget    = EXCLUDED.length_budget,
  framing_guidance = EXCLUDED.framing_guidance,
  is_fallback      = EXCLUDED.is_fallback,
  source           = EXCLUDED.source,
  sort_order       = EXCLUDED.sort_order,
  updated_at       = now();

INSERT INTO proficiency_levels (
  id, user_id, value, label, rank, aliases, source, sort_order
) VALUES
  ('50000000-0000-0000-0000-000000000011', '5a000000-0000-0000-0000-000000000001',
   'novice', 'Novice', 1, ARRAY['beginner', 'familiarity', 'exposure'], 'default', 1),
  ('50000000-0000-0000-0000-000000000012', '5a000000-0000-0000-0000-000000000001',
   'proficient', 'Proficient', 2, ARRAY['working knowledge', 'solid'], 'default', 2),
  ('50000000-0000-0000-0000-000000000013', '5a000000-0000-0000-0000-000000000001',
   'expert', 'Expert', 3, ARRAY['deep expertise', 'advanced', 'mastery'], 'default', 3)

ON CONFLICT (user_id, value) DO UPDATE SET
  label      = EXCLUDED.label,
  rank       = EXCLUDED.rank,
  aliases    = EXCLUDED.aliases,
  source     = EXCLUDED.source,
  sort_order = EXCLUDED.sort_order,
  updated_at = now();

-- ============================================================
-- Employers
-- ============================================================
INSERT INTO employers (id, user_id, name, industry, notes) VALUES
  ('5e000000-0000-0000-0000-000000000001', '5a000000-0000-0000-0000-000000000001',
   'Continental Freightways', 'Trucking / LTL freight',
   'Regional less-than-truckload carrier, ~4,000 employees. Mainframe-era systems under active modernization. Where the persona learned production operations and EDI.'),

  ('5e000000-0000-0000-0000-000000000002', '5a000000-0000-0000-0000-000000000001',
   'Palletwise', 'Freight brokerage / logistics software',
   'Digital freight brokerage startup. Series A through Series C during tenure; engineering org grew from 9 to ~70. High growth, high load, thin operational maturity.'),

  ('5e000000-0000-0000-0000-000000000003', '5a000000-0000-0000-0000-000000000001',
   'Northbound Logistics', 'Supply chain visibility platform',
   'Supply chain visibility platform serving enterprise shippers. Multi-tenant, high-ingest, strict customer SLAs. Current employer.')

ON CONFLICT (id) DO UPDATE SET
  name       = EXCLUDED.name,
  industry   = EXCLUDED.industry,
  notes      = EXCLUDED.notes,
  updated_at = now();

-- ============================================================
-- Positions
-- ============================================================
INSERT INTO positions (
  id, user_id, employer_id, title, industry_level, industry_role, level_rationale,
  started_on, ended_on, context_narrative, sort_order, location
) VALUES
  -- Continental Freightways
  ('5b000000-0000-0000-0000-000000000001', '5a000000-0000-0000-0000-000000000001',
   '5e000000-0000-0000-0000-000000000001',
   'Software Engineer', 'mid', 'Backend Engineer',
   'Second job out of school; entered at mid level on the strength of an internship on the same codebase.',
   '2013-06-01', '2015-08-01',
   'Shipment tracking and EDI integration team. Maintained a Java monolith fronting a DB2 mainframe of record, and wrote the first automated tests the team had.',
   1, 'Columbus, OH'),

  ('5b000000-0000-0000-0000-000000000002', '5a000000-0000-0000-0000-000000000001',
   '5e000000-0000-0000-0000-000000000001',
   'Senior Software Engineer', 'senior', 'Senior Backend Engineer',
   'Title as held. Owned the EDI integration surface outright and led a two-engineer effort to carve the first services off the monolith.',
   '2015-08-01', '2017-11-01',
   'Led the initial strangler-fig decomposition of the shipment monolith, and rebuilt EDI 214 status messaging as an event-driven pipeline. First exposure to Kafka and to running services the team was paged for.',
   2, 'Columbus, OH'),

  -- Palletwise
  ('5b000000-0000-0000-0000-000000000003', '5a000000-0000-0000-0000-000000000001',
   '5e000000-0000-0000-0000-000000000002',
   'Senior Software Engineer', 'senior', 'Senior Backend Engineer',
   'Title as held. Joined as the ninth engineer, with immediate ownership of the load-matching service.',
   '2017-12-01', '2019-06-01',
   'Load matching and carrier pricing. Rewrote the matching engine in Go, took the service through a 40x traffic increase, and built the on-call practice that had not previously existed.',
   1, 'Remote'),

  ('5b000000-0000-0000-0000-000000000004', '5a000000-0000-0000-0000-000000000001',
   '5e000000-0000-0000-0000-000000000002',
   'Staff Engineer', 'staff', 'Staff Engineer / Platform',
   'Promoted to the company''s first staff-level IC role. Scope spanned three teams without direct reports.',
   '2019-06-01', '2021-09-01',
   'Founded the platform team. Owned the migration from ECS to Kubernetes, the internal service scaffolding, and the reliability program that took the marketplace from two nines to three and a half.',
   2, 'Remote'),

  -- Northbound Logistics
  ('5b000000-0000-0000-0000-000000000005', '5a000000-0000-0000-0000-000000000001',
   '5e000000-0000-0000-0000-000000000003',
   'Staff Software Engineer', 'staff', 'Staff Engineer / Data Platform',
   'Hired at staff level to own the telemetry ingest path end to end.',
   '2021-10-01', '2024-02-01',
   'Rebuilt the carrier telemetry ingest pipeline that backs every customer-facing shipment ETA. Took ingest from 40M to 900M events/day while cutting per-event cost.',
   1, 'Remote'),

  ('5b000000-0000-0000-0000-000000000006', '5a000000-0000-0000-0000-000000000001',
   '5e000000-0000-0000-0000-000000000003',
   'Principal Engineer', 'principal', 'Principal Engineer',
   'Promoted to principal. Company-wide technical scope: architecture review, the multi-region program, and the engineering mentorship track.',
   '2024-02-01', NULL,
   'Current role. Leads the multi-region active-active program and chairs architecture review. Splits time between the hardest technical problems and growing the senior engineers who will take them over.',
   2, 'Remote')

ON CONFLICT (id) DO UPDATE SET
  title             = EXCLUDED.title,
  industry_level    = EXCLUDED.industry_level,
  industry_role     = EXCLUDED.industry_role,
  level_rationale   = EXCLUDED.level_rationale,
  started_on        = EXCLUDED.started_on,
  ended_on          = EXCLUDED.ended_on,
  context_narrative = EXCLUDED.context_narrative,
  sort_order        = EXCLUDED.sort_order,
  location          = EXCLUDED.location,
  updated_at        = now();

-- ============================================================
-- Tag categories
--
-- Deliberately the same nine categories the private seed set uses, so the
-- sample exercises the same category vocabulary the generator expects.
-- ============================================================
-- aliases carries the same competency vocabulary the private seed set uses, and
-- for the same reason: it is the fit gate's third matching layer, the one that
-- lets a capability-worded JD ("CI/CD", "observability", "apis") reach a
-- technology-worded skill inventory. Seeded here rather than by migration 012,
-- which introduced the column but whose UPDATEs run before these rows exist
-- and so matched nothing — see #74 and the note in seed/001_foundation.sql.
--
-- The sample must carry it too, or it stops exercising the layer the generator
-- depends on and a regression here would go unnoticed against sample data.
INSERT INTO tag_categories (id, user_id, name, aliases, sort_order) VALUES
  ('5c000000-0000-0000-0000-000000000001', '5a000000-0000-0000-0000-000000000001', 'Languages', NULL, 1),
  ('5c000000-0000-0000-0000-000000000002', '5a000000-0000-0000-0000-000000000001', 'Frameworks & Libraries', NULL, 2),
  ('5c000000-0000-0000-0000-000000000003', '5a000000-0000-0000-0000-000000000001', 'Cloud & Infrastructure', NULL, 3),
  ('5c000000-0000-0000-0000-000000000004', '5a000000-0000-0000-0000-000000000001', 'Databases',
   ARRAY['data modeling', 'data modelling', 'database design', 'data systems',
         'data stores', 'schema design', 'persistence', 'data pipelines', 'sql'], 4),
  ('5c000000-0000-0000-0000-000000000005', '5a000000-0000-0000-0000-000000000001', 'Protocols & Messaging',
   ARRAY['event-driven systems', 'event driven systems', 'event-driven',
         'event driven architecture', 'api design', 'apis', 'messaging',
         'message brokers', 'pub/sub', 'streaming', 'rpc'], 5),
  ('5c000000-0000-0000-0000-000000000006', '5a000000-0000-0000-0000-000000000001', 'Testing',
   ARRAY['automated testing', 'test automation', 'automated tests',
         'unit testing', 'integration testing', 'testing frameworks',
         'test coverage'], 6),
  ('5c000000-0000-0000-0000-000000000007', '5a000000-0000-0000-0000-000000000001', 'Observability',
   ARRAY['observability', 'monitoring', 'instrumentation', 'telemetry',
         'logging', 'tracing', 'distributed tracing', 'apm',
         'production operations', 'operational readiness'], 7),
  ('5c000000-0000-0000-0000-000000000008', '5a000000-0000-0000-0000-000000000001', 'Methodologies',
   ARRAY['system design', 'systems design', 'architecture',
         'software architecture', 'distributed systems', 'scalability',
         'agile', 'agile development', 'sdlc'], 8),
  ('5c000000-0000-0000-0000-000000000009', '5a000000-0000-0000-0000-000000000001', 'Tools & CI/CD',
   ARRAY['ci/cd', 'ci', 'cd', 'continuous integration', 'continuous delivery',
         'continuous deployment', 'build tooling', 'build automation',
         'release automation', 'devops'], 9)

ON CONFLICT (id) DO UPDATE SET
  name       = EXCLUDED.name,
  aliases    = EXCLUDED.aliases,
  sort_order = EXCLUDED.sort_order;

-- ============================================================
-- Tags
-- ============================================================
INSERT INTO tags (id, user_id, name, aliases, category, sort_order) VALUES
  -- Languages
  ('57000000-0000-0000-0000-000000000001', '5a000000-0000-0000-0000-000000000001', 'Go',         ARRAY['Golang'],                    'Languages', 1),
  ('57000000-0000-0000-0000-000000000002', '5a000000-0000-0000-0000-000000000001', 'Python',     ARRAY['Python 3'],                  'Languages', 2),
  ('57000000-0000-0000-0000-000000000003', '5a000000-0000-0000-0000-000000000001', 'Java',
   ARRAY['Java 8','Java 17','backend systems','backend services','backend engineering','backend development'], 'Languages', 3),
  ('57000000-0000-0000-0000-000000000004', '5a000000-0000-0000-0000-000000000001', 'SQL',        ARRAY['ANSI SQL'],                  'Languages', 4),
  ('57000000-0000-0000-0000-000000000005', '5a000000-0000-0000-0000-000000000001', 'TypeScript', ARRAY['TS'],                        'Languages', 5),

  -- Frameworks & Libraries
  ('57000000-0000-0000-0000-000000000010', '5a000000-0000-0000-0000-000000000001', 'chi',         ARRAY['go-chi'],                   'Frameworks & Libraries', 1),
  ('57000000-0000-0000-0000-000000000011', '5a000000-0000-0000-0000-000000000001', 'FastAPI',     NULL,                              'Frameworks & Libraries', 2),
  ('57000000-0000-0000-0000-000000000012', '5a000000-0000-0000-0000-000000000001', 'Spring Boot', ARRAY['Spring'],                   'Frameworks & Libraries', 3),
  ('57000000-0000-0000-0000-000000000013', '5a000000-0000-0000-0000-000000000001', 'React',       ARRAY['React.js'],                 'Frameworks & Libraries', 4),

  -- Cloud & Infrastructure
  ('57000000-0000-0000-0000-000000000020', '5a000000-0000-0000-0000-000000000001', 'AWS',        ARRAY['Amazon Web Services'],       'Cloud & Infrastructure', 1),
  ('57000000-0000-0000-0000-000000000021', '5a000000-0000-0000-0000-000000000001', 'Kubernetes', ARRAY['K8s','EKS'],                 'Cloud & Infrastructure', 2),
  ('57000000-0000-0000-0000-000000000022', '5a000000-0000-0000-0000-000000000001', 'Terraform',  ARRAY['IaC','Infrastructure as Code'], 'Cloud & Infrastructure', 3),
  ('57000000-0000-0000-0000-000000000023', '5a000000-0000-0000-0000-000000000001', 'Docker',     ARRAY['containers'],                'Cloud & Infrastructure', 4),

  -- Databases
  ('57000000-0000-0000-0000-000000000030', '5a000000-0000-0000-0000-000000000001', 'PostgreSQL',    ARRAY['Postgres'],               'Databases', 1),
  ('57000000-0000-0000-0000-000000000031', '5a000000-0000-0000-0000-000000000001', 'Redis',         NULL,                            'Databases', 2),
  ('57000000-0000-0000-0000-000000000032', '5a000000-0000-0000-0000-000000000001', 'DynamoDB',      ARRAY['Dynamo'],                 'Databases', 3),
  ('57000000-0000-0000-0000-000000000033', '5a000000-0000-0000-0000-000000000001', 'Elasticsearch', ARRAY['ES','OpenSearch'],        'Databases', 4),
  ('57000000-0000-0000-0000-000000000034', '5a000000-0000-0000-0000-000000000001', 'TimescaleDB',   ARRAY['Timescale'],              'Databases', 5),

  -- Protocols & Messaging
  ('57000000-0000-0000-0000-000000000040', '5a000000-0000-0000-0000-000000000001', 'Kafka',      ARRAY['Apache Kafka'],              'Protocols & Messaging', 1),
  ('57000000-0000-0000-0000-000000000041', '5a000000-0000-0000-0000-000000000001', 'gRPC',       ARRAY['protobuf','Protocol Buffers'], 'Protocols & Messaging', 2),
  ('57000000-0000-0000-0000-000000000042', '5a000000-0000-0000-0000-000000000001', 'REST',
   ARRAY['REST API','HTTP API','apis','restful api','backend systems','backend services','backend engineering','backend development'], 'Protocols & Messaging', 3),
  ('57000000-0000-0000-0000-000000000043', '5a000000-0000-0000-0000-000000000001', 'EDI',        ARRAY['EDI 214','EDI 204','X12'],   'Protocols & Messaging', 4),
  ('57000000-0000-0000-0000-000000000044', '5a000000-0000-0000-0000-000000000001', 'WebSockets', NULL,                               'Protocols & Messaging', 5),

  -- Testing
  ('57000000-0000-0000-0000-000000000050', '5a000000-0000-0000-0000-000000000001', 'Integration Testing', ARRAY['integration tests'], 'Testing', 1),
  ('57000000-0000-0000-0000-000000000051', '5a000000-0000-0000-0000-000000000001', 'Load Testing',        ARRAY['performance testing'], 'Testing', 2),
  ('57000000-0000-0000-0000-000000000052', '5a000000-0000-0000-0000-000000000001', 'Contract Testing',    ARRAY['consumer-driven contracts'], 'Testing', 3),

  -- Observability
  ('57000000-0000-0000-0000-000000000060', '5a000000-0000-0000-0000-000000000001', 'Prometheus',    NULL,                            'Observability', 1),
  ('57000000-0000-0000-0000-000000000061', '5a000000-0000-0000-0000-000000000001', 'Grafana',       NULL,                            'Observability', 2),
  ('57000000-0000-0000-0000-000000000062', '5a000000-0000-0000-0000-000000000001', 'OpenTelemetry', ARRAY['OTel','distributed tracing'], 'Observability', 3),

  -- Methodologies
  ('57000000-0000-0000-0000-000000000070', '5a000000-0000-0000-0000-000000000001', 'Distributed Systems',
   ARRAY['backend systems','backend services','backend engineering','backend development'], 'Methodologies', 1),
  ('57000000-0000-0000-0000-000000000071', '5a000000-0000-0000-0000-000000000001', 'Event-Driven Architecture', ARRAY['EDA','event streaming'], 'Methodologies', 2),
  ('57000000-0000-0000-0000-000000000072', '5a000000-0000-0000-0000-000000000001', 'Domain-Driven Design',      ARRAY['DDD'],         'Methodologies', 3),
  ('57000000-0000-0000-0000-000000000073', '5a000000-0000-0000-0000-000000000001', 'Incident Response',         ARRAY['on-call','SRE practices'], 'Methodologies', 4),
  ('57000000-0000-0000-0000-000000000074', '5a000000-0000-0000-0000-000000000001', 'Mentoring',                 ARRAY['coaching','technical mentorship'], 'Methodologies', 5),
  ('57000000-0000-0000-0000-000000000075', '5a000000-0000-0000-0000-000000000001', 'Technical Leadership',      ARRAY['tech lead'],   'Methodologies', 6),

  -- Tools & CI/CD
  ('57000000-0000-0000-0000-000000000080', '5a000000-0000-0000-0000-000000000001', 'GitHub Actions', NULL,                            'Tools & CI/CD', 1),
  ('57000000-0000-0000-0000-000000000081', '5a000000-0000-0000-0000-000000000001', 'ArgoCD',         ARRAY['GitOps'],                 'Tools & CI/CD', 2),
  ('57000000-0000-0000-0000-000000000082', '5a000000-0000-0000-0000-000000000001', 'Jenkins',        NULL,                            'Tools & CI/CD', 3)

ON CONFLICT (id) DO UPDATE SET
  name       = EXCLUDED.name,
  aliases    = EXCLUDED.aliases,
  category   = EXCLUDED.category,
  sort_order = EXCLUDED.sort_order;

COMMIT;
