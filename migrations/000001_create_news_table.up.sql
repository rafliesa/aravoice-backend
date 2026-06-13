CREATE TABLE news (
    id SERIAL PRIMARY KEY,
    slug TEXT NOT NULL UNIQUE,
    category TEXT NOT NULL,
    title TEXT NOT NULL,
    excerpt TEXT NOT NULL,
    body TEXT NOT NULL,
    author TEXT NOT NULL,
    reading_time INTEGER NOT NULL CHECK (reading_time >= 0),
    cover_image TEXT NOT NULL,
    caption TEXT NOT NULL,
    formats TEXT[] NOT NULL DEFAULT '{}',
    published_at TIMESTAMPTZ NOT NULL,
    is_published BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_news_published
    ON news (published_at DESC)
    WHERE is_published = TRUE;

CREATE INDEX idx_news_category ON news (category);
