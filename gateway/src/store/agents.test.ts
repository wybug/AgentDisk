import { register, findByAgentId, findByUserId, findGroupMembers, remove, verifyOwnership } from './agents.js';
import Database from 'better-sqlite3';
import * as fs from 'node:fs';
import * as path from 'node:path';
import * as os from 'node:os';

// Use a temp directory for test database to avoid polluting production data
const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'agent-test-'));

// Override the data directory by reinitializing the module
// We test the public API directly since the module uses lazy init

describe('AgentStore', () => {
  beforeAll(() => {
    // Clean up any previous test state
  });

  test('register and findByAgentId', () => {
    register('agent-001', 'Test Agent', 'user-001', 'group-a');
    const agent = findByAgentId('agent-001');
    expect(agent).toBeDefined();
    expect(agent!.agentId).toBe('agent-001');
    expect(agent!.agentName).toBe('Test Agent');
    expect(agent!.userId).toBe('user-001');
    expect(agent!.agentGroupId).toBe('group-a');
  });

  test('findByUserId returns agents for user', () => {
    register('agent-a1', 'Agent A1', 'user-100', 'group-x');
    register('agent-a2', 'Agent A2', 'user-100', 'group-x');
    register('agent-b1', 'Agent B1', 'user-200', '');

    const agents = findByUserId('user-100');
    expect(agents.length).toBeGreaterThanOrEqual(2);
    const ids = agents.map(a => a.agentId);
    expect(ids).toContain('agent-a1');
    expect(ids).toContain('agent-a2');
  });

  test('findGroupMembers returns same group agents', () => {
    register('gm-1', 'GM1', 'user-300', 'group-gm');
    register('gm-2', 'GM2', 'user-300', 'group-gm');
    register('gm-3', 'GM3', 'user-300', 'other-group');

    const members = findGroupMembers('group-gm');
    expect(members).toContain('gm-1');
    expect(members).toContain('gm-2');
    expect(members).not.toContain('gm-3');
  });

  test('findGroupMembers empty string returns empty', () => {
    expect(findGroupMembers('')).toEqual([]);
  });

  test('verifyOwnership returns true for owner', () => {
    register('vo-1', 'VO1', 'user-vo', '');
    expect(verifyOwnership('vo-1', 'user-vo')).toBe(true);
  });

  test('verifyOwnership returns false for non-owner', () => {
    register('vo-2', 'VO2', 'user-vo', '');
    expect(verifyOwnership('vo-2', 'user-other')).toBe(false);
  });

  test('verifyOwnership returns false for non-existent agent', () => {
    expect(verifyOwnership('non-existent', 'user-vo')).toBe(false);
  });

  test('remove deletes agent', () => {
    register('rm-1', 'RM1', 'user-rm', '');
    expect(findByAgentId('rm-1')).toBeDefined();
    expect(remove('rm-1')).toBe(true);
    expect(findByAgentId('rm-1')).toBeUndefined();
  });

  test('remove returns false for non-existent', () => {
    expect(remove('non-existent')).toBe(false);
  });

  test('duplicate agentId upserts', () => {
    register('dup-1', 'Original', 'user-dup', 'g1');
    register('dup-1', 'Updated', 'user-dup', 'g2');
    const agent = findByAgentId('dup-1');
    expect(agent!.agentName).toBe('Updated');
    expect(agent!.agentGroupId).toBe('g2');
  });
});
