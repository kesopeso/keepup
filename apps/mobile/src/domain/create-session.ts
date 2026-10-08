import type { RoutesApi } from '../api/routes.ts';
import type { SessionRepository } from '../storage/session-repository.ts';
import { isTransportMode } from './routes.ts';
import type { CreateRouteRequest, Membership, Profile } from './routes.ts';

export function normalizeCreateRequest(input: CreateRouteRequest): CreateRouteRequest {
  if (!input.name.trim()) throw new Error('Enter a route name.');
  if (!input.displayName.trim()) throw new Error('Enter your display name.');
  if (!input.clientId.trim() || !isTransportMode(input.transportMode) ||
    !['everyone_can_share', 'joiners_can_view_only'].includes(input.sharingPolicy)) {
    throw new Error('Check your details and try again.');
  }
  return { ...input, name: input.name.trim(), displayName: input.displayName.trim(), description: input.description.trim() };
}

// Keep returned credentials until saving succeeds. Retrying storage must never create another route.
export function createRouteSession(api: RoutesApi, repository: SessionRepository) {
  let created: { member: Membership; profile: Profile } | null = null;
  let inFlight: Promise<{ member: Membership; profile: Profile }> | null = null;
  async function submit(input: CreateRouteRequest, signal: AbortSignal) {
    if (!created) {
      const normalized = normalizeCreateRequest(input);
      const { clientId, displayName, transportMode } = normalized;
      const profile = { clientId, displayName, transportMode };
      await repository.saveProfile(profile);
      if (signal.aborted) throw new Error('Route creation was cancelled.');
      const result = await api.create(normalized, signal);
      created = {
        member: { code: result.route.code, memberId: result.owner.id, memberToken: result.memberToken, ownerToken: result.ownerToken },
        profile,
      };
    }
    await repository.saveMembership(created.member);
    return created;
  }
  return {
    getCreatedCode: () => created?.member.code ?? null,
    submit(input: CreateRouteRequest, signal: AbortSignal) {
      if (inFlight) return inFlight;
      inFlight = submit(input, signal).finally(() => { inFlight = null; });
      return inFlight;
    },
  };
}
