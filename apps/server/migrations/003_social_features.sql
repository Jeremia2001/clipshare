-- +goose Up
-- +goose StatementBegin
CREATE TABLE comments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    clip_id UUID REFERENCES clips(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    parent_id UUID REFERENCES comments(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    is_edited BOOLEAN DEFAULT false,
    edited_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_comments_clip_id ON comments(clip_id);
CREATE INDEX idx_comments_user_id ON comments(user_id);
CREATE INDEX idx_comments_parent_id ON comments(parent_id);

CREATE TABLE clip_reactions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    clip_id UUID REFERENCES clips(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    reaction VARCHAR(32) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(clip_id, user_id, reaction)
);

CREATE INDEX idx_clip_reactions_clip_id ON clip_reactions(clip_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS clip_reactions;
DROP TABLE IF EXISTS comments;
-- +goose StatementEnd
