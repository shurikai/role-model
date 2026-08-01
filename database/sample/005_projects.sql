-- Sample data: projects
--
-- FICTIONAL -- see database/sample/README.md.
--
-- Note: this file populates project_contributions, which no seed file in the
-- private set does (see issue #22). Projects that link back to the work that
-- produced them are what let generation say "this open-source tool came out
-- of that platform work" rather than treating the two as unrelated.

BEGIN;

INSERT INTO projects (
  id, user_id, name, tagline, role, status,
  started_on, ended_on, repo_url, live_url, writeup_url,
  force_include, force_exclude
) VALUES
  ('5d000000-0000-0000-0000-000000000001', '5a000000-0000-0000-0000-000000000001',
   'x12lint',
   'Command-line validator for X12 EDI freight transactions, with error messages written for logistics staff rather than developers.',
   'author', 'active',
   '2016-03-01', NULL,
   'https://github.com/example-morgan-reyes/x12lint',
   NULL,
   'https://example.com/morgan-reyes/writing/x12lint',
   FALSE, FALSE),

  ('5d000000-0000-0000-0000-000000000002', '5a000000-0000-0000-0000-000000000001',
   'hypersink',
   'Go library for high-throughput Kafka-to-TimescaleDB ingest with batching, deduplication, and backpressure.',
   'maintainer', 'active',
   '2022-05-01', NULL,
   'https://github.com/example-morgan-reyes/hypersink',
   NULL,
   NULL,
   FALSE, FALSE),

  ('5d000000-0000-0000-0000-000000000003', '5a000000-0000-0000-0000-000000000001',
   'slobudget',
   'Grafana panel plugin for error-budget burn-down against multi-window SLOs.',
   'contributor', 'dormant',
   '2020-08-01', '2022-11-01',
   'https://github.com/example-org/slobudget',
   'https://grafana.example.com/plugins/slobudget',
   NULL,
   FALSE, FALSE)

ON CONFLICT (id) DO UPDATE SET
  name          = EXCLUDED.name,
  tagline       = EXCLUDED.tagline,
  role          = EXCLUDED.role,
  status        = EXCLUDED.status,
  started_on    = EXCLUDED.started_on,
  ended_on      = EXCLUDED.ended_on,
  repo_url      = EXCLUDED.repo_url,
  live_url      = EXCLUDED.live_url,
  writeup_url   = EXCLUDED.writeup_url,
  force_include = EXCLUDED.force_include,
  force_exclude = EXCLUDED.force_exclude,
  updated_at    = now();

-- ============================================================
-- Project tags
-- ============================================================
INSERT INTO project_tags (project_id, tag_id) VALUES
  -- x12lint: Go, EDI, Integration Testing
  ('5d000000-0000-0000-0000-000000000001', '57000000-0000-0000-0000-000000000001'),
  ('5d000000-0000-0000-0000-000000000001', '57000000-0000-0000-0000-000000000043'),
  ('5d000000-0000-0000-0000-000000000001', '57000000-0000-0000-0000-000000000050'),

  -- hypersink: Go, Kafka, TimescaleDB, Redis, Load Testing
  ('5d000000-0000-0000-0000-000000000002', '57000000-0000-0000-0000-000000000001'),
  ('5d000000-0000-0000-0000-000000000002', '57000000-0000-0000-0000-000000000040'),
  ('5d000000-0000-0000-0000-000000000002', '57000000-0000-0000-0000-000000000034'),
  ('5d000000-0000-0000-0000-000000000002', '57000000-0000-0000-0000-000000000031'),
  ('5d000000-0000-0000-0000-000000000002', '57000000-0000-0000-0000-000000000051'),

  -- slobudget: TypeScript, React, Grafana, Prometheus
  ('5d000000-0000-0000-0000-000000000003', '57000000-0000-0000-0000-000000000005'),
  ('5d000000-0000-0000-0000-000000000003', '57000000-0000-0000-0000-000000000013'),
  ('5d000000-0000-0000-0000-000000000003', '57000000-0000-0000-0000-000000000061'),
  ('5d000000-0000-0000-0000-000000000003', '57000000-0000-0000-0000-000000000060')

ON CONFLICT (project_id, tag_id) DO NOTHING;

-- ============================================================
-- Project <-> contribution links
--
-- Deliberately spans employers: x12lint began as a tool built alongside the
-- Continental EDI work and was still in use for the Northbound integration
-- framework years later.
-- ============================================================
INSERT INTO project_contributions (project_id, contribution_id) VALUES
  -- x12lint <- EDI parser hardening, self-service onboarding, integration framework
  ('5d000000-0000-0000-0000-000000000001', '51000000-0000-0000-0000-000000000001'),
  ('5d000000-0000-0000-0000-000000000001', '51000000-0000-0000-0000-000000000008'),
  ('5d000000-0000-0000-0000-000000000001', '51000000-0000-0000-0000-000000000033'),

  -- hypersink <- telemetry ingest rebuild, tiered retention
  ('5d000000-0000-0000-0000-000000000002', '51000000-0000-0000-0000-000000000023'),
  ('5d000000-0000-0000-0000-000000000002', '51000000-0000-0000-0000-000000000025'),

  -- slobudget <- reliability program, continuous load testing
  ('5d000000-0000-0000-0000-000000000003', '51000000-0000-0000-0000-000000000019'),
  ('5d000000-0000-0000-0000-000000000003', '51000000-0000-0000-0000-000000000034')

ON CONFLICT (contribution_id, project_id) DO NOTHING;

COMMIT;
