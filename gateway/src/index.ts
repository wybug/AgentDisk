import express from 'express';
import cookieParser from 'cookie-parser';
import * as path from 'node:path';
import * as fs from 'node:fs';
import * as http from 'node:http';
import jwt from 'jsonwebtoken';

import * as userStore from './store/users.js';
import * as authorize from './oauth2/authorize.js';
import * as token from './oauth2/token.js';
import * as userinfo from './oauth2/userinfo.js';
import { generateSessionId } from './oauth2/pkce.js';
import type { SessionUser } from './oauth2/types.js';

const app = express();
const PORT = 3100;

app.use(express.json());
app.use(express.urlencoded({ extended: true }));
app.use(cookieParser());

// Session middleware
const sessions = new Map<string, SessionUser>();

function sessionMiddleware(req: express.Request, _res: express.Response, next: express.NextFunction): void {
  const sid = req.cookies?.gw_session;
  if (sid) {
    const user = sessions.get(sid);
    if (user) {
      (req as any).sessionUser = user;
    }
  }
  next();
}

app.use(sessionMiddleware);

// Serve views
function serveView(name: string) {
  return (_req: express.Request, res: express.Response) => {
    const filePath = path.join(__dirname, 'views', name);
    res.type('html').send(fs.readFileSync(filePath, 'utf-8'));
  };
}

// Page routes
app.get('/login', serveView('login.html'));
app.get('/authorize', serveView('authorize.html'));
app.get('/dashboard', serveView('dashboard.html'));
app.get('/chat', serveView('chat.html'));

// Static assets for chat UI
app.use('/static', express.static(path.join(__dirname, 'views', 'static')));

// Serve chat-ui CSS from node_modules
app.get('/static/chat-styles.css', (_req, res) => {
  const cssPath = path.join(__dirname, '..', 'node_modules', '@nexus-ai', 'agent-chat-ui', 'dist', 'styles.css');
  res.type('css').sendFile(cssPath);
});

const JWT_SECRET = 'dev-jwt-secret-for-testing-only';

// SSE proxy: /process -> http://localhost:8090/process
app.all('/process', (req, res) => {
  const user: SessionUser | undefined = (req as any).sessionUser;
  if (!user) {
    res.status(401).json({ error: '请先登录' });
    return;
  }
  const token = jwt.sign({ userId: user.userId }, JWT_SECRET, { expiresIn: '72h' });
  const proxyReq = http.request(
    {
      hostname: 'localhost',
      port: 8090,
      path: '/process',
      method: req.method,
      headers: {
        'Content-Type': 'application/json',
        Accept: 'text/event-stream',
        Authorization: `Bearer ${token}`,
      },
    },
    (proxyRes) => {
      res.writeHead(proxyRes.statusCode || 502, proxyRes.headers);
      proxyRes.pipe(res, { end: true });
    }
  );
  proxyReq.on('error', (err) => {
    console.error('[Proxy] Error:', err.message);
    if (!res.headersSent) {
      res.status(502).json({ error: 'Agent service unavailable' });
    }
  });
  if (req.body && Object.keys(req.body).length > 0) {
    proxyReq.write(JSON.stringify(req.body));
  }
  proxyReq.end();
});

// API: Login
app.post('/api/login', (req, res) => {
  const { userId, password } = req.body;
  const user = userStore.findByCredentials(userId, password);
  if (!user) {
    res.json({ success: false, message: '用户名或密码错误' });
    return;
  }
  const sid = generateSessionId();
  sessions.set(sid, { userId: user.userId, userName: user.userName });
  res.cookie('gw_session', sid, {
    maxAge: 86400000,
    httpOnly: true,
    sameSite: 'lax',
  });
  res.json({ success: true });
});

// API: Logout
app.post('/api/logout', (_req, res) => {
  const sid = _req.cookies?.gw_session;
  if (sid) sessions.delete(sid);
  res.clearCookie('gw_session');
  res.redirect('/login');
});

// API: Current user
app.get('/api/me', (req, res) => {
  const user: SessionUser | undefined = (req as any).sessionUser;
  if (!user) {
    res.json({ success: false });
    return;
  }
  res.json({ success: true, user });
});

// API: List users
app.get('/api/users', (_req, res) => {
  res.json(userStore.listAll());
});

// API: Add user
app.post('/api/users', (req, res) => {
  const { userId, userName, password } = req.body;
  if (!userId || !userName || !password) {
    res.status(400).json({ error: 'Missing fields' });
    return;
  }
  userStore.add({ userId, userName, password });
  res.json({ success: true });
});

// API: Delete user
app.delete('/api/users/:userId', (req, res) => {
  userStore.remove(req.params.userId);
  res.json({ success: true });
});

// OAuth2 endpoints
app.get('/oauth2/authorize', authorize.handleAuthorize);
app.post('/oauth2/approve', authorize.handleAuthorizeApprove);
app.post('/oauth2/token', token.handleToken);
app.get('/oauth2/userinfo', userinfo.handleUserInfo);

// OAuth2 debug page
app.get('/oauth2/debug', (_req, res) => {
  res.type('html').send(`<!DOCTYPE html><html><head><meta charset="UTF-8"><title>OAuth2 Debug</title>
<style>
body { font-family: monospace; max-width: 800px; margin: 40px auto; padding: 0 20px; background: #1a1a2e; color: #eee; }
h1 { color: #667eea; }
.form-group { margin-bottom: 16px; }
label { display: block; margin-bottom: 4px; color: #aaa; }
input, select { width: 100%; padding: 8px; background: #16213e; border: 1px solid #333; color: #eee; border-radius: 4px; }
button { padding: 10px 24px; background: #667eea; color: #fff; border: none; border-radius: 4px; cursor: pointer; }
button:hover { opacity: 0.9; }
pre { background: #16213e; padding: 16px; border-radius: 4px; overflow-x: auto; margin-top: 16px; }
</style></head><body>
<h1>OAuth2 Debugger</h1>
<p>模拟发起 OAuth2 授权请求</p>
<div class="form-group"><label>Client ID</label><input id="clientId" value="agentdisk"></div>
<div class="form-group"><label>Redirect URI</label><input id="redirectUri" value="http://localhost:9101/auth/callback"></div>
<div class="form-group"><label>Scope</label><input id="scope" value="openid profile"></div>
<div class="form-group"><label>State</label><input id="state" value="test-state"></div>
<div class="form-group"><label>Code Challenge (optional)</label><input id="codeChallenge" value=""></div>
<div class="form-group"><label>Prompt</label><select id="prompt"><option value="">默认</option><option value="none">none (无感)</option></select></div>
<button onclick="startAuth()">发起授权</button>
<pre id="result"></pre>
<script>
function startAuth() {
  const params = new URLSearchParams({
    response_type: 'code',
    client_id: document.getElementById('clientId').value,
    redirect_uri: document.getElementById('redirectUri').value,
    scope: document.getElementById('scope').value,
    state: document.getElementById('state').value,
    prompt: document.getElementById('prompt').value,
  });
  const cc = document.getElementById('codeChallenge').value;
  if (cc) {
    params.set('code_challenge', cc);
    params.set('code_challenge_method', 'S256');
  }
  const url = '/oauth2/authorize?' + params.toString();
  document.getElementById('result').textContent = 'Redirecting to: ' + url;
  setTimeout(() => { window.location.href = url; }, 1000);
}
</script></body></html>`);
});

// Redirect root to dashboard or login
app.get('/', (req, res) => {
  const user: SessionUser | undefined = (req as any).sessionUser;
  if (user) {
    res.redirect('/dashboard');
  } else {
    res.redirect('/login');
  }
});

app.listen(PORT, () => {
  console.log(`Agent 网关已启动: http://localhost:${PORT}`);
  console.log(`  登录页:     http://localhost:${PORT}/login`);
  console.log(`  仪表盘:     http://localhost:${PORT}/dashboard`);
  console.log(`  OAuth2 调试: http://localhost:${PORT}/oauth2/debug`);
});
