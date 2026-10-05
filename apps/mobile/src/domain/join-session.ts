import { invalidMembership } from '../api/routes.ts';
import type { RoutesApi } from '../api/routes.ts';
import type { SessionRepository } from '../storage/session-repository.ts';
import type { JoinRequest, Membership, RouteSnapshot } from './routes.ts';

export async function joinOrResume(api: RoutesApi, repository: SessionRepository, code: string, input: JoinRequest, signal: AbortSignal): Promise<Membership> {
  const saved = await repository.getMembership(code);
  if (saved) {
    await repository.saveMembership(saved);
    return saved;
  }
  const { password, ...profile } = input;
  await repository.saveProfile(profile);
  const joined = await api.join(code, { ...profile, displayName: profile.displayName.trim(), password }, signal);
  const member = { code, memberId: joined.member.id, memberToken: joined.memberToken };
  // Persist before requesting the snapshot. A failed snapshot must never repeat membership creation.
  await repository.saveMembership(member);
  return member;
}

export async function loadMemberSnapshot(api: RoutesApi, repository: SessionRepository, member: Membership, signal: AbortSignal): Promise<RouteSnapshot> {
  try {
    const snapshot = await api.getSnapshot(member.code, member.memberToken, signal);
    if (snapshot.viewer.memberId !== member.memberId) throw new Error('KeepUp returned access for a different member. Please try again.');
    return snapshot;
  } catch (error) {
    if (invalidMembership(error)) await repository.forgetMembership(member.code);
    throw error;
  }
}
