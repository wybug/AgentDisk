export interface OAuth2Client {
  clientId: string;
  clientSecret: string;
  redirectUris: string[];
}

export interface SessionUser {
  userId: string;
  userName: string;
}
