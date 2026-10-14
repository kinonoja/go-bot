CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email text UNIQUE NOT NULL,
    display_name text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE submissions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id),
    course_slug text NOT NULL,
    chapter_slug text NOT NULL,
    lesson_slug text NOT NULL,
    status text NOT NULL,
    passed boolean NOT NULL,
    missing_tests text[] NOT NULL DEFAULT '{}',
    duration_ms integer NOT NULL,
    code text NOT NULL CHECK (char_length(code) <= 100000),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE progress (
    user_id uuid NOT NULL REFERENCES users(id),
    course_slug text NOT NULL,
    chapter_slug text NOT NULL,
    lesson_slug text NOT NULL,
    completed_at timestamptz NOT NULL DEFAULT now(),
    attempts integer NOT NULL,
    xp_awarded integer NOT NULL,
    PRIMARY KEY (user_id, course_slug, chapter_slug, lesson_slug)
);

CREATE TABLE xp_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id),
    course_slug text NOT NULL,
    chapter_slug text NOT NULL,
    lesson_slug text NOT NULL,
    amount integer NOT NULL,
    reason text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX submissions_user_created_idx ON submissions (user_id, created_at DESC);
CREATE INDEX progress_user_idx ON progress (user_id);
CREATE INDEX xp_events_user_created_idx ON xp_events (user_id, created_at DESC);
