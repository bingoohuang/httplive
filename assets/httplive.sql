-- name: CreateTable
CREATE TABLE IF NOT EXISTS httplive_endpoint
(
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    endpoint    TEXT NOT NULL UNIQUE,
    methods     TEXT NOT NULL,
    mime_type   TEXT,
    filename    TEXT,
    body        TEXT,
    create_time TEXT NOT NULL,
    update_time TEXT NOT NULL,
    deleted_at  TEXT NULL
);

-- name: LastInsertRowID
SELECT last_insert_rowid();

-- name: FindEndpoint
SELECT id,
       endpoint,
       methods,
       mime_type,
       filename,
       body,
       create_time,
       update_time,
       deleted_at
FROM httplive_endpoint
WHERE id = ':1';

-- name: FindByEndpoint
SELECT id,
       endpoint,
       methods,
       mime_type,
       filename,
       body,
       create_time,
       update_time,
       deleted_at
FROM httplive_endpoint
WHERE endpoint = ':1';

-- name: ListEndpoints
SELECT id,
       endpoint,
       methods,
       mime_type,
       filename,
       body,
       create_time,
       update_time,
       deleted_at
FROM httplive_endpoint
WHERE deleted_at = ''
   or deleted_at IS NULL
ORDER BY id;

-- name: AddEndpoint
INSERT INTO httplive_endpoint(endpoint, methods, mime_type, filename, body, create_time, update_time, deleted_at)
VALUES (:endpoint, :methods, :mime_type, :filename, :body, :create_time, :update_time, :deleted_at);

-- name: AddEndpointID
INSERT INTO httplive_endpoint(id, endpoint, methods, mime_type, filename, body, create_time, update_time, deleted_at)
VALUES (:id, :endpoint, :methods, :mime_type, :filename, :body, :create_time, :update_time, :deleted_at);

-- name: UpdateEndpoint
UPDATE httplive_endpoint
SET endpoint    = :endpoint,
    methods     = :methods,
    mime_type   = :mime_type,
    filename    = :filename,
    body        = :body,
    create_time = :create_time,
    update_time = :update_time,
    deleted_at  = :deleted_at
WHERE id = :id;

-- name: DeleteEndpoint
UPDATE httplive_endpoint
SET deleted_at = :deleted_at
WHERE id = :id;
