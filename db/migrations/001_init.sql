CREATE TABLE polls (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
)

CREATE TABLE candidates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    poll_id TEXT NOT NULL,
    name TEXT NOT NULL,
    FOREIGN KEY(poll_id) REFERENCES polls(id)
)

CREATE TABLE ballots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    poll_id TEXT NOT NULL,
    voter_id TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(poll_id) REFERENCES polls(id),
)

CREATE TABLE ballot_rankings (
    ballot_id INTEGER NOT NULL,
    candidate_id INTEGER NOT NULL,
    rank INTEGER NOT NULL,
    PRIMARY KEY (ballot_id, candidate_id),
    FOREIGN KEY (ballot_id) REFERENCES ballots(id),
    FOREIGN KEY (candidate_id) REFERENCES candidates(id)
)
