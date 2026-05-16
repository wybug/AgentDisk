export interface TokenEntry {
  accessToken: string;
  tokenType: string;
  expiresIn: number;
  refreshToken: string;
  userId: string;
  userName: string;
  scope: string;
  expiresAt: number;
}

const tokens = new Map<string, TokenEntry>();

export function store(entry: TokenEntry): void {
  tokens.set(entry.accessToken, entry);
  tokens.set(entry.refreshToken, entry);
}

export function findByAccessToken(token: string): TokenEntry | undefined {
  const entry = tokens.get(token);
  if (!entry) return undefined;
  if (Date.now() > entry.expiresAt) {
    tokens.delete(entry.accessToken);
    tokens.delete(entry.refreshToken);
    return undefined;
  }
  return entry;
}
