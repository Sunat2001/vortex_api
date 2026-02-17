-- СИСТЕМА ПРАВ И ДОСТУПА (RBAC)

CREATE TABLE permissions
(
    id          UUID PRIMARY KEY         DEFAULT gen_random_uuid(),
    slug        VARCHAR(100) UNIQUE NOT NULL,
    module      VARCHAR(50)         NOT NULL,
    description TEXT,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE roles
(
    id         UUID PRIMARY KEY         DEFAULT gen_random_uuid(),
    name       VARCHAR(100)        NOT NULL,
    slug       VARCHAR(100) UNIQUE NOT NULL,
    is_system  BOOLEAN                  DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE role_permissions
(
    role_id       UUID REFERENCES roles (id) ON DELETE CASCADE,
    permission_id UUID REFERENCES permissions (id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE users
(
    id         UUID PRIMARY KEY         DEFAULT gen_random_uuid(),
    email      VARCHAR(255) UNIQUE NOT NULL,
    full_name  VARCHAR(255),
    status     VARCHAR(50)              DEFAULT 'offline',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE user_roles
(
    user_id UUID REFERENCES users (id) ON DELETE CASCADE,
    role_id UUID REFERENCES roles (id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, role_id)
);