// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { loadAccountCatalogs } from '../src/i18n/catalogs';
import { setLocale, t } from '../src/i18n/i18n';
import { ProfileBody, ProfileHeading, profileViewData, type ProfileHost } from '../src/shared/ranking/ProfileBody';
import type { ConquestRow, Profile } from '../src/shared/ranking/types';

const profile = (over: Partial<Profile['player']> = {}, daily = [1, 5, 9]): Profile => ({
  player: { id: 'p1', name: 'Ala', color: '#d23b2f', status: 'active', points: 1234, villageIds: ['v1'], isBot: false, ...over },
  villages: [{ id: 'v1', name: 'Ala 1', x: 12, y: 34, points: 321 }],
  history: { hourly: [], daily },
});
const conquest = (i: number, toId: string, fromId: string | null): ConquestRow => ({
  t: 1000 - i, x: i, y: i, toId, to: toId === 'p1' ? 'Ala' : 'Other', fromId, from: fromId === null ? null : fromId === 'p1' ? 'Ala' : 'Other',
  toIsBot: false, fromIsBot: false,
});
const calls: string[] = [];
const host: ProfileHost = {
  renderCoords: (x, y) => <button onClick={() => calls.push(`${x}|${y}`)}>{`${x}|${y}`}</button>,
  formatTime: (ms) => `at ${ms}`,
};

beforeEach(() => { loadAccountCatalogs(); setLocale('en'); calls.length = 0; });
afterEach(cleanup);

describe('profileViewData', () => {
  it("keeps only this player's conquests, newest first, at most 20", () => {
    const mixed = Array.from({ length: 30 }, (_, i) => (i % 2 === 0 ? conquest(i, 'p1', null) : conquest(i, 'p2', 'p3')));
    const d = profileViewData(profile(), 2, mixed);
    expect(d.conquests).toHaveLength(15);
    expect(d.conquests.map((c) => c.t)).toEqual([...d.conquests.map((c) => c.t)].sort((a, b) => b - a));
    const all = Array.from({ length: 50 }, (_, i) => conquest(i, 'p1', 'p2'));
    expect(profileViewData(profile(), 2, all).conquests).toHaveLength(20);
  });

  it('labels a gain with where it came from and a loss with who took it', () => {
    const d = profileViewData(profile(), 1, [conquest(1, 'p1', null), conquest(2, 'p9', 'p1')]);
    expect(d.conquests[0]).toMatchObject({ gained: true, label: `1|1 — ${t('profile.from', { name: t('profile.barbarians') })}` });
    expect(d.conquests[1]).toMatchObject({ gained: false, label: `2|2 — ${t('profile.took', { name: 'Other' })}` });
  });

  it('knows whether the viewer is the player', () => {
    expect(profileViewData(profile(), 1, [], 'p1').isMe).toBe(true);
    expect(profileViewData(profile(), 1, [], 'p2').isMe).toBe(false);
    expect(profileViewData(profile(), 1, []).isMe).toBe(false);
  });
});

describe('ProfileBody', () => {
  it('lists villages with host-rendered coordinates and the history', () => {
    const data = profileViewData(profile(), 4, [conquest(1, 'p1', null)]);
    const { container } = render(<ProfileBody data={data} host={host} />);
    fireEvent.click(screen.getByText('12|34'));
    expect(calls).toEqual(['12|34']);
    expect(container.querySelector('svg[aria-label]')).toBeTruthy();
    expect(container.textContent).toContain('at 999');
    expect(container.textContent).toContain('4');
    expect(container.textContent).toContain('1,234');
  });

  it('says there is too little data for a short history', () => {
    render(<ProfileBody data={profileViewData(profile({}, [3]), 1, [])} host={host} />);
    expect(screen.getByText(t('profile.tooFewData'))).toBeTruthy();
  });

  it('says so when there are no villages or conquests', () => {
    const p = { ...profile(), villages: [] };
    render(<ProfileBody data={profileViewData(p, 1, [])} host={host} />);
    expect(screen.getAllByText(t('profile.none'))).toHaveLength(2);
  });
});

describe('ProfileHeading', () => {
  it('shows status for quit and eliminated players, the bot tag and the you mark', () => {
    for (const [status, text] of [['quit', t('profile.statusQuit')], ['eliminated', t('profile.statusEliminated')]] as const) {
      const { container } = render(<ProfileHeading data={profileViewData(profile({ status }), 1, [])} />);
      expect(container.textContent).toContain(text);
      cleanup();
    }
    const bot = render(<ProfileHeading data={profileViewData(profile({ isBot: true }), 1, [])} />);
    expect(bot.container.textContent).toContain(t('ranking.bot'));
    cleanup();
    const me = render(<ProfileHeading data={profileViewData(profile(), 1, [], 'p1')} />);
    expect(me.container.textContent).toContain(t('profile.you').trim());
  });
});
