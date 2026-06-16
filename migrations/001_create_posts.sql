CREATE TABLE IF NOT EXISTS posts (
    id         SERIAL PRIMARY KEY,
    title      TEXT        NOT NULL,
    content    TEXT        NOT NULL,
    author_id  TEXT        NOT NULL,
    status     TEXT        NOT NULL DEFAULT 'draft'
                           CHECK (status IN ('draft', 'pending_review', 'published')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_posts_author_id ON posts (author_id);
CREATE INDEX IF NOT EXISTS idx_posts_status    ON posts (status);
