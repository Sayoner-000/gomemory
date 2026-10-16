package persistence

const socraticaSchema = `CREATE TABLE IF NOT EXISTS socratica_flows (
  id TEXT PRIMARY KEY,
  schema_version INTEGER NOT NULL DEFAULT 1,
  project TEXT NOT NULL,
  client TEXT NOT NULL,
  mode TEXT NOT NULL,
  original_prompt TEXT NOT NULL,
  questions_json TEXT NOT NULL DEFAULT '[]',
  answers_json TEXT NOT NULL DEFAULT '[]',
  proposed_prompt TEXT NOT NULL DEFAULT '',
  approved_prompt TEXT NOT NULL DEFAULT '',
  scrutiny_json TEXT,
  phase TEXT NOT NULL,
  last_stable_phase TEXT NOT NULL,
  revision INTEGER NOT NULL DEFAULT 0,
  dispatch_id TEXT,
  dispatch_ack INTEGER NOT NULL DEFAULT 0,
  dispatch_consumed INTEGER NOT NULL DEFAULT 0,
  dispatch_claimed_at TEXT,
  dispatch_claim_owner TEXT,
  ack_nonce TEXT,
  approved_hash TEXT,
  last_error TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_socratica_dispatch ON socratica_flows(dispatch_id) WHERE dispatch_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_socratica_flows_scope ON socratica_flows(project, client, updated_at);
-- Un solo flujo no terminal por (project, client) (F2):
CREATE UNIQUE INDEX IF NOT EXISTS idx_socratica_one_pending ON socratica_flows(project, client) WHERE phase NOT IN ('dispatched','cancelled');

CREATE TABLE IF NOT EXISTS socratica_history (
  id TEXT PRIMARY KEY,
  flow_id TEXT NOT NULL UNIQUE,
  project TEXT NOT NULL,
  client TEXT NOT NULL,
  mode TEXT NOT NULL,
  original_prompt TEXT NOT NULL,
  proposed_prompt TEXT NOT NULL DEFAULT '',
  approved_prompt TEXT,
  scrutiny_json TEXT,
  additions_json TEXT NOT NULL DEFAULT '[]',
  words_before INTEGER NOT NULL,
  words_after INTEGER NOT NULL,
  completed_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_socratica_history_scope ON socratica_history(project, client, completed_at DESC);
`
