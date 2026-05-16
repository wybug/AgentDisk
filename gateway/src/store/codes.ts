export interface AuthCodeEntry {
  code: string;
  clientId: string;
  redirectUri: string;
  userId: string;
  codeChallenge: string;
  codeChallengeMethod: string;
  scope: string;
  expiresAt: number;
}

const codes = new Map<string, AuthCodeEntry>();

export function store(entry: AuthCodeEntry): void {
  codes.set(entry.code, entry);
}

export function consume(code: string): AuthCodeEntry | undefined {
  const entry = codes.get(code);
  if (!entry) return undefined;
  codes.delete(code);
  if (Date.now() > entry.expiresAt) return undefined;
  return entry;
}
