CREATE TABLE todos (
    id UUID PRIMARY KEY,
    title VARCHAR(200) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    completed BOOLEAN NOT NULL DEFAULT FALSE,
    due_date TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT todos_title_not_blank CHECK (char_length(btrim(title)) > 0),
    CONSTRAINT todos_description_length CHECK (char_length(description) <= 2000)
);

CREATE INDEX todos_completed_idx ON todos (completed);
CREATE INDEX todos_created_at_idx ON todos (created_at);
CREATE INDEX todos_due_date_idx ON todos (due_date);
CREATE INDEX todos_title_idx ON todos (title);
