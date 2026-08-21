-- Maps two capability terms onto the specific skills that answer them, using
-- tags.aliases as a one-to-many mechanism.
--
-- # Why this works without a schema change
--
-- The fit gate's alias layer does not stop at the first hit. satisfies() loops
-- every skill, collects every one whose aliases match the JD term, and returns
-- them as a single match with all of them as evidence. So the *same* alias
-- string placed on several tags is a term -> {skills} mapping, and unlike a
-- category alias it can span categories. tags.category is a single NOT NULL
-- column, so categories strictly partition tags: "backend systems" reaching
-- Java (Languages), Microservices and Distributed systems (Methodologies), and
-- REST (Protocols & Messaging) is not expressible any other way today.
--
-- # Why not the category layer
--
-- 018 restored 'apis' and 'api design' onto the Protocols & Messaging category,
-- which made a JD requiring "APIs" match — with DDS, DIS and HLA in the
-- evidence. Those are distributed-simulation protocols, not API work, and a
-- narrative citing them as API experience is exactly the false credit the
-- category rule exists to prevent. Naming REST directly returns REST alone.
--
-- Nothing is removed from the category vocabulary. The layers are ordered
-- direct -> alias -> category, so a precise alias here automatically demotes
-- the broader category match while leaving it as the fallback for a profile
-- that does not claim REST at all.
--
-- # The cost, recorded honestly
--
-- tags.aliases means "other names for this tag". "backend systems" is not
-- another name for Java, so this column now carries two relations: "is also
-- called" and "contributes to". The match therefore reports Kind: alias, which
-- overstates a competency roll-up as a synonym. The mapping also has no single
-- home — changing what "backend systems" covers means editing four rows — and
-- it does not follow new skills the way a category alias does.
--
-- That is accepted deliberately rather than overlooked. A competency_terms /
-- competency_term_tags pair with its own MatchKind is the correct model, and
-- the signal to build it is this list outgrowing a handful of terms or someone
-- needing to ask "what does 'backend systems' currently cover". See #74.
--
-- # Matching notes
--
-- Terms are whole-phrase matched, so 'backend systems' does NOT answer a JD
-- that says only "backend" — hence the four spellings rather than one.
--
-- Two terms were considered and deliberately left out, both because they reach
-- further than the capability they name:
--
--   'server-side' also matches "server-side rendering", a Next.js/React
--   concern, which would answer a frontend requirement with Java and
--   Microservices.
--
--   bare 'api' matches any "<X> API" phrase — "GraphQL API", "SOAP API",
--   "gRPC API" would all be answered by REST. That one mattered here: GraphQL
--   was deactivated as a skill in seed 020 precisely so a JD requiring it
--   would report an honest gap, and 'api' would have silently undone that.
--   'api' was also inert for its own case, since the direct layer's substring
--   direction answers a bare "API" with Anthropic API and FastAPI before the
--   alias layer runs. See #75 for that asymmetry.
--
-- 'apis' is kept: it is the plural the extraction actually emitted, and it
-- matches only the bare term.
--
-- Idempotent: appends only terms not already present, compared
-- case-insensitively, and preserves existing alias order. Matching on lower(name)
-- so it reaches datasets that capitalize differently ("Distributed Systems").
-- Not scoped to a user, following 012: this is vocabulary, and a user whose
-- tags are named differently gets no rows updated and no error.

UPDATE tags SET aliases = COALESCE(aliases, '{}') || ARRAY(
  SELECT term
  FROM unnest(ARRAY['backend systems', 'backend services', 'backend engineering',
                    'backend development']) AS term
  WHERE lower(term) NOT IN (
    SELECT lower(existing) FROM unnest(COALESCE(tags.aliases, '{}')) AS existing
  )
)
WHERE lower(name) IN ('java', 'microservices', 'distributed systems', 'rest');

UPDATE tags SET aliases = COALESCE(aliases, '{}') || ARRAY(
  SELECT term
  FROM unnest(ARRAY['apis', 'rest api', 'restful api']) AS term
  WHERE lower(term) NOT IN (
    SELECT lower(existing) FROM unnest(COALESCE(tags.aliases, '{}')) AS existing
  )
)
WHERE lower(name) = 'rest';
