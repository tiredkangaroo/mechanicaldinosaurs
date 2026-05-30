-- name: ListRemoteServers :many
SELECT * FROM remote_servers;

-- name: GetRemoteServer :one
SELECT * FROM remote_servers WHERE name = ?;

-- name: AddRemoteServer :exec
INSERT INTO remote_servers (name, hostport, secret) VALUES (?, ?, ?);

-- name: UpdateRemoteServer :exec
UPDATE remote_servers SET name = ?, hostport = ?, secret = ? WHERE name = ?;

-- name: DeleteRemoteServer :exec
DELETE FROM remote_servers WHERE name = ?;