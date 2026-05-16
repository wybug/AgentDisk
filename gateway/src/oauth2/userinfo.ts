import type { Request, Response } from 'express';
import * as tokenStore from '../store/tokens.js';

export function handleUserInfo(req: Request, res: Response): void {
  const auth = req.headers.authorization;
  if (!auth || !auth.startsWith('Bearer ')) {
    res.status(401).json({ error: 'invalid_token', message: 'Missing or invalid Authorization header' });
    return;
  }

  const accessToken = auth.slice(7);
  const entry = tokenStore.findByAccessToken(accessToken);
  if (!entry) {
    res.status(401).json({ error: 'invalid_token', message: 'Token expired or invalid' });
    return;
  }

  res.json({
    userId: entry.userId,
    userName: entry.userName,
  });
}
