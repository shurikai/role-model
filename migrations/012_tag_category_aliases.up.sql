-- Gives tag_categories the same alias vocabulary that tags already carry,
-- so competency-worded job descriptions can reach a technology-worded skill
-- inventory.
--
-- Context: a staff-level JD scored 0/100 technical with all nine required
-- skills reported as gaps, for capabilities the user demonstrably has. The
-- JD named no concrete technologies at all — it asked for "CI/CD",
-- "observability", "automated testing", "event-driven systems", "data
-- modeling". Extraction faithfully returned those phrases, and the scorer
-- compared them against skill tag *names* (Jenkins, Splunk, JUnit, Kafka,
-- PostgreSQL). No amount of string matching bridges "CI/CD" -> Jenkins:
-- the two sit at different levels of abstraction, and that is a taxonomy
-- problem, not a spelling problem.
--
-- Categories already sit at exactly the abstraction level JDs use — the
-- existing set includes "Tools & CI/CD", "Observability", and "Testing",
-- near-verbatim matches for the phrases above — but scoring never saw them.
-- Aliasing the category is what lets "data modeling" find the Databases
-- category and, through it, PostgreSQL and Cassandra.
--
-- This mirrors tags.aliases (migration 001) deliberately rather than
-- introducing a second mechanism. The vocabulary is user-owned data, editable
-- by SQL as new JD phrasings turn up, and stays out of Go constants.

ALTER TABLE tag_categories ADD COLUMN aliases TEXT[];

-- Seed the standard competency vocabulary, matched by category name.
--
-- Categories are user-defined, so this is deliberately best-effort: a user
-- whose categories are named differently gets no rows updated and no error,
-- and can populate aliases themselves. It is written as one UPDATE per
-- category rather than a CASE so that adding a phrase later is a one-line
-- diff against the category it belongs to.
--
-- A category alias should name the *capability*, not a technology. Putting
-- "kafka" here would be wrong — that belongs in tags.aliases on the Kafka
-- row, where it matches one skill instead of granting credit for the whole
-- category.

UPDATE tag_categories SET aliases =
  ARRAY['ci/cd', 'ci', 'cd', 'continuous integration', 'continuous delivery',
        'continuous deployment', 'build tooling', 'build automation',
        'release automation', 'devops']
WHERE name = 'Tools & CI/CD';

UPDATE tag_categories SET aliases =
  ARRAY['observability', 'monitoring', 'instrumentation', 'telemetry',
        'logging', 'tracing', 'distributed tracing', 'apm',
        'production operations', 'operational readiness']
WHERE name = 'Observability';

UPDATE tag_categories SET aliases =
  ARRAY['automated testing', 'test automation', 'automated tests',
        'unit testing', 'integration testing', 'testing frameworks',
        'test coverage']
WHERE name = 'Testing';

UPDATE tag_categories SET aliases =
  ARRAY['data modeling', 'data modelling', 'database design', 'data systems',
        'data stores', 'schema design', 'persistence', 'data pipelines',
        'sql']
WHERE name = 'Databases';

UPDATE tag_categories SET aliases =
  ARRAY['event-driven systems', 'event driven systems', 'event-driven',
        'event driven architecture', 'api design', 'apis', 'messaging',
        'message brokers', 'pub/sub', 'streaming', 'rpc']
WHERE name = 'Protocols & Messaging';

UPDATE tag_categories SET aliases =
  ARRAY['system design', 'systems design', 'architecture',
        'software architecture', 'distributed systems', 'scalability',
        'agile', 'agile development', 'sdlc']
WHERE name = 'Methodologies';

-- Note what is deliberately absent here: 'iac' and 'infrastructure as code'.
-- Those name a specific practice (Terraform, Pulumi, CloudFormation), not the
-- cloud category as a whole. Aliasing them here would let a JD asking for IaC
-- be answered by Docker and S3, which is exactly the kind of false credit
-- this mechanism must not hand out.
UPDATE tag_categories SET aliases =
  ARRAY['cloud', 'cloud-native', 'cloud native', 'infrastructure',
        'containers', 'container orchestration']
WHERE name = 'Cloud & Infrastructure';

UPDATE tag_categories SET aliases =
  ARRAY['programming languages', 'modern languages', 'backend languages']
WHERE name = 'Languages';

-- Bare 'frameworks' is deliberately NOT an alias. It is a suffix on
-- unrelated requirements — "auth/authz frameworks", "evaluation frameworks",
-- "testing frameworks" — and because matching is whole-word, aliasing it
-- would let every one of those claim credit for React and Spring Boot. A
-- category alias has to be specific enough that matching it means something.
UPDATE tag_categories SET aliases =
  ARRAY['libraries', 'web frameworks']
WHERE name = 'Frameworks & Libraries';
