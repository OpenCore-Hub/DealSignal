export const membershipRoles = ['owner', 'admin', 'member', 'viewer'] as const;

export type MembershipRole = (typeof membershipRoles)[number];

const roleRank: Record<MembershipRole, number> = {
  viewer: 0,
  member: 1,
  admin: 2,
  owner: 3,
};

export function isMembershipRole(value: unknown): value is MembershipRole {
  return typeof value === 'string' && membershipRoles.includes(value as MembershipRole);
}

export function parseMembershipRole(value: unknown): MembershipRole | null {
  return isMembershipRole(value) ? value : null;
}

export function hasMinimumWorkspaceRole(role: MembershipRole, minimumRole: MembershipRole): boolean {
  return roleRank[role] >= roleRank[minimumRole];
}
