import { call, type HostAction, type HostedCard, type HostForm, type HostSettings } from '../api';

/** What an invite link shows anyone who has it. */
export interface InviteInfo {
  name: string; host: string; status: 'waiting' | 'running' | 'ended'; humans: number; seats: number; open: boolean;
  reason?: 'worldFull' | 'joinClosed' | 'worldEnded'; registeredOnly: boolean;
}

/** Hosting's calls, kept out of the front page's bundle. */
export const hostApi = {
  createHosted: (form: HostForm) => call<HostedCard>('POST', '/api/v1/hosted', form),
  /** The world as the host now sees it; null once a world nobody played was closed. */
  hostAction: async (worldId: number, action: HostAction, body?: unknown) =>
    (await call<HostedCard | undefined>('POST', `/api/v1/worlds/${worldId}/host/${action}`, body)) ?? null,
  replaceInvite: (worldId: number) => call<{ inviteCode: string }>('POST', `/api/v1/worlds/${worldId}/host/invite`),
  setRegisteredOnly: (worldId: number, registeredOnly: boolean) =>
    call<HostedCard>('PATCH', `/api/v1/worlds/${worldId}/host`, { registeredOnly }),
  hostSettings: () => call<HostSettings>('GET', '/api/v1/admin/host-settings'),
  saveHostSettings: (s: HostSettings) => call<HostSettings>('PUT', '/api/v1/admin/host-settings', s),
  inviteInfo: (code: string) => call<InviteInfo>('GET', `/api/v1/public/invites/${encodeURIComponent(code)}`),
  joinInvite: (code: string) => call<{ worldId: number; watch?: boolean }>('POST', `/api/v1/invites/${encodeURIComponent(code)}/join`),
};
