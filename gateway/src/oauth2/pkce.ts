import * as crypto from 'node:crypto';

export function verifyPKCE(codeVerifier: string, codeChallenge: string, method: string): boolean {
  if (method !== 'S256') return false;
  const hash = crypto.createHash('sha256').update(codeVerifier).digest('base64url');
  return hash === codeChallenge;
}

export function generateCode(): string {
  return crypto.randomBytes(24).toString('base64url');
}

export function generateToken(): string {
  return 'gw_' + crypto.randomBytes(32).toString('base64url');
}

export function generateRefreshToken(): string {
  return 'gw_r_' + crypto.randomBytes(32).toString('base64url');
}

export function generateSessionId(): string {
  return crypto.randomBytes(32).toString('base64url');
}
