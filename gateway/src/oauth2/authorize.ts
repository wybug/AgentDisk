import type { Request, Response } from 'express';
import * as codeStore from '../store/codes.js';
import * as userStore from '../store/users.js';
import { generateCode } from './pkce.js';
import type { OAuth2Client, SessionUser } from './types.js';

const CLIENT: OAuth2Client = {
  clientId: 'agentdisk',
  clientSecret: 'agentdisk-secret',
  redirectUris: [
    'http://localhost:9101/auth/callback',
    'http://localhost:9100/auth/callback',
  ],
};

export function handleAuthorize(req: Request, res: Response): void {
  const {
    response_type,
    client_id,
    redirect_uri,
    scope = '',
    state = '',
    code_challenge,
    code_challenge_method,
    prompt,
  } = req.query as Record<string, string>;

  // 验证 client_id
  if (client_id !== CLIENT.clientId) {
    res.status(400).json({ error: 'invalid_client', message: 'Unknown client_id' });
    return;
  }

  // 验证 redirect_uri
  if (!redirect_uri || !CLIENT.redirectUris.includes(redirect_uri)) {
    res.status(400).json({ error: 'invalid_request', message: 'Invalid redirect_uri' });
    return;
  }

  // 验证 response_type
  if (response_type !== 'code') {
    const url = new URL(redirect_uri);
    url.searchParams.set('error', 'unsupported_response_type');
    if (state) url.searchParams.set('state', state);
    res.redirect(url.toString());
    return;
  }

  // 获取当前登录用户（从 session cookie）
  const sessionUser: SessionUser | undefined = (req as any).sessionUser;

  // prompt=none：无感跳转模式
  if (prompt === 'none') {
    if (!sessionUser) {
      const url = new URL(redirect_uri);
      url.searchParams.set('error', 'login_required');
      if (state) url.searchParams.set('state', state);
      res.redirect(url.toString());
      return;
    }
    // 自动批准
    issueCodeAndRedirect(res, sessionUser.userId, redirect_uri, scope, state, code_challenge, code_challenge_method);
    return;
  }

  // 未登录 → 跳转到登录页
  if (!sessionUser) {
    const returnTo = encodeURIComponent(req.originalUrl);
    res.redirect(`/login?returnTo=${returnTo}`);
    return;
  }

  // 已登录但需要确认 → 显示授权确认页
  // 保存授权请求参数到 cookie 用于确认后使用
  res.cookie('oauth2_pending', JSON.stringify({
    redirect_uri,
    scope,
    state,
    code_challenge,
    code_challenge_method,
    userId: sessionUser.userId,
  }), { maxAge: 600000, httpOnly: false });

  res.redirect('/authorize');
}

export function handleAuthorizeApprove(req: Request, res: Response): void {
  const sessionUser: SessionUser | undefined = (req as any).sessionUser;
  if (!sessionUser) {
    res.redirect('/login');
    return;
  }

  const { redirect_uri, scope, state, code_challenge, code_challenge_method, userId } =
    req.body as Record<string, string>;

  if (userId !== sessionUser.userId) {
    res.status(403).json({ error: 'forbidden' });
    return;
  }

  issueCodeAndRedirect(res, userId, redirect_uri, scope, state, code_challenge, code_challenge_method);
}

function issueCodeAndRedirect(
  res: Response,
  userId: string,
  redirectUri: string,
  scope: string,
  state: string,
  codeChallenge: string,
  codeChallengeMethod: string,
): void {
  const code = generateCode();
  codeStore.store({
    code,
    clientId: 'agentdisk',
    redirectUri,
    userId,
    codeChallenge: codeChallenge || '',
    codeChallengeMethod: codeChallengeMethod || '',
    scope,
    expiresAt: Date.now() + 10 * 60 * 1000, // 10 minutes
  });

  const url = new URL(redirectUri);
  url.searchParams.set('code', code);
  if (state) url.searchParams.set('state', state);
  res.clearCookie('oauth2_pending');
  res.redirect(url.toString());
}
