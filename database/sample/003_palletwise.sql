-- Sample data: Palletwise contributions (2017-12 .. 2021-09)
--
-- FICTIONAL -- see database/sample/README.md.
--
-- Positions:
--   5b000000-...-03  Senior Software Engineer  (2017-12 .. 2019-06)
--   5b000000-...-04  Staff Engineer            (2019-06 .. 2021-09)

BEGIN;

INSERT INTO contributions (
  id, user_id, position_id, summary, full_description, outcomes, scale_context
) VALUES
  -- ---------- Senior Software Engineer ----------
  ('51000000-0000-0000-0000-000000000011', '5a000000-0000-0000-0000-000000000001',
   '5b000000-0000-0000-0000-000000000003',
   'Rewrote the load-matching engine in Go, cutting match latency 20x',
   'The load-matching service paired available freight with carrier capacity and was the core of the marketplace. The original Python implementation recomputed the full candidate set per request and had become the limiting factor on how fast the marketplace could grow. Rewrote it in Go around an in-memory geospatial index refreshed from a Kafka change feed, with the scoring model kept in a separate module so the data science team could keep iterating on it independently of the serving path.',
   'p99 match latency fell from about 4.2s to 190ms, which made real-time matching viable in the mobile app for the first time. Marketplace GMV roughly tripled over the following year on the same serving infrastructure.',
   'Roughly 12,000 active loads and 40,000 registered carriers at rewrite time.'),

  ('51000000-0000-0000-0000-000000000012', '5a000000-0000-0000-0000-000000000001',
   '5b000000-0000-0000-0000-000000000003',
   'Took load matching through a 40x traffic increase over 14 months',
   'Owned matching as the company went from Series A to Series C and traffic grew roughly 40x. This was less a single project than a sustained sequence: sharding the geospatial index by region, adding a Redis read-through cache for carrier profiles, moving the scoring model behind its own service so it could scale separately, and repeatedly finding that the bottleneck had moved somewhere new. Kept a running capacity model so the team could see the next ceiling before hitting it.',
   'Sustained sub-250ms p99 throughout, with no matching-caused outage during the growth period. The capacity model became the basis for the company''s infrastructure budget forecasting.',
   'Grew from roughly 30,000 to 1.2M match requests per day.'),

  ('51000000-0000-0000-0000-000000000013', '5a000000-0000-0000-0000-000000000001',
   '5b000000-0000-0000-0000-000000000003',
   'Built the carrier pricing service and its backtesting harness',
   'Carrier pricing was hardcoded lane-by-lane in the matching service, which meant every pricing change was a deploy and no one could evaluate a change before shipping it. Extracted pricing into its own gRPC service with versioned pricing strategies, and built a backtesting harness that replayed historical loads against a candidate strategy to estimate margin and fill-rate impact before rollout.',
   'Pricing changes went from a deploy-gated engineering task to a self-service operation the pricing team ran themselves, from roughly one change a month to several a week. The first backtested strategy improved contribution margin by 3.1 points.',
   'Roughly 4,000 lanes; backtests replayed 90 days of historical loads, about 800,000 records per run.'),

  ('51000000-0000-0000-0000-000000000014', '5a000000-0000-0000-0000-000000000001',
   '5b000000-0000-0000-0000-000000000003',
   'Introduced contract testing between the mobile app and backend services',
   'Carrier mobile app releases and backend deploys broke each other regularly, and because app releases go through store review the fix cycle was days rather than minutes. Introduced consumer-driven contract testing: the app team publishes the contract it depends on, backend CI verifies against every published contract, and an incompatible change fails the build rather than the app.',
   'Backend-caused app breakages went from roughly one per month to two over the following two years. The backend team stopped treating mobile compatibility as something to remember and started treating it as something CI enforced.',
   'Two mobile apps (carrier and shipper) against 11 backend services.'),

  ('51000000-0000-0000-0000-000000000015', '5a000000-0000-0000-0000-000000000001',
   '5b000000-0000-0000-0000-000000000003',
   'Created the company''s first on-call rotation and incident practice',
   'The company had grown past the point where the founding engineers could absorb every incident personally, but nothing formal existed: no rotation, no severity definitions, no postmortems. Set up the rotation, wrote severity definitions that the business side actually agreed with, and ran the first year of incident reviews personally to establish that they were blameless in practice and not just in the document.',
   'Mean time to resolution fell from about 3.5 hours to 40 minutes over the first year. The practice survived the engineering org growing from 9 to 70, which was the real test.',
   'Engineering org of 9 at introduction, roughly 70 at departure.'),

  -- ---------- Staff Engineer ----------
  ('51000000-0000-0000-0000-000000000016', '5a000000-0000-0000-0000-000000000001',
   '5b000000-0000-0000-0000-000000000004',
   'Founded the platform team and defined its charter',
   'As the org passed 40 engineers, infrastructure work was being done inconsistently by whichever product team hit the problem first, producing five different deployment patterns and no shared answer for basic questions. Wrote the proposal for a platform team, argued it through to the CTO, and led it as its first technical lead. Deliberately scoped the charter to paved-road tooling rather than gatekeeping, because the failure mode of a new platform team is becoming a ticket queue.',
   'Grew to 6 engineers over 18 months. New service time-to-production went from roughly two weeks of bespoke setup to under a day on the paved road, and 90% of services had migrated to it by departure.',
   'Engineering org of roughly 70 across 9 product teams.'),

  ('51000000-0000-0000-0000-000000000017', '5a000000-0000-0000-0000-000000000001',
   '5b000000-0000-0000-0000-000000000004',
   'Led the migration from ECS to Kubernetes across 40+ services',
   'The ECS setup had accumulated enough per-service special-casing that infrastructure changes required knowing which of five deployment patterns a service used. Led the migration to EKS: built the Terraform modules and Helm chart scaffolding, migrated the three highest-risk services personally to prove the path, then supported product teams through the rest rather than doing it for them. Ran it as an opt-in migration with a hard deadline rather than a big-bang cutover.',
   'Migrated 43 services over nine months with two rollbacks and no customer-visible incidents. Deploy frequency across the org roughly doubled, and infrastructure cost per service fell 30% from improved bin-packing.',
   '43 services, 9 product teams, roughly 70 engineers.'),

  ('51000000-0000-0000-0000-000000000018', '5a000000-0000-0000-0000-000000000001',
   '5b000000-0000-0000-0000-000000000004',
   'Built the internal service scaffolding that became the paved road',
   'Every new service reimplemented the same concerns slightly differently: config loading, structured logging, metrics, tracing, health checks, graceful shutdown. Built a Go service template plus a thin shared library covering those concerns, with a generator that produced a deployable service with CI, dashboards, and alerts already wired. Kept the library deliberately small and resisted requests to put business abstractions in it, which is how shared libraries become the thing everyone fights.',
   'New service setup went from roughly two weeks to under a day. Every service on the template emitted consistent telemetry, which is what made org-wide SLO reporting possible at all.',
   'Adopted by 38 of 43 services; five legacy services intentionally left alone.'),

  ('51000000-0000-0000-0000-000000000019', '5a000000-0000-0000-0000-000000000001',
   '5b000000-0000-0000-0000-000000000004',
   'Ran the reliability program that took the marketplace from 99% to 99.95%',
   'Marketplace availability was around two nines and the business had started losing enterprise deals over it. Led a cross-team reliability program: defined SLOs per user-facing journey rather than per service, instrumented them with OpenTelemetry and Prometheus, and used error budget burn to decide when a team paused feature work. The organizational work of getting product managers to accept error budgets as a real constraint was harder than the instrumentation.',
   'Availability went from about 99.0% to 99.95% over four quarters. Two enterprise deals that had stalled on reliability closed, and the SLO dashboard became a standing item in the weekly business review.',
   '9 product teams, 12 defined user journeys, roughly 1.2M matches per day.'),

  ('51000000-0000-0000-0000-000000000020', '5a000000-0000-0000-0000-000000000001',
   '5b000000-0000-0000-0000-000000000004',
   'Migrated the deployment pipeline to GitOps with ArgoCD',
   'Deploys ran through Jenkins jobs with credentials and cluster state that only the platform team could inspect, which made every deploy question a platform-team interruption. Moved to ArgoCD with declarative manifests in git: what is deployed is what is in the repository, and a product engineer can read the diff themselves. Ran both pipelines in parallel for two months before cutting over.',
   'Deploy-related interruptions to the platform team fell by roughly 70%. Rollback became a git revert, dropping mean rollback time from about 25 minutes to under 3.',
   '43 services across staging and production clusters.'),

  ('51000000-0000-0000-0000-000000000021', '5a000000-0000-0000-0000-000000000001',
   '5b000000-0000-0000-0000-000000000004',
   'Designed the multi-tenant data isolation model for the enterprise tier',
   'The enterprise tier required tenant data isolation guarantees the shared-schema model could not credibly make. Designed a row-level security model in PostgreSQL with tenant context propagated through the service mesh and enforced at the database rather than in application code, on the reasoning that application-layer enforcement fails the first time someone writes a query that forgets it. Wrote the threat model and walked it through the customer security reviews personally.',
   'Passed three enterprise security reviews without remediation findings. The model was later cited in the company''s SOC 2 Type II report as a key control.',
   'Roughly 200 tenants at design time, with the largest three accounting for 40% of volume.'),

  ('51000000-0000-0000-0000-000000000022', '5a000000-0000-0000-0000-000000000001',
   '5b000000-0000-0000-0000-000000000004',
   'Established the engineering onboarding and mentorship program',
   'The org was hiring roughly four engineers a month with onboarding that amounted to a wiki page and a buddy, and new engineers were taking two months to first meaningful contribution. Built a structured program: a curated first-week task sequence that touched every part of the paved road, an assigned mentor with an explicit weekly commitment, and a 30/60/90 checklist owned by the mentor rather than the manager. Personally mentored seven engineers through it.',
   'Time to first production deploy fell from roughly three weeks to four days, and time to first meaningful feature from about two months to five weeks. Four of the seven direct mentees were promoted within two years.',
   'Roughly 45 engineers onboarded through the program during tenure.')

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
  -- 011 Go matching rewrite: Go, Python, Kafka, Distributed Systems, Redis
  ('51000000-0000-0000-0000-000000000011', '57000000-0000-0000-0000-000000000001'),
  ('51000000-0000-0000-0000-000000000011', '57000000-0000-0000-0000-000000000002'),
  ('51000000-0000-0000-0000-000000000011', '57000000-0000-0000-0000-000000000040'),
  ('51000000-0000-0000-0000-000000000011', '57000000-0000-0000-0000-000000000070'),
  ('51000000-0000-0000-0000-000000000011', '57000000-0000-0000-0000-000000000031'),

  -- 012 40x scaling: Go, Redis, AWS, Distributed Systems, Load Testing, Prometheus
  ('51000000-0000-0000-0000-000000000012', '57000000-0000-0000-0000-000000000001'),
  ('51000000-0000-0000-0000-000000000012', '57000000-0000-0000-0000-000000000031'),
  ('51000000-0000-0000-0000-000000000012', '57000000-0000-0000-0000-000000000020'),
  ('51000000-0000-0000-0000-000000000012', '57000000-0000-0000-0000-000000000070'),
  ('51000000-0000-0000-0000-000000000012', '57000000-0000-0000-0000-000000000051'),
  ('51000000-0000-0000-0000-000000000012', '57000000-0000-0000-0000-000000000060'),

  -- 013 pricing service: Go, gRPC, PostgreSQL, DDD
  ('51000000-0000-0000-0000-000000000013', '57000000-0000-0000-0000-000000000001'),
  ('51000000-0000-0000-0000-000000000013', '57000000-0000-0000-0000-000000000041'),
  ('51000000-0000-0000-0000-000000000013', '57000000-0000-0000-0000-000000000030'),
  ('51000000-0000-0000-0000-000000000013', '57000000-0000-0000-0000-000000000072'),

  -- 014 contract testing: Contract Testing, REST, GitHub Actions, TypeScript
  ('51000000-0000-0000-0000-000000000014', '57000000-0000-0000-0000-000000000052'),
  ('51000000-0000-0000-0000-000000000014', '57000000-0000-0000-0000-000000000042'),
  ('51000000-0000-0000-0000-000000000014', '57000000-0000-0000-0000-000000000080'),
  ('51000000-0000-0000-0000-000000000014', '57000000-0000-0000-0000-000000000005'),

  -- 015 on-call practice: Incident Response, Technical Leadership
  ('51000000-0000-0000-0000-000000000015', '57000000-0000-0000-0000-000000000073'),
  ('51000000-0000-0000-0000-000000000015', '57000000-0000-0000-0000-000000000075'),

  -- 016 platform team: Technical Leadership, Kubernetes, Mentoring
  ('51000000-0000-0000-0000-000000000016', '57000000-0000-0000-0000-000000000075'),
  ('51000000-0000-0000-0000-000000000016', '57000000-0000-0000-0000-000000000021'),
  ('51000000-0000-0000-0000-000000000016', '57000000-0000-0000-0000-000000000074'),

  -- 017 ECS -> Kubernetes: Kubernetes, Terraform, AWS, Docker, Technical Leadership
  ('51000000-0000-0000-0000-000000000017', '57000000-0000-0000-0000-000000000021'),
  ('51000000-0000-0000-0000-000000000017', '57000000-0000-0000-0000-000000000022'),
  ('51000000-0000-0000-0000-000000000017', '57000000-0000-0000-0000-000000000020'),
  ('51000000-0000-0000-0000-000000000017', '57000000-0000-0000-0000-000000000023'),
  ('51000000-0000-0000-0000-000000000017', '57000000-0000-0000-0000-000000000075'),

  -- 018 service scaffolding: Go, chi, OpenTelemetry, Prometheus, GitHub Actions
  ('51000000-0000-0000-0000-000000000018', '57000000-0000-0000-0000-000000000001'),
  ('51000000-0000-0000-0000-000000000018', '57000000-0000-0000-0000-000000000010'),
  ('51000000-0000-0000-0000-000000000018', '57000000-0000-0000-0000-000000000062'),
  ('51000000-0000-0000-0000-000000000018', '57000000-0000-0000-0000-000000000060'),
  ('51000000-0000-0000-0000-000000000018', '57000000-0000-0000-0000-000000000080'),

  -- 019 reliability program: OpenTelemetry, Prometheus, Grafana, Incident Response, Technical Leadership
  ('51000000-0000-0000-0000-000000000019', '57000000-0000-0000-0000-000000000062'),
  ('51000000-0000-0000-0000-000000000019', '57000000-0000-0000-0000-000000000060'),
  ('51000000-0000-0000-0000-000000000019', '57000000-0000-0000-0000-000000000061'),
  ('51000000-0000-0000-0000-000000000019', '57000000-0000-0000-0000-000000000073'),
  ('51000000-0000-0000-0000-000000000019', '57000000-0000-0000-0000-000000000075'),

  -- 020 GitOps: ArgoCD, Kubernetes, Jenkins, Terraform
  ('51000000-0000-0000-0000-000000000020', '57000000-0000-0000-0000-000000000081'),
  ('51000000-0000-0000-0000-000000000020', '57000000-0000-0000-0000-000000000021'),
  ('51000000-0000-0000-0000-000000000020', '57000000-0000-0000-0000-000000000082'),
  ('51000000-0000-0000-0000-000000000020', '57000000-0000-0000-0000-000000000022'),

  -- 021 multi-tenant isolation: PostgreSQL, SQL, Distributed Systems, DDD
  ('51000000-0000-0000-0000-000000000021', '57000000-0000-0000-0000-000000000030'),
  ('51000000-0000-0000-0000-000000000021', '57000000-0000-0000-0000-000000000004'),
  ('51000000-0000-0000-0000-000000000021', '57000000-0000-0000-0000-000000000070'),
  ('51000000-0000-0000-0000-000000000021', '57000000-0000-0000-0000-000000000072'),

  -- 022 onboarding program: Mentoring, Technical Leadership
  ('51000000-0000-0000-0000-000000000022', '57000000-0000-0000-0000-000000000074'),
  ('51000000-0000-0000-0000-000000000022', '57000000-0000-0000-0000-000000000075')

ON CONFLICT (contribution_id, tag_id) DO NOTHING;

COMMIT;
