CREATE TABLE remote_servers (
    name TEXT PRIMARY KEY,
    hostport TEXT NOT NULL,
    secret TEXT NOT NULL
);

CREATE TABLE users (
    name TEXT PRIMARY KEY,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    superuser BOOLEAN NOT NULL DEFAULT FALSE,
    totp_secret TEXT NOT NULL
);