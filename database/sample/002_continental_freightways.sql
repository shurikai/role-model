-- Sample data: Continental Freightways contributions (2013-06 .. 2017-11)
--
-- FICTIONAL -- see database/sample/README.md.
--
-- Positions:
--   5b000000-...-01  Software Engineer         (2013-06 .. 2015-08)
--   5b000000-...-02  Senior Software Engineer  (2015-08 .. 2017-11)

BEGIN;

INSERT INTO contributions (
  id, user_id, position_id, summary, full_description, outcomes, scale_context
) VALUES
  -- ---------- Software Engineer ----------
  ('51000000-0000-0000-0000-000000000001', '5a000000-0000-0000-0000-000000000001',
   '5b000000-0000-0000-0000-000000000001',
   'Hardened the EDI 214 shipment status parser against malformed carrier feeds',
   'The inbound EDI 214 parser assumed well-formed X12 envelopes and crashed the whole batch when any single segment was malformed, which meant one bad partner feed silently stalled status updates for every customer until someone noticed. Rewrote the parser to validate and quarantine per-transaction rather than per-batch, added a dead-letter table with an operator review queue, and wrote a corpus of 60 real-world malformed samples collected from support tickets as regression fixtures.',
   'Cut EDI-related status delays from roughly 12 incidents a quarter to under one. The quarantine queue became the standard tool support used to answer "where is my shipment status" questions without engineering involvement.',
   'Approximately 180 trading partners, 40,000 EDI 214 transactions per day.'),

  ('51000000-0000-0000-0000-000000000002', '5a000000-0000-0000-0000-000000000001',
   '5b000000-0000-0000-0000-000000000001',
   'Built the first public shipment tracking API for customer integrations',
   'Enterprise customers were screen-scraping the tracking web page because there was no programmatic access. Designed and shipped a REST tracking API over the existing Java monolith: token authentication, per-customer rate limiting, and a response model that deliberately did not leak the mainframe record structure so the backing store could be replaced later. Wrote the public documentation and the first two customer integration guides.',
   'Adopted by 23 enterprise customers in the first year and eliminated the scraping traffic that had been the top source of unplanned load on the tracking page.',
   'Roughly 2M tracking requests per month at handoff.'),

  ('51000000-0000-0000-0000-000000000003', '5a000000-0000-0000-0000-000000000001',
   '5b000000-0000-0000-0000-000000000001',
   'Introduced the team''s first automated test suite and CI pipeline',
   'The shipment team had no automated tests and released by hand on Thursday nights. Started by writing characterization tests around the rating and routing code that changed most often, then stood up a Jenkins pipeline that ran them on every commit. Spent several months pairing with teammates to write tests alongside changes rather than mandating coverage targets, which had failed on a previous team.',
   'Took the suite from zero to about 400 tests over 18 months. Thursday-night release rollbacks went from routine to rare, and the practice spread to two adjacent teams.',
   'Java monolith of roughly 400,000 lines, 11 engineers across three teams.'),

  ('51000000-0000-0000-0000-000000000004', '5a000000-0000-0000-0000-000000000001',
   '5b000000-0000-0000-0000-000000000001',
   'Offloaded read traffic from the DB2 system of record to a PostgreSQL replica',
   'Reporting and tracking queries ran directly against the DB2 mainframe of record, and month-end reporting reliably degraded shipment lookups for everyone. Built a change-data-capture feed into a PostgreSQL replica shaped for read patterns rather than for the mainframe''s record layout, then migrated tracking and reporting reads over behind a feature flag, one query family at a time.',
   'Removed month-end tracking degradation entirely and cut p95 tracking latency from 1.8s to 240ms. The replica later became the foundation the strangler-fig decomposition built on.',
   '14 years of shipment history, roughly 900GB at migration time.'),

  ('51000000-0000-0000-0000-000000000005', '5a000000-0000-0000-0000-000000000001',
   '5b000000-0000-0000-0000-000000000001',
   'Cut the nightly freight rating batch from six hours to under one',
   'The nightly rating job recalculated tariffs for all open shipments and had grown to a six-hour run that regularly overran its window and pushed into the business day. Profiled it and found the dominant cost was per-shipment tariff lookups rather than the rating math. Restructured it to load the tariff set once into memory, partition shipments across worker threads, and write results in bulk.',
   'Runtime dropped from about 6 hours to 50 minutes, which ended the overrun problem and gave the business room to add two new tariff dimensions that had been blocked on the window.',
   'Roughly 250,000 open shipments per nightly run.'),

  -- ---------- Senior Software Engineer ----------
  ('51000000-0000-0000-0000-000000000006', '5a000000-0000-0000-0000-000000000001',
   '5b000000-0000-0000-0000-000000000002',
   'Led the strangler-fig decomposition of the shipment monolith',
   'Owned the technical plan for extracting services from the Java monolith without a rewrite. Chose shipment status as the first seam because it had the clearest boundary and the most operational pain, and built an HTTP facade that routed each endpoint to either the monolith or the new service by configuration, so extraction could proceed and be reversed one endpoint at a time. Led two other engineers through the first three extractions and wrote the decomposition guide the team used afterward.',
   'Extracted three services over 20 months with no customer-visible downtime and no rollback that took more than ten minutes. The pattern was adopted by the billing team for their own decomposition.',
   'Monolith of roughly 400,000 lines; the extracted status service handled about 60% of external request volume.'),

  ('51000000-0000-0000-0000-000000000007', '5a000000-0000-0000-0000-000000000001',
   '5b000000-0000-0000-0000-000000000002',
   'Rebuilt EDI 214 status messaging as an event-driven Kafka pipeline',
   'Outbound status messaging to customers ran as a polling batch every 15 minutes, which was the single largest source of "your tracking is stale" complaints. Redesigned it around Kafka: status changes published as events at the point of change, with per-customer consumers handling format translation and delivery, including retry and replay. Designed the event schema to be additive-only and wrote the compatibility rules the team still uses.',
   'Median status delivery latency went from about 8 minutes to under 15 seconds. Replay capability meant a failed partner integration could be recovered without a database export, which had previously been a multi-day manual process.',
   'Approximately 180 partners, 40,000 outbound messages per day at peak.'),

  ('51000000-0000-0000-0000-000000000008', '5a000000-0000-0000-0000-000000000001',
   '5b000000-0000-0000-0000-000000000002',
   'Shipped self-service carrier onboarding to replace a manual integration process',
   'Onboarding a new trading partner took roughly three weeks of engineering time to hand-configure mappings and validate test transactions. Built a self-service onboarding flow where partners upload sample transactions, see validation results against the X12 spec immediately, and progress through a certification checklist without engineering involvement. The hard part was making the validation errors legible to logistics staff rather than to developers.',
   'Onboarding dropped from about three weeks to four days, and engineering involvement went from every partner to only genuinely unusual ones. Partner count grew 40% the following year without adding integration headcount.',
   'Roughly 180 existing partners at launch, 70 onboarded through the self-service flow in the first year.'),

  ('51000000-0000-0000-0000-000000000009', '5a000000-0000-0000-0000-000000000001',
   '5b000000-0000-0000-0000-000000000002',
   'Established the team''s first on-call rotation, runbooks, and incident review',
   'As services moved out of the monolith the team started getting paged, with no rotation, no runbooks, and no shared idea of what was worth waking up for. Set up a weekly rotation, wrote the first runbooks for the three services that paged most, and introduced blameless incident reviews. Also cut the alert set roughly in half, because most existing alerts were not actionable and were training people to ignore the pager.',
   'Pages per week fell from about 14 to 5 within two quarters, with the reduction concentrated in non-actionable alerts. Mean time to acknowledge improved from 22 minutes to under 5.',
   'Three extracted services plus the remaining monolith, 6 engineers in the rotation.'),

  ('51000000-0000-0000-0000-000000000010', '5a000000-0000-0000-0000-000000000001',
   '5b000000-0000-0000-0000-000000000002',
   'Mentored three junior engineers and rebuilt the team''s technical interview',
   'Took on formal mentorship for three junior engineers, two of whom were career changers from the operations side of the business and knew freight far better than they knew Java. Restructured mentorship around shipping real work with close review rather than side projects. Separately, replaced the team''s algorithm-puzzle interview with a debugging exercise on a realistic codebase, because the puzzle round had been screening out exactly the operations-background candidates who turned out to perform best.',
   'All three mentees were promoted within two years. The revised interview roughly doubled the offer rate for non-traditional candidates without any drop in performance ratings at the one-year mark.',
   '11-engineer department; the interview format was adopted department-wide.')

ON CONFLICT (id) DO UPDATE SET
  summary          = EXCLUDED.summary,
  full_description = EXCLUDED.full_description,
  outcomes         = EXCLUDED.outcomes,
  scale_context    = EXCLUDED.scale_context,
  updated_at       = now();

-- ============================================================
-- Contribution tags
-- ============================================================
INSERT INTO contribution_tags (contribution_id, tag_id) VALUES
  -- 001 EDI 214 parser hardening: Java, EDI, Integration Testing
  ('51000000-0000-0000-0000-000000000001', '57000000-0000-0000-0000-000000000003'),
  ('51000000-0000-0000-0000-000000000001', '57000000-0000-0000-0000-000000000043'),
  ('51000000-0000-0000-0000-000000000001', '57000000-0000-0000-0000-000000000050'),

  -- 002 tracking API: Java, Spring Boot, REST, SQL
  ('51000000-0000-0000-0000-000000000002', '57000000-0000-0000-0000-000000000003'),
  ('51000000-0000-0000-0000-000000000002', '57000000-0000-0000-0000-000000000012'),
  ('51000000-0000-0000-0000-000000000002', '57000000-0000-0000-0000-000000000042'),
  ('51000000-0000-0000-0000-000000000002', '57000000-0000-0000-0000-000000000004'),

  -- 003 test suite + CI: Java, Integration Testing, Jenkins, Mentoring
  ('51000000-0000-0000-0000-000000000003', '57000000-0000-0000-0000-000000000003'),
  ('51000000-0000-0000-0000-000000000003', '57000000-0000-0000-0000-000000000050'),
  ('51000000-0000-0000-0000-000000000003', '57000000-0000-0000-0000-000000000082'),
  ('51000000-0000-0000-0000-000000000003', '57000000-0000-0000-0000-000000000074'),

  -- 004 DB2 -> Postgres replica: PostgreSQL, SQL, Java, Distributed Systems
  ('51000000-0000-0000-0000-000000000004', '57000000-0000-0000-0000-000000000030'),
  ('51000000-0000-0000-0000-000000000004', '57000000-0000-0000-0000-000000000004'),
  ('51000000-0000-0000-0000-000000000004', '57000000-0000-0000-0000-000000000003'),
  ('51000000-0000-0000-0000-000000000004', '57000000-0000-0000-0000-000000000070'),

  -- 005 rating batch: Java, SQL, Load Testing
  ('51000000-0000-0000-0000-000000000005', '57000000-0000-0000-0000-000000000003'),
  ('51000000-0000-0000-0000-000000000005', '57000000-0000-0000-0000-000000000004'),
  ('51000000-0000-0000-0000-000000000005', '57000000-0000-0000-0000-000000000051'),

  -- 006 strangler-fig: Java, Spring Boot, REST, DDD, Technical Leadership, Distributed Systems
  ('51000000-0000-0000-0000-000000000006', '57000000-0000-0000-0000-000000000003'),
  ('51000000-0000-0000-0000-000000000006', '57000000-0000-0000-0000-000000000012'),
  ('51000000-0000-0000-0000-000000000006', '57000000-0000-0000-0000-000000000042'),
  ('51000000-0000-0000-0000-000000000006', '57000000-0000-0000-0000-000000000072'),
  ('51000000-0000-0000-0000-000000000006', '57000000-0000-0000-0000-000000000075'),
  ('51000000-0000-0000-0000-000000000006', '57000000-0000-0000-0000-000000000070'),

  -- 007 EDI over Kafka: Kafka, EDI, Event-Driven Architecture, Java, Distributed Systems
  ('51000000-0000-0000-0000-000000000007', '57000000-0000-0000-0000-000000000040'),
  ('51000000-0000-0000-0000-000000000007', '57000000-0000-0000-0000-000000000043'),
  ('51000000-0000-0000-0000-000000000007', '57000000-0000-0000-0000-000000000071'),
  ('51000000-0000-0000-0000-000000000007', '57000000-0000-0000-0000-000000000003'),
  ('51000000-0000-0000-0000-000000000007', '57000000-0000-0000-0000-000000000070'),

  -- 008 self-service onboarding: EDI, REST, Java, PostgreSQL
  ('51000000-0000-0000-0000-000000000008', '57000000-0000-0000-0000-000000000043'),
  ('51000000-0000-0000-0000-000000000008', '57000000-0000-0000-0000-000000000042'),
  ('51000000-0000-0000-0000-000000000008', '57000000-0000-0000-0000-000000000003'),
  ('51000000-0000-0000-0000-000000000008', '57000000-0000-0000-0000-000000000030'),

  -- 009 on-call practice: Incident Response, Prometheus, Grafana, Technical Leadership
  ('51000000-0000-0000-0000-000000000009', '57000000-0000-0000-0000-000000000073'),
  ('51000000-0000-0000-0000-000000000009', '57000000-0000-0000-0000-000000000060'),
  ('51000000-0000-0000-0000-000000000009', '57000000-0000-0000-0000-000000000061'),
  ('51000000-0000-0000-0000-000000000009', '57000000-0000-0000-0000-000000000075'),

  -- 010 mentoring + interview redesign: Mentoring, Technical Leadership
  ('51000000-0000-0000-0000-000000000010', '57000000-0000-0000-0000-000000000074'),
  ('51000000-0000-0000-0000-000000000010', '57000000-0000-0000-0000-000000000075')

ON CONFLICT (contribution_id, tag_id) DO NOTHING;

COMMIT;
