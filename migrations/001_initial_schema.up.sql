-- Users (stub for now, enables multi-tenant later)
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Employers
CREATE TABLE employers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    name TEXT NOT NULL,
    industry TEXT,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Positions held within an employer
CREATE TABLE positions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    employer_id UUID NOT NULL REFERENCES employers(id),
    title TEXT NOT NULL,
    industry_level TEXT
        CHECK (industry_level IN (
            'junior','mid','senior','staff','principal',
            'lead','manager','director','vp','ic'
        )),
    industry_role TEXT,
    level_rationale TEXT,
    started_on DATE NOT NULL,
    ended_on DATE,
    context_narrative TEXT,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Contributions: the atomic unit of career knowledge
CREATE TABLE contributions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    position_id UUID NOT NULL REFERENCES positions(id),
    summary TEXT NOT NULL,
    full_description TEXT NOT NULL,
    outcomes TEXT,
    scale_context TEXT,
    is_active BOOL NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Education
CREATE TABLE education (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    institution TEXT NOT NULL,
    degree TEXT,
    field_of_study TEXT,
    started_on DATE,
    ended_on DATE,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Credentials
CREATE TABLE credentials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    name TEXT NOT NULL,
    issuer TEXT,
    issued_on DATE,
    expires_on DATE,
    credential_url TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Tag categories
CREATE TABLE tag_categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    name TEXT NOT NULL,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, name)
);

-- Canonical tag vocabulary
CREATE TABLE tags (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    name TEXT NOT NULL,
    aliases TEXT[],
    category TEXT NOT NULL,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, name),
    FOREIGN KEY (user_id, category) REFERENCES tag_categories(user_id, name)
);

-- Tag join tables
CREATE TABLE contribution_tags (
    contribution_id UUID NOT NULL REFERENCES contributions(id),
    tag_id UUID NOT NULL REFERENCES tags(id),
    PRIMARY KEY (contribution_id, tag_id)
);

CREATE TABLE education_tags (
    education_id UUID NOT NULL REFERENCES education(id),
    tag_id UUID NOT NULL REFERENCES tags(id),
    PRIMARY KEY (education_id, tag_id)
);

CREATE TABLE credential_tags (
    credential_id UUID NOT NULL REFERENCES credentials(id),
    tag_id UUID NOT NULL REFERENCES tags(id),
    PRIMARY KEY (credential_id, tag_id)
);

-- Projects
CREATE TABLE projects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    name TEXT NOT NULL,
    tagline TEXT,
    role TEXT NOT NULL DEFAULT 'author'
        CHECK (role IN ('author','maintainer','contributor','lead')),
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active','dormant','archived')),
    started_on DATE,
    ended_on DATE,
    repo_url TEXT,
    live_url TEXT,
    writeup_url TEXT,
    force_include BOOL NOT NULL DEFAULT FALSE,
    force_exclude BOOL NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Project join tables
CREATE TABLE project_contributions (
    contribution_id UUID NOT NULL REFERENCES contributions(id),
    project_id UUID NOT NULL REFERENCES projects(id),
    PRIMARY KEY (contribution_id, project_id)
);

CREATE TABLE project_tags (
    project_id UUID NOT NULL REFERENCES projects(id),
    tag_id UUID NOT NULL REFERENCES tags(id),
    PRIMARY KEY (project_id, tag_id)
);

-- Applications
CREATE TABLE applications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    company_name TEXT NOT NULL,
    role_title TEXT NOT NULL,
    jd_url TEXT,
    jd_text TEXT,
    jd_signals JSONB,
    status TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN (
            'draft','applied','screen','interview',
            'offer','rejected','withdrawn'
        )),
    applied_on DATE,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Resume versions
CREATE TABLE resume_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    application_id UUID NOT NULL REFERENCES applications(id),
    version_number INT NOT NULL,
    generation_params JSONB,
    structured_output JSONB NOT NULL,
    generation_notes TEXT,
    submitted BOOL NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (application_id, version_number)
);

-- Contribution feedback
CREATE TABLE contribution_feedback (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    contribution_id UUID NOT NULL REFERENCES contributions(id),
    resume_version_id UUID NOT NULL REFERENCES resume_versions(id),
    accepted_bullets TEXT[],
    rejected_bullets TEXT[],
    edited_deltas JSONB,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Indexes
CREATE INDEX ON employers(user_id);
CREATE INDEX ON positions(user_id);
CREATE INDEX ON positions(employer_id);
CREATE INDEX ON contributions(user_id);
CREATE INDEX ON contributions(position_id);
CREATE INDEX ON education(user_id);
CREATE INDEX ON credentials(user_id);
CREATE INDEX ON projects(user_id);
CREATE INDEX ON applications(user_id);
CREATE INDEX ON applications(status);
CREATE INDEX ON resume_versions(application_id);
CREATE INDEX ON contribution_feedback(contribution_id);
CREATE INDEX ON contribution_tags(tag_id);
CREATE INDEX ON education_tags(tag_id);
CREATE INDEX ON credential_tags(tag_id);
CREATE INDEX ON project_contributions(project_id);
CREATE INDEX ON project_tags(tag_id);
CREATE INDEX ON tags(user_id);
CREATE INDEX ON tag_categories(user_id);
