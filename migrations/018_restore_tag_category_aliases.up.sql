-- Restores the competency vocabulary migration 012 was supposed to seed.
--
-- 012 added tag_categories.aliases and populated it with one UPDATE per
-- category, so that a capability-worded JD could reach a technology-worded
-- skill inventory: "CI/CD" -> Jenkins, "observability" -> Splunk, "apis" ->
-- REST. In the live database nine of ten categories carried NULL, and the
-- whole third matching layer in internal/fitgate was therefore dead.
--
-- 012 did not fail. It ran against an empty table. Migrations run before
-- `make seed`, and the nine base categories are *created* afterward by
-- database/seed/001_foundation.sql, whose insert names only
-- (id, user_id, name, sort_order) and whose ON CONFLICT (id) DO UPDATE sets
-- only name and sort_order. The UPDATE matched zero rows and the seed never
-- supplied aliases, so the column was never written rather than written and
-- lost. 'AI & LLM' is the sole survivor because seed file 023 carries its
-- aliases inline in its own INSERT.
--
-- 012's comment anticipated a *naming* mismatch ("a user whose categories are
-- named differently gets no rows updated and no error"). The real hazard was
-- ordering, and it is not fixable inside 012: a migration cannot seed rows
-- that do not exist yet. This migration repairs existing databases; the
-- durable fix is that the seed files now carry the aliases themselves, so the
-- two stop disagreeing about who owns the column.
--
-- Symptom that surfaced it: a Senior Staff posting requiring "APIs" scored it
-- as a gap against a profile holding REST at expert, and the narrative told
-- the user they were missing API work. See #74.
--
-- COALESCE, not assignment. A category that already has aliases keeps them —
-- this must not clobber 'AI & LLM', and it must not overwrite vocabulary a
-- user has curated by hand since 012. It is idempotent and safe to re-run:
-- a second pass finds every target non-NULL and changes nothing.
--
-- A category alias names the *capability*, not a technology. Putting 'kafka'
-- here would grant the whole Protocols & Messaging category for one tool;
-- that belongs in tags.aliases on the Kafka row.

UPDATE tag_categories SET aliases = COALESCE(aliases,
  ARRAY['ci/cd', 'ci', 'cd', 'continuous integration', 'continuous delivery',
        'continuous deployment', 'build tooling', 'build automation',
        'release automation', 'devops'])
WHERE name = 'Tools & CI/CD';

UPDATE tag_categories SET aliases = COALESCE(aliases,
  ARRAY['observability', 'monitoring', 'instrumentation', 'telemetry',
        'logging', 'tracing', 'distributed tracing', 'apm',
        'production operations', 'operational readiness'])
WHERE name = 'Observability';

UPDATE tag_categories SET aliases = COALESCE(aliases,
  ARRAY['automated testing', 'test automation', 'automated tests',
        'unit testing', 'integration testing', 'testing frameworks',
        'test coverage'])
WHERE name = 'Testing';

UPDATE tag_categories SET aliases = COALESCE(aliases,
  ARRAY['data modeling', 'data modelling', 'database design', 'data systems',
        'data stores', 'schema design', 'persistence', 'data pipelines',
        'sql'])
WHERE name = 'Databases';

UPDATE tag_categories SET aliases = COALESCE(aliases,
  ARRAY['event-driven systems', 'event driven systems', 'event-driven',
        'event driven architecture', 'api design', 'apis', 'messaging',
        'message brokers', 'pub/sub', 'streaming', 'rpc'])
WHERE name = 'Protocols & Messaging';

UPDATE tag_categories SET aliases = COALESCE(aliases,
  ARRAY['system design', 'systems design', 'architecture',
        'software architecture', 'distributed systems', 'scalability',
        'agile', 'agile development', 'sdlc'])
WHERE name = 'Methodologies';

-- Deliberately absent, same as in 012: 'iac' and 'infrastructure as code' on
-- Cloud & Infrastructure. Those name a specific practice (Terraform, Pulumi,
-- CloudFormation), not the category as a whole, and aliasing them here would
-- let a JD asking for IaC be answered by Docker and S3.
--
-- Also absent: anything for 'backend systems', the other gap on the report
-- that surfaced this. No category is a defensible home for it — Methodologies
-- is closest, but it holds Agile, and offering Agile as evidence of backend
-- systems is the false credit this mechanism exists to avoid. Tracked as its
-- own vocabulary decision rather than smuggled in here.
