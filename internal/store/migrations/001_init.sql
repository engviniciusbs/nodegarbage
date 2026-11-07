CREATE TABLE snapshots (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    volume       TEXT NOT NULL UNIQUE,
    scanned_at   INTEGER NOT NULL,
    engine       TEXT NOT NULL,
    duration_ms  INTEGER NOT NULL,
    root_id      INTEGER NOT NULL,
    total_size   INTEGER NOT NULL,
    total_files  INTEGER NOT NULL,
    denied_count INTEGER NOT NULL
);

CREATE TABLE snapshot_nodes (
    snapshot_id  INTEGER NOT NULL REFERENCES snapshots(id) ON DELETE CASCADE,
    node_id      INTEGER NOT NULL,
    parent_id    INTEGER NOT NULL,
    name         TEXT NOT NULL,
    own_size     INTEGER NOT NULL,
    total_size   INTEGER NOT NULL,
    own_files    INTEGER NOT NULL,
    total_files  INTEGER NOT NULL,
    own_latest   INTEGER NOT NULL,
    tree_latest  INTEGER NOT NULL,
    flags        INTEGER NOT NULL,
    PRIMARY KEY (snapshot_id, node_id)
) WITHOUT ROWID;

CREATE TABLE snapshot_top_files (
    snapshot_id INTEGER NOT NULL REFERENCES snapshots(id) ON DELETE CASCADE,
    rank        INTEGER NOT NULL,
    path        TEXT NOT NULL,
    size        INTEGER NOT NULL,
    mod_time    INTEGER NOT NULL,
    PRIMARY KEY (snapshot_id, rank)
) WITHOUT ROWID;

CREATE TABLE snapshot_extensions (
    snapshot_id INTEGER NOT NULL REFERENCES snapshots(id) ON DELETE CASCADE,
    rank        INTEGER NOT NULL,
    ext         TEXT NOT NULL,
    count       INTEGER NOT NULL,
    bytes       INTEGER NOT NULL,
    PRIMARY KEY (snapshot_id, rank)
) WITHOUT ROWID;

CREATE TABLE snapshot_denied (
    snapshot_id INTEGER NOT NULL REFERENCES snapshots(id) ON DELETE CASCADE,
    rank        INTEGER NOT NULL,
    path        TEXT NOT NULL,
    PRIMARY KEY (snapshot_id, rank)
) WITHOUT ROWID;

CREATE TABLE cleanups (
    id      INTEGER PRIMARY KEY AUTOINCREMENT,
    path    TEXT NOT NULL,
    size    INTEGER NOT NULL,
    mode    TEXT NOT NULL,
    source  TEXT NOT NULL,
    rule_id TEXT NOT NULL,
    status  TEXT NOT NULL,
    reason  TEXT NOT NULL,
    freed   INTEGER NOT NULL,
    at      INTEGER NOT NULL
);
CREATE INDEX cleanups_at ON cleanups(at DESC, id DESC);

CREATE TABLE ai_reports (
    key        TEXT PRIMARY KEY,
    provider   TEXT NOT NULL,
    model      TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    body       BLOB NOT NULL
);

CREATE TABLE settings (
    key   TEXT PRIMARY KEY,
    value BLOB NOT NULL
);
