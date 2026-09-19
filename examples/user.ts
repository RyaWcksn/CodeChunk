// example Go-flavored user service in TypeScript for chunking demos

export interface User {
  id: string;
  email: string;
  createdAt: Date;
}

export type Role = "admin" | "member" | "guest";

const MAX_USERS = 1000;

export class UserService {
  private cache = new Map<string, User>();

  async getUser(id: string): Promise<User | null> {
    if (this.cache.has(id)) return this.cache.get(id)!;
    const u = await this.fetch(id);
    if (u) this.cache.set(id, u);
    return u;
  }

  private async fetch(id: string): Promise<User | null> {
    return null;
  }
}

export function normalizeEmail(email: string): string {
  return email.trim().toLowerCase();
}

export const DEFAULT_ROLE: Role = "member";
