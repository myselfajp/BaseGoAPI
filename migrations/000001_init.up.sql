CREATE TABLE users (
    id                    BIGSERIAL PRIMARY KEY,
    email                 VARCHAR(255) NOT NULL,
    password_hash         VARCHAR(255) NOT NULL,
    role                  VARCHAR(50)  NOT NULL,
    is_active             BOOLEAN      NOT NULL DEFAULT TRUE,
    full_name             VARCHAR(255) NOT NULL DEFAULT '',
    phone_number          VARCHAR(20)  NOT NULL DEFAULT '',
    is_email_verified     BOOLEAN      NOT NULL DEFAULT FALSE,
    is_two_factor_enabled BOOLEAN      NOT NULL DEFAULT FALSE,
    failed_login_attempts INTEGER      NOT NULL DEFAULT 0,
    last_failed_login     TIMESTAMPTZ,
    created_at            TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX idx_users_email ON users (email);

CREATE TABLE email_verification_tokens (
    id         VARCHAR(36) PRIMARY KEY,
    user_id    BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token      VARCHAR(255) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX idx_evt_token ON email_verification_tokens (token);
CREATE INDEX idx_evt_user_id ON email_verification_tokens (user_id);

CREATE TABLE login_otps (
    id         VARCHAR(36) PRIMARY KEY,
    user_id    BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    code_hash  VARCHAR(255) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_login_otps_user_id ON login_otps (user_id);

CREATE TABLE password_reset_tokens (
    id         VARCHAR(36) PRIMARY KEY,
    user_id    BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token      VARCHAR(255) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX idx_prt_token ON password_reset_tokens (token);
CREATE INDEX idx_prt_user_id ON password_reset_tokens (user_id);
