import { and, eq } from 'drizzle-orm';
import { db as defaultDb, type Database } from '../../db/index.js';
import { userCredentials, users, workspaceMemberships, workspaces } from '../../db/schema.js';
import type { MembershipRole } from './roles.js';

export type WorkspaceMode = 'founder' | 'investment_firm' | 'sales' | 'mixed';

export type AuthUser = {
  id: string;
  email: string;
  name: string;
  avatarUrl?: string | null;
};

export type AuthWorkspace = {
  id: string;
  name: string;
  slug: string;
  mode: WorkspaceMode;
};

export type WorkspaceMembership = {
  workspaceId: string;
  role: MembershipRole;
};

export type WorkspaceMembershipWithWorkspace = {
  workspace: AuthWorkspace;
  role: MembershipRole;
};

export type CreateUserWorkspaceInput = {
  email: string;
  name: string;
  passwordHash: string;
  workspaceName: string;
  workspaceSlug: string;
  workspaceMode: WorkspaceMode;
};

export type CreateWorkspaceForUserInput = {
  userId: string;
  name: string;
  slug: string;
  mode: WorkspaceMode;
};

export type AddWorkspaceMemberInput = {
  workspaceId: string;
  email: string;
  role: MembershipRole;
};

export type AddWorkspaceMemberResult = {
  user: AuthUser;
  workspace: AuthWorkspace;
  role: MembershipRole;
};

export interface AuthStore {
  createUserWorkspace(input: CreateUserWorkspaceInput): Promise<AddWorkspaceMemberResult>;
  findUserByEmail(email: string): Promise<AuthUser | null>;
  getUserById(userId: string): Promise<AuthUser | null>;
  getPasswordHash(userId: string): Promise<string | null>;
  getWorkspaceById(workspaceId: string): Promise<AuthWorkspace | null>;
  getMembership(userId: string, workspaceId: string): Promise<WorkspaceMembership | null>;
  listMembershipsForUser(userId: string): Promise<WorkspaceMembershipWithWorkspace[]>;
  createWorkspaceForUser(
    input: CreateWorkspaceForUserInput
  ): Promise<WorkspaceMembershipWithWorkspace>;
  addWorkspaceMember(input: AddWorkspaceMemberInput): Promise<AddWorkspaceMemberResult | null>;
}

const userSelect = {
  id: users.id,
  email: users.email,
  name: users.name,
  avatarUrl: users.avatarUrl,
};

const workspaceSelect = {
  id: workspaces.id,
  name: workspaces.name,
  slug: workspaces.slug,
  mode: workspaces.mode,
};

export function createDrizzleAuthStore(database: Database = defaultDb): AuthStore {
  return {
    async createUserWorkspace(input) {
      return await database.transaction(async (tx) => {
        const [user] = await tx
          .insert(users)
          .values({ email: input.email, name: input.name })
          .returning(userSelect);

        await tx.insert(userCredentials).values({
          userId: user.id,
          passwordHash: input.passwordHash,
        });

        const [workspace] = await tx
          .insert(workspaces)
          .values({
            name: input.workspaceName,
            slug: input.workspaceSlug,
            mode: input.workspaceMode,
          })
          .returning(workspaceSelect);

        const [membership] = await tx
          .insert(workspaceMemberships)
          .values({ workspaceId: workspace.id, userId: user.id, role: 'owner' })
          .returning({ role: workspaceMemberships.role });

        return { user, workspace, role: membership.role };
      });
    },

    async findUserByEmail(email) {
      const [user] = await database.select(userSelect).from(users).where(eq(users.email, email));
      return user ?? null;
    },

    async getUserById(userId) {
      const [user] = await database.select(userSelect).from(users).where(eq(users.id, userId));
      return user ?? null;
    },

    async getPasswordHash(userId) {
      const [credential] = await database
        .select({ passwordHash: userCredentials.passwordHash })
        .from(userCredentials)
        .where(eq(userCredentials.userId, userId));

      return credential?.passwordHash ?? null;
    },

    async getWorkspaceById(workspaceId) {
      const [workspace] = await database
        .select(workspaceSelect)
        .from(workspaces)
        .where(eq(workspaces.id, workspaceId));

      return workspace ?? null;
    },

    async getMembership(userId, workspaceId) {
      const [membership] = await database
        .select({ workspaceId: workspaceMemberships.workspaceId, role: workspaceMemberships.role })
        .from(workspaceMemberships)
        .where(
          and(
            eq(workspaceMemberships.userId, userId),
            eq(workspaceMemberships.workspaceId, workspaceId)
          )
        );

      return membership ?? null;
    },

    async listMembershipsForUser(userId) {
      const rows = await database
        .select({ workspace: workspaceSelect, role: workspaceMemberships.role })
        .from(workspaceMemberships)
        .innerJoin(workspaces, eq(workspaceMemberships.workspaceId, workspaces.id))
        .where(eq(workspaceMemberships.userId, userId));

      return rows.map((row) => ({ workspace: row.workspace, role: row.role }));
    },

    async createWorkspaceForUser(input) {
      return await database.transaction(async (tx) => {
        const [workspace] = await tx
          .insert(workspaces)
          .values({ name: input.name, slug: input.slug, mode: input.mode })
          .returning(workspaceSelect);

        const [membership] = await tx
          .insert(workspaceMemberships)
          .values({ workspaceId: workspace.id, userId: input.userId, role: 'owner' })
          .returning({ role: workspaceMemberships.role });

        return { workspace, role: membership.role };
      });
    },

    async addWorkspaceMember(input) {
      const user = await this.findUserByEmail(input.email);
      const workspace = await this.getWorkspaceById(input.workspaceId);
      if (!user || !workspace) return null;

      const [membership] = await database
        .insert(workspaceMemberships)
        .values({ workspaceId: input.workspaceId, userId: user.id, role: input.role })
        .returning({ role: workspaceMemberships.role });

      return { user, workspace, role: membership.role };
    },
  };
}
