package persistence

const hcdSchema = `CREATE TABLE IF NOT EXISTS hcd_units (
  project TEXT NOT NULL,
  id TEXT NOT NULL,
  kind TEXT NOT NULL,
  title TEXT NOT NULL,
  slug TEXT NOT NULL,
  approach TEXT NOT NULL,
  state TEXT NOT NULL,
  prev_state TEXT NOT NULL DEFAULT '',
  state_revision INTEGER NOT NULL DEFAULT 0,
  definition_rev TEXT NOT NULL DEFAULT '',
  delivery_rev TEXT NOT NULL DEFAULT '',
  approved_rev TEXT NOT NULL DEFAULT '',
  playbook_rev TEXT NOT NULL DEFAULT '',
  constitution_rev TEXT NOT NULL DEFAULT '',
  implementer TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (project, id)
);
CREATE TABLE IF NOT EXISTS hcd_operations (
  project TEXT NOT NULL,
  operation_id TEXT NOT NULL,
  unit_id TEXT NOT NULL,
  command TEXT NOT NULL,
  payload_hash TEXT NOT NULL,
  result_json TEXT NOT NULL,
  created_at TEXT NOT NULL,
  PRIMARY KEY (project, operation_id)
);
CREATE TABLE IF NOT EXISTS hcd_approval_prompts (
  project TEXT NOT NULL, id TEXT NOT NULL, unit_id TEXT NOT NULL,
  object TEXT NOT NULL, revision TEXT NOT NULL, options_json TEXT NOT NULL,
  asked_at TEXT NOT NULL, consumed INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (project, id)
);
CREATE INDEX IF NOT EXISTS idx_hcd_approval_prompts_unit_object ON hcd_approval_prompts(project, unit_id, object);
CREATE TABLE IF NOT EXISTS hcd_approvals (
  project TEXT NOT NULL, id TEXT NOT NULL, unit_id TEXT NOT NULL,
  object TEXT NOT NULL, revision TEXT NOT NULL, origin TEXT NOT NULL,
  quote TEXT NOT NULL, client TEXT NOT NULL, approved_at TEXT NOT NULL,
  PRIMARY KEY (project, id)
);
CREATE INDEX IF NOT EXISTS idx_hcd_approvals_unit_object ON hcd_approvals(project, unit_id, object);
CREATE TABLE IF NOT EXISTS hcd_clarifications (
  project TEXT NOT NULL, id TEXT NOT NULL, unit_id TEXT NOT NULL,
  target_ref TEXT NOT NULL, question TEXT NOT NULL, options_json TEXT NOT NULL DEFAULT '[]',
  recommendation TEXT NOT NULL DEFAULT '', answer TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL, blocking INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (project, id)
);
CREATE INDEX IF NOT EXISTS idx_hcd_clarifications_unit_status ON hcd_clarifications(project, unit_id, status);
CREATE TABLE IF NOT EXISTS hcd_decisions (
  project TEXT NOT NULL, id TEXT NOT NULL, unit_id TEXT NOT NULL,
  kind TEXT NOT NULL, status TEXT NOT NULL, summary TEXT NOT NULL,
  revision TEXT NOT NULL DEFAULT '', refs_json TEXT NOT NULL DEFAULT '[]',
  source TEXT NOT NULL DEFAULT '', memory_id INTEGER, memory_sync TEXT NOT NULL DEFAULT 'pending',
  PRIMARY KEY (project, id)
);
CREATE INDEX IF NOT EXISTS idx_hcd_decisions_unit ON hcd_decisions(project, unit_id);
CREATE INDEX IF NOT EXISTS idx_hcd_decisions_memory_sync ON hcd_decisions(project, memory_sync);
CREATE TABLE IF NOT EXISTS hcd_exceptions (
  project TEXT NOT NULL, id TEXT NOT NULL, unit_id TEXT NOT NULL,
  rule_id TEXT NOT NULL, scope TEXT NOT NULL, reason TEXT NOT NULL,
  alternatives TEXT NOT NULL, risks TEXT NOT NULL, compensating TEXT NOT NULL,
  owner TEXT NOT NULL, approval_id TEXT NOT NULL, status TEXT NOT NULL,
  expires_at TEXT NOT NULL DEFAULT '', review_condition TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (project, id)
);
CREATE INDEX IF NOT EXISTS idx_hcd_exceptions_unit_status ON hcd_exceptions(project, unit_id, status);
CREATE TABLE IF NOT EXISTS hcd_blocks (
  project TEXT NOT NULL, id TEXT NOT NULL, unit_id TEXT NOT NULL,
  target_ref TEXT NOT NULL, reason TEXT NOT NULL, critical INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL, resolution TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (project, id)
);
CREATE INDEX IF NOT EXISTS idx_hcd_blocks_unit_status ON hcd_blocks(project, unit_id, status);
CREATE TABLE IF NOT EXISTS hcd_executions (
  project TEXT NOT NULL, id TEXT NOT NULL, unit_id TEXT NOT NULL, task_id TEXT NOT NULL,
  definition_rev TEXT NOT NULL, delivery_rev TEXT NOT NULL, state TEXT NOT NULL,
  granted_at TEXT NOT NULL, reported_at TEXT NOT NULL DEFAULT '', result_hash TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (project, id)
);
CREATE INDEX IF NOT EXISTS idx_hcd_executions_unit_task ON hcd_executions(project, unit_id, task_id);
CREATE TABLE IF NOT EXISTS hcd_evidence (
  project TEXT NOT NULL, id TEXT NOT NULL, unit_id TEXT NOT NULL,
  execution_id TEXT NOT NULL DEFAULT '', covers_json TEXT NOT NULL DEFAULT '[]',
  procedure TEXT NOT NULL, environment TEXT NOT NULL, result TEXT NOT NULL,
  subject_ref TEXT NOT NULL, inspect_ref TEXT NOT NULL DEFAULT '',
  definition_rev TEXT NOT NULL, delivery_rev TEXT NOT NULL,
  playbook_rev TEXT NOT NULL, constitution_rev TEXT NOT NULL,
  executor TEXT NOT NULL, recorded_at TEXT NOT NULL,
  PRIMARY KEY (project, id)
);
CREATE INDEX IF NOT EXISTS idx_hcd_evidence_unit ON hcd_evidence(project, unit_id);
CREATE TABLE IF NOT EXISTS hcd_links (
  project TEXT NOT NULL, unit_id TEXT NOT NULL,
  from_kind TEXT NOT NULL, from_id TEXT NOT NULL,
  to_kind TEXT NOT NULL, to_id TEXT NOT NULL, relation TEXT NOT NULL,
  created_at TEXT NOT NULL,
  UNIQUE (project, unit_id, from_kind, from_id, to_kind, to_id, relation)
);
CREATE TABLE IF NOT EXISTS hcd_normative_revisions (
  project TEXT NOT NULL, doc TEXT NOT NULL, revision TEXT NOT NULL,
  content TEXT NOT NULL, first_pinned_at TEXT NOT NULL,
  source TEXT NOT NULL, change_kind TEXT NOT NULL, urgent INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (project, doc, revision)
);
CREATE TABLE IF NOT EXISTS hcd_focus (
  project TEXT NOT NULL PRIMARY KEY, unit_id TEXT NOT NULL,
  client TEXT NOT NULL, session_id TEXT NOT NULL,
  set_at TEXT NOT NULL, set_by TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS hcd_heartbeats (
  project TEXT NOT NULL, id TEXT NOT NULL, unit_id TEXT NOT NULL,
  session_id TEXT NOT NULL, client TEXT NOT NULL, stage TEXT NOT NULL,
  state_revision INTEGER NOT NULL, files_json TEXT NOT NULL DEFAULT '[]',
  drift_json TEXT NOT NULL DEFAULT '[]', seen_at TEXT NOT NULL,
  PRIMARY KEY (project, id)
);
CREATE INDEX IF NOT EXISTS idx_hcd_heartbeats_unit_seen ON hcd_heartbeats(project, unit_id, seen_at);
CREATE TABLE IF NOT EXISTS hcd_playbook_adoptions (
  project TEXT NOT NULL, id TEXT NOT NULL, unit_id TEXT NOT NULL,
  doc TEXT NOT NULL, from_rev TEXT NOT NULL, to_rev TEXT NOT NULL,
  impact_json TEXT NOT NULL, reason TEXT NOT NULL, actor TEXT NOT NULL,
  approval_id TEXT NOT NULL, adopted_at TEXT NOT NULL,
  PRIMARY KEY (project, id)
);
CREATE INDEX IF NOT EXISTS idx_hcd_playbook_adoptions_unit_doc ON hcd_playbook_adoptions(project, unit_id, doc);
CREATE TABLE IF NOT EXISTS hcd_migrations (
  project TEXT NOT NULL, id TEXT NOT NULL, unit_id TEXT NOT NULL DEFAULT '',
  source_dir TEXT NOT NULL, source_rev TEXT NOT NULL,
  proposal_json TEXT NOT NULL, status TEXT NOT NULL,
  created_at TEXT NOT NULL, activated_at TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (project, id)
);
CREATE INDEX IF NOT EXISTS idx_hcd_migrations_source_dir ON hcd_migrations(project, source_dir);
`
