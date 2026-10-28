// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen, waitFor, within } from '@testing-library/react';
import type { LiveStrip as Live, PublicEvent } from '../src/account/api';
import { EventList } from '../src/account/EventList';
import { LiveStrip } from '../src/account/LiveStrip';
import { agoText } from '../src/account/live';
import { loadAccountCatalogs } from '../src/i18n/catalogs';
import { setLocale } from '../src/i18n/i18n';

const now = Date.now();
const event = (over: Partial<PublicEvent> = {}): PublicEvent => ({
  at: now - 3 * 60_000, kind: 'battle', worldId: 3, worldName: 'Kraina', instant: false, attacker: 'Ala', defender: 'Ola',
  attackerBot: false, defenderBot: false, troops: 1200, x: 5, y: 7, ...over,
});
const live = (over: Partial<Live> = {}): Live => ({
  games: 2, humans: 3, bots: 40, openSeats: 5, lastBattleAt: now - 2 * 60_000,
  events: [event({ attacker: 'Wolf', attackerBot: true }), event(), event(), event(), event()], ...over,
});

let reply: () => Promise<Response>;

beforeEach(() => {
  loadAccountCatalogs();
  setLocale('en');
  reply = async () => new Response(JSON.stringify(live()), { status: 200 });
  vi.stubGlobal('fetch', vi.fn((url: string) => (url === '/api/v1/public/live' ? reply() : Promise.reject(new Error(url)))));
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  history.replaceState(null, '', '/');
  setLocale('pl');
});

const strip = () => screen.getByRole('region', { name: 'Live now' });

describe('live strip', () => {
  it('numbers, bot label and five events', async () => {
    render(<LiveStrip />);
    await waitFor(() => expect(strip().textContent).toContain('2 games running'));
    const text = strip().textContent!;
    for (const part of ['3 players', '40 bots', '5 free seats', 'Last battle 2 min ago']) expect(text).toContain(part);
    const rows = within(strip()).getAllByRole('listitem');
    expect(rows).toHaveLength(5);
    expect(rows[0]!.textContent).toContain('Wolf (bot) attacked Ola in Kraina · 1,200 troops');
  });

  it('nothing running says so and points at Play', async () => {
    reply = async () => new Response(JSON.stringify(live({ games: 0, humans: 0, bots: 0, openSeats: 0 })), { status: 200 });
    render(<LiveStrip />);
    await waitFor(() => expect(strip().textContent).toContain('Nobody is playing right now'));
    expect(strip().textContent).toContain('Play now');
    expect(strip().textContent).not.toContain('0 games');
  });

  it('with no battle yet it says so once', async () => {
    reply = async () => new Response(JSON.stringify(live({ lastBattleAt: null, events: [] })), { status: 200 });
    render(<LiveStrip />);
    await waitFor(() => expect(strip().textContent).toContain('No battle yet'));
    expect(strip().textContent).not.toContain('No large battle yet');
    expect(within(strip()).getAllByRole('listitem')).toHaveLength(5);
  });

  it('before data it reserves five rows', async () => {
    let resolve!: (r: Response) => void;
    reply = () => new Promise((r) => { resolve = r; });
    render(<LiveStrip />);
    expect(within(strip()).getAllByRole('listitem')).toHaveLength(5);
    expect(strip().textContent).toContain('—');
    resolve(new Response(JSON.stringify(live()), { status: 200 }));
    await waitFor(() => expect(strip().textContent).toContain('2 games running'));
    expect(within(strip()).getAllByRole('listitem')).toHaveLength(5);
  });

  it('a failed fetch keeps the reserved strip', async () => {
    reply = async () => new Response(JSON.stringify({ error: { code: 'internal' } }), { status: 500 });
    render(<LiveStrip />);
    await waitFor(() => expect(fetch).toHaveBeenCalled());
    await new Promise((r) => setTimeout(r, 20));
    expect(within(strip()).getAllByRole('listitem')).toHaveLength(5);
    expect(strip().textContent).toContain('—');
    expect(screen.queryByRole('alert')).toBeNull();
  });
});

describe('event list', () => {
  it('an instant world links to its game page and a long world to its world page', () => {
    const { rerender } = render(<EventList events={[event({ worldId: 7, instant: true, gameNumber: 7 }), event()]} now={now} />);
    const hrefs = screen.getAllByRole('link').map((a) => a.getAttribute('href'));
    expect(hrefs).toEqual(['/games/7', '/worlds/3']);
    history.replaceState(null, '', '/pl/');
    setLocale('pl');
    rerender(<EventList events={[event({ worldId: 7, instant: true, gameNumber: 7 })]} now={now} />);
    expect(screen.getByRole('link').getAttribute('href')).toBe('/pl/gry/7');
  });

  it('a barbarian defender and a bot attacker read as such', () => {
    render(<EventList events={[event({ attacker: 'Wolf', attackerBot: true, defender: '' }),
      event({ kind: 'conquest', defender: 'Bear', defenderBot: true })]} now={now} />);
    const rows = screen.getAllByRole('listitem');
    expect(rows[0]!.textContent).toContain('Wolf (bot) attacked barbarians in Kraina');
    expect(rows[1]!.textContent).toContain('Ala took a village from Bear (bot) in Kraina');
  });

  it('an event a little in the future reads just now', () => {
    expect(agoText(now + 5000, now)).toBe('just now');
    expect(agoText(now - 30_000, now)).toBe('just now');
    expect(agoText(now - 5 * 60_000, now)).toBe('5 min ago');
    render(<EventList events={[event({ at: now + 5000 })]} now={now} />);
    expect(screen.getByRole('listitem').textContent).toContain('just now');
  });

  it('an instant game reads Game #41, Gra #41 in Polish', () => {
    const e = event({ instant: true, gameNumber: 41, worldId: 9, worldName: 'Game #41' });
    const { rerender } = render(<EventList events={[e]} now={now} />);
    expect(screen.getByRole('link').textContent).toBe('Game #41');
    setLocale('pl');
    rerender(<EventList events={[{ ...e }]} now={now} />);
    expect(screen.getByRole('link').textContent).toBe('Gra #41');
  });
});
