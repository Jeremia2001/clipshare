-- +goose Up
-- +goose StatementBegin
CREATE TABLE instance_config (
    id INTEGER PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    instance_name VARCHAR(64) DEFAULT 'ClipShare',
    instance_url VARCHAR(255),
    allow_signups BOOLEAN DEFAULT true,
    max_clip_duration_seconds INTEGER DEFAULT 300,
    default_clip_lifetime_days INTEGER,
    max_upload_size_bytes BIGINT DEFAULT 1073741824,
    require_email_verification BOOLEAN DEFAULT true,
    custom_domains_enabled BOOLEAN DEFAULT false,
    wildcard_domain VARCHAR(255),
    updated_at TIMESTAMP DEFAULT NOW()
);

INSERT INTO instance_config (instance_name) VALUES ('ClipShare');

CREATE TABLE clips (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    rustfs_bucket VARCHAR(64) NOT NULL DEFAULT 'clips',
    rustfs_object_key VARCHAR(512) NOT NULL,
    original_filename VARCHAR(255),
    file_size_bytes BIGINT NOT NULL,
    duration_seconds INTEGER NOT NULL,
    width INTEGER NOT NULL,
    height INTEGER NOT NULL,
    fps INTEGER,
    bitrate_kbps INTEGER,
    thumbnail_key VARCHAR(512),
    processed_variant_keys TEXT[],
    codec VARCHAR(32),
    is_public BOOLEAN DEFAULT true,
    allow_comments BOOLEAN DEFAULT true,
    expires_at TIMESTAMP,
    view_count INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_clips_user_id ON clips(user_id);
CREATE INDEX idx_clips_created_at ON clips(created_at DESC);
CREATE INDEX idx_clips_expires_at ON clips(expires_at) WHERE expires_at IS NOT NULL;

CREATE TABLE shares (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    clip_id UUID REFERENCES clips(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    share_code VARCHAR(16) UNIQUE NOT NULL,
    custom_slug VARCHAR(64) UNIQUE,
    password_hash VARCHAR(255),
    expires_at TIMESTAMP,
    max_views INTEGER,
    view_count INTEGER DEFAULT 0,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_shares_clip_id ON shares(clip_id);
CREATE INDEX idx_shares_share_code ON shares(share_code);
CREATE INDEX idx_shares_custom_slug ON shares(custom_slug);

CREATE TABLE clip_views (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    clip_id UUID REFERENCES clips(id) ON DELETE CASCADE,
    share_id UUID REFERENCES shares(id) ON DELETE SET NULL,
    viewer_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    viewer_ip INET,
    user_agent TEXT,
    country_code VARCHAR(2),
    referrer TEXT,
    watched_seconds INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_clip_views_clip_id ON clip_views(clip_id);
CREATE INDEX idx_clip_views_created_at ON clip_views(created_at);

CREATE TABLE custom_domains (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    domain VARCHAR(255) NOT NULL,
    status VARCHAR(20) DEFAULT 'pending',
    verification_method VARCHAR(20) DEFAULT 'cname',
    dns_record VARCHAR(255),
    verified_at TIMESTAMP,
    certificate_expires_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(domain)
);

CREATE INDEX idx_custom_domains_user_id ON custom_domains(user_id);
CREATE INDEX idx_custom_domains_domain ON custom_domains(domain);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS custom_domains;
DROP TABLE IF EXISTS clip_views;
DROP TABLE IF EXISTS shares;
DROP TABLE IF EXISTS clips;
DROP TABLE IF EXISTS instance_config;
-- +goose StatementEnd
