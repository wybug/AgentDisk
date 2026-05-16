import type { Request, Response } from 'express';
import * as codeStore from '../store/codes.js';
import * as tokenStore from '../store/tokens.js';
import * as userStore from '../store/users.js';
import { verifyPKCE, generateToken, generateRefreshToken } from './pkce.js';

export function handleToken(req: Request, res: Response): void {
  const { grant_type, code, redirect_uri, code_verifier } = req.body as Record<string, string>;

  if (grant_type !== 'authorization_code') {
    res.status(400).json({ error: 'unsupported_grant_type' });
    return;
  }

  if (!code) {
    res.status(400).json({ error: 'invalid_request', message: 'Missing code' });
    return;
  }

  const entry = codeStore.consume(code);
  if (!entry) {
    res.status(400).json({ error: 'invalid_grant', message: 'Invalid or expired authorization code' });
    return;
  }

  // 验证 redirect_uri
  if (redirect_uri && redirect_uri !== entry.redirectUri) {
    res.status(400).json({ error: 'invalid_grant', message: 'redirect_uri mismatch' });
    return;
  }

  // 验证 PKCE
  if (entry.codeChallenge) {
    if (!code_verifier) {
      res.status(400).json({ error: 'invalid_request', message: 'Missing code_verifier' });
      return;
    }
    if (!verifyPKCE(code_verifier, entry.codeChallenge, entry.codeChallengeMethod)) {
      res.status(400).json({ error: 'invalid_grant', message: 'PKCE verification failed' });
      return;
    }
  }

  const user = userStore.findById(entry.userId);
  if (!user) {
    res.status(400).json({ error: 'invalid_grant', message: 'User not found' });
    return;
  }

  const accessToken = generateToken();
  const refreshToken = generateRefreshToken();
  const expiresIn = 86400;

  tokenStore.store({
    accessToken,
    tokenType: 'Bearer',
    expiresIn,
    refreshToken,
    userId: user.userId,
    userName: user.userName,
    scope: entry.scope,
    expiresAt: Date.now() + expiresIn * 1000,
  });

  res.json({
    access_token: accessToken,
    token_type: 'Bearer',
    expires_in: expiresIn,
    refresh_token: refreshToken,
  });
}
