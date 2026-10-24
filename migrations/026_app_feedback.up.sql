-- Product feedback from testers, sent from inside the app (#167).
--
-- This is feedback ABOUT THE APP ("this screen confused me", "the import
-- dropped a job"), not feedback on generated output. Output feedback is
-- scoped per resume version and lives in contribution_feedback (#9); the two
-- answer different questions and must not share a table.
--
-- A report is only actionable if it says where the tester was and which
-- build they were on. The app knows both, so the client sends them and the
-- tester never types them: route is pathname + search at the moment of the
-- report, app_version is the frontend build SHA ('dev' locally), and
-- user_agent is read from the request header, never from the body.
--
-- Reports are triaged from the terminal (cmd/feedback) into GitHub issues,
-- so status is the whole workflow: 'new' until someone has looked at it,
-- 'triaged' after. The index serves exactly that read -- new reports,
-- newest first.
CREATE TABLE app_feedback (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id),
    category TEXT NOT NULL CHECK (category IN ('bug', 'confusing', 'idea')),
    message TEXT NOT NULL CHECK (length(message) BETWEEN 1 AND 4000),
    route TEXT NOT NULL,
    app_version TEXT NOT NULL,
    user_agent TEXT,
    viewport TEXT,
    status TEXT NOT NULL DEFAULT 'new' CHECK (status IN ('new', 'triaged')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX app_feedback_status_created_at_idx ON app_feedback (status, created_at);
