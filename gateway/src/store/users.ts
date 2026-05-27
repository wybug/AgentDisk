export interface TestUser {
  userId: string;
  userName: string;
  password: string;
  department: string;
}

const users = new Map<string, TestUser>();

const defaults: TestUser[] = [
  { userId: '5001185', userName: '测试用户', password: '123456', department: '' },
  { userId: 'user001', userName: '张三', password: 'test123', department: 'engineering' },
  { userId: 'user002', userName: '李四', password: 'test123', department: 'marketing' },
  { userId: 'user003', userName: '王五', password: 'test123', department: '' },
];

for (const u of defaults) {
  users.set(u.userId, u);
}

export function findByCredentials(userId: string, password: string): TestUser | undefined {
  const u = users.get(userId);
  if (u && u.password === password) return u;
  return undefined;
}

export function findById(userId: string): TestUser | undefined {
  return users.get(userId);
}

export type SafeUser = Omit<TestUser, 'password'>;

export function listAll(): SafeUser[] {
  return Array.from(users.values()).map(({ password: _, ...rest }) => rest);
}

export function add(user: TestUser): void {
  users.set(user.userId, user);
}

export function remove(userId: string): boolean {
  return users.delete(userId);
}
