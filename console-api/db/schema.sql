CREATE TABLE remote_servers (
    name TEXT PRIMARY KEY,
    hostport TEXT NOT NULL,
    secret TEXT NOT NULL
);