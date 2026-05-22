import Database from 'better-sqlite3';
import * as path from 'node:path';
import * as fs from 'node:fs';

export interface RegisteredAgent {
  agentId: string;
  agentName: string;
  userId: string;
  agentGroupId: string;
  endpoint: string;
}

const DATA_DIR = path.join(__dirname, '..', 'data');

let db: Database.Database;

function getDb(): Database.Database {
  if (db) return db;
  if (!fs.existsSync(DATA_DIR)) {
    fs.mkdirSync(DATA_DIR, { recursive: true });
  }
  db = new Database(path.join(DATA_DIR, 'agents.db'));
  db.pragma('journal_mode = WAL');
  db.exec(`
    CREATE TABLE IF NOT EXISTS agent_registry (
      agent_id       TEXT PRIMARY KEY,
      agent_name     TEXT NOT NULL,
      user_id        TEXT NOT NULL,
      agent_group_id TEXT NOT NULL DEFAULT '',
      endpoint       TEXT NOT NULL DEFAULT '',
      created_at     TEXT NOT NULL DEFAULT (datetime('now'))
    );
    CREATE INDEX IF NOT EXISTS idx_agent_user ON agent_registry(user_id);
    CREATE INDEX IF NOT EXISTS idx_agent_group ON agent_registry(agent_group_id);
  `);
  // Migration: add endpoint column if missing
  const cols = db.prepare("PRAGMA table_info(agent_registry)").all() as { name: string }[];
  if (!cols.some(c => c.name === 'endpoint')) {
    db.exec("ALTER TABLE agent_registry ADD COLUMN endpoint TEXT NOT NULL DEFAULT ''");
  }
  return db;
}

export function register(agentId: string, agentName: string, userId: string, agentGroupId = '', endpoint = ''): void {
  getDb().prepare(
    'INSERT OR REPLACE INTO agent_registry (agent_id, agent_name, user_id, agent_group_id, endpoint) VALUES (?, ?, ?, ?, ?)'
  ).run(agentId, agentName, userId, agentGroupId, endpoint);
}

export function findByAgentId(agentId: string): RegisteredAgent | undefined {
  const row = getDb().prepare(
    'SELECT agent_id AS agentId, agent_name AS agentName, user_id AS userId, agent_group_id AS agentGroupId, endpoint FROM agent_registry WHERE agent_id = ?'
  ).get(agentId) as RegisteredAgent | undefined;
  return row;
}

export function findByUserId(userId: string): RegisteredAgent[] {
  return getDb().prepare(
    'SELECT agent_id AS agentId, agent_name AS agentName, user_id AS userId, agent_group_id AS agentGroupId, endpoint FROM agent_registry WHERE user_id = ?'
  ).all(userId) as RegisteredAgent[];
}

export function findGroupMembers(agentGroupId: string): string[] {
  if (!agentGroupId) return [];
  const rows = getDb().prepare(
    'SELECT agent_id FROM agent_registry WHERE agent_group_id = ?'
  ).all(agentGroupId) as { agent_id: string }[];
  return rows.map(r => r.agent_id);
}

export function remove(agentId: string): boolean {
  const result = getDb().prepare('DELETE FROM agent_registry WHERE agent_id = ?').run(agentId);
  return result.changes > 0;
}

export function verifyOwnership(agentId: string, userId: string): boolean {
  const row = getDb().prepare(
    'SELECT 1 FROM agent_registry WHERE agent_id = ? AND user_id = ?'
  ).get(agentId, userId);
  return !!row;
}
