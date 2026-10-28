// @vitest-environment happy-dom
import { describe, it, expect, beforeEach, afterEach, beforeAll, vi } from 'vitest';
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react';
import type { ReactElement } from 'react';
import { BUILDINGS } from '../src/data/buildings.data';
import { HUMAN_ID, BUILDING_IDS, MS_PER_HOUR, UNIT_IDS, type BuildingId, type GameState, type Village } from '../src/engine/types';
import { storageCapacity } from '../src/engine/buildings';
import { villageProductionPerHour } from '../src/engine/resources';
import { formatNumber } from '../src/ui/format';
import { reportVillageOf } from '../src/engine/reports';
import { useGameStore } from '../src/state/gameStore';
import { useUiStore } from '../src/state/uiStore';
import { activeVillage, addCommand, addOwnVillage, barbarianVillage, newDefaultGame } from './helpers';
import { stubServer } from './serverStub';
import { navigation } from '../src/net/api';
import { App } from '../src/ui/App';
import { setLocale } from '../src/i18n/i18n';

import { AcademyScreen } from '../src/ui/views/AcademyScreen';
import { BuildingScreen } from '../src/ui/views/BuildingScreen';
import { BuildPlanPanel } from '../src/ui/views/BuildPlanPanel';
import { BuildQueuePanel } from '../src/ui/views/BuildQueuePanel';
import { FarmAssistantView } from '../src/ui/views/FarmAssistantView';
import { MapView } from '../src/ui/views/MapView';
import { MarketScreen } from '../src/ui/views/MarketScreen';
import { NewsView } from '../src/ui/views/NewsView';
import { OverviewView } from '../src/ui/views/OverviewView';
import { QuestsView } from '../src/ui/views/QuestsView';
import { RallyPointView } from '../src/ui/views/RallyPointView';
import { RecruitScreen } from '../src/ui/views/RecruitScreen';
import { ReportsView } from '../src/ui/views/ReportsView';
import { ResearchScreen } from '../src/ui/views/ResearchScreen';
import { SettingsView } from '../src/ui/views/SettingsView';
import { UnitsPanel } from '../src/ui/views/UnitsPanel';
import { VillageView } from '../src/ui/views/VillageView';
import { Header } from '../src/ui/layout/Header';
import { Nav } from '../src/ui/layout/Nav';
import { ResourceBar } from '../src/ui/layout/ResourceBar';
import { ErrorToast } from '../src/ui/components/ErrorToast';
import { IncomingList } from '../src/ui/components/IncomingList';

/**
 * UI smoke tests: every view renders from a real game state without throwing and shows its key content.
 * They do not test pixels; they catch runtime errors, empty screens and selectors broken by engine changes.
 */

beforeAll(() => {
  if (!('ResizeObserver' in globalThis)) {
    (globalThis as unknown as { ResizeObserver: unknown }).ResizeObserver = class {
      observe() {} unobserve() {} disconnect() {}
    };
  }
});

/** A grown game: buildings, an army, a battle report, a command on the road, rumours, a second village. */
function richGame(): GameState {
  const g = newDefaultGame();
  const v = activeVillage(g);
  for (const b of BUILDING_IDS) v.buildings[b] = Math.min(BUILDINGS[b].maxLevel, 12);
  v.buildings.academy = 1;
  for (const u of UNIT_IDS) { v.units[u] = 50; v.research[u] = 2; }
  v.resources = { wood: 20_000, clay: 20_000, iron: 20_000 };
  v.points = 2500;
  g.players[HUMAN_ID]!.coins = 3;

  const second = addOwnVillage(g, v.x + 3, v.y + 2, 'Second village');
  for (const b of BUILDING_IDS) second.buildings[b] = 5;
  second.points = 500;

  g.now = MS_PER_HOUR;
  const target = barbarianVillage(g);
  g.reports.push({
    id: 'r1', time: g.now - 10 * 60_000, type: 'attack', originId: v.id, targetId: target.id,
    origin: reportVillageOf(g, v.id), target: reportVillageOf(g, target.id),
    attacker: { units: { axe: 20 }, losses: { axe: 2 } }, defender: { units: { spear: 3 }, losses: { spear: 3 } },
    luck: 0.05, morale: 1, wallBefore: 0, wallAfter: 0, haul: { wood: 120, clay: 90, iron: 60 }, haulCapacity: 360,
    outcome: 'attackerWon', read: false,
  });
  addCommand(g, { type: 'attack', originId: v.id, targetId: target.id, units: { lightCav: 10 } });
  g.news.push({ id: 'n1', time: g.now, kind: 'world', key: 'news.newPlayer', params: { player: 'Torvik', x: 1, y: 2 } });
  g.quests.completed.push('wood1');
  return g;
}

const game = (() => {
  let cached: GameState | null = null;
  return () => (cached ??= richGame());
})();

beforeEach(() => {
  const g = structuredClone(game());
  useGameStore.setState({ game: g, phase: 'ready', devMode: true, notice: null, lastError: null });
  useUiStore.setState({ view: 'village' });
});
afterEach(() => cleanup());

function renders(name: string, el: ReactElement, expected: RegExp): void {
  it(`${name} renders without an error`, () => {
    const { container } = render(el);
    expect(container.textContent ?? '').toMatch(expected);
  });
}

describe('smoke: the main views', () => {
  renders('Village view', <VillageView />, /Budynki|Produkcja/);
  renders('Map', <MapView />, /Mapa|Moja wioska/);
  renders('Rally Point', <RallyPointView />, /Wyślij wojska|Komendy/);
  renders('Farm assistant', <FarmAssistantView />, /Asystent farmienia/);
  renders('Reports', <ReportsView />, /Raport|raport/);
  renders('Overview', <OverviewView />, /Budowa|Jednostki/);
  renders('Rumours', <NewsView />, /Pogłoski/);
  renders('Quests', <QuestsView />, /Zadani/);
  renders('Settings', <SettingsView />, /Ustawienia|Świat/);
});

describe('smoke: the main views in English', () => {
  beforeEach(() => setLocale('en'));
  afterEach(() => setLocale('pl'));
  renders('Village view', <VillageView />, /Buildings|Production/);
  renders('Map', <MapView />, /Map|My village/);
  renders('Rally Point', <RallyPointView />, /Send troops|Commands/);
  renders('Farm assistant', <FarmAssistantView />, /Farm assistant/);
  renders('Reports', <ReportsView />, /Report|report/);
  renders('Overview', <OverviewView />, /Construction|Units/);
  renders('Rumours', <NewsView />, /Rumours/);
  renders('Quests', <QuestsView />, /Quest/);
  renders('Settings', <SettingsView />, /Settings|World/);
  it('no Polish words show on the village and rally screens', () => {
    for (const el of [<VillageView key="v" />, <RallyPointView key="r" />, <ReportsView key="p" />]) {
      const { container, unmount } = render(el);
      expect(container.textContent ?? '').not.toMatch(/[ąćęłńóśźż]|Wioska|Budynki|Wyślij|Raporty/);
      unmount();
    }
  });
});

describe('smoke: building screens', () => {
  renders('Market', <MarketScreen />, /Rynek|kupc/i);
  renders('Smithy (research)', <ResearchScreen />, /Kuźnia|badan/i);
  renders('Academy', <AcademyScreen />, /hetman|monet/i);
  renders('Recruitment (barracks)', <RecruitScreen building="barracks" />, /Pikinier|rekrut/i);
  renders('Building screen', <BuildingScreen building="main" onBack={() => {}} />, /Ratusz/);
});

describe('smoke: panels and the bar', () => {
  renders('Header', <Header />, /Wioska/);
  renders('Resource bar', <ResourceBar />, /\d/);
  renders('Navigation', <Nav view="village" onChange={() => {}} />, /Mapa/);
  renders('Build queue', <BuildQueuePanel />, /Kolejka budowy/);
  renders('Build plan', <BuildPlanPanel />, /[A-Za-zĄ-ż]/);
  renders('Units panel', <UnitsPanel />, /Pikinier|Brak wojska/);
});

describe('smoke: messages', () => {
  it('a toast shows a notification from the world', () => {
    useGameStore.setState({ notice: '🤝 Horda uznała cię za wroga.' });
    render(<ErrorToast />);
    expect(screen.getByText(/Horda uznała cię za wroga/)).toBeTruthy();
  });

});

describe('scheduled sends at the rally point', () => {
  it('the list shows a send with its time to the millisecond and a cancel button', () => {
    const g = useGameStore.getState().game!;
    const v = activeVillage(g);
    g.scheduled.push({ id: 's_test', ownerId: HUMAN_ID, originId: v.id, x: v.x + 3, y: v.y, units: { axe: 5 }, type: 'attack', mode: 'arriveAt', at: g.now + 3_600_250, sendAt: g.now + 1_800_000 });
    useGameStore.setState({ game: { ...g } });
    render(<RallyPointView />);
    expect(screen.getByText('Zaplanowane')).toBeTruthy();
    expect(screen.getByText(/:250$/)).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Odwołaj' })).toBeTruthy();
  });
});

describe('incoming attacks', () => {
  it('the list shows the label, the countdown and dodge / snipe buttons for a train', () => {
    const g = useGameStore.getState().game!;
    const v = activeVillage(g);
    g.players.bot_ui = { id: 'bot_ui', name: 'Foe', isHuman: false, color: '#d23b2f', villageIds: [], coins: 0, noblesTrained: 0, buildQueueSlots: 2, protectedUntil: 0, joinAt: 0, status: 'active', points: 0, history: { hourly: [], daily: [] } };
    const enemy: Village = { ...structuredClone(v), id: `v:${v.x + 4}:${v.y + 3}`, x: v.x + 4, y: v.y + 3, name: 'Hostile', ownerId: 'bot_ui' };
    g.villages[enemy.id] = enemy;
    g.players.bot_ui.villageIds.push(enemy.id);
    const travel = 5 * 35 * 60_000;
    for (const [i, units] of [[0, { axe: 100 }], [1, { noble: 1 }], [2, { noble: 1 }]] as const) {
      g.commands.push({ id: `c_in${i}`, type: 'attack', originId: enemy.id, targetId: v.id, units, departedAt: g.now, arrivesAt: g.now + (i === 0 ? 5 * 18 * 60_000 : travel + i * 100) });
    }
    useGameStore.setState({ game: { ...g } });
    render(<IncomingList villageId={v.id} />);
    expect(screen.getByText('Nadchodzące ataki')).toBeTruthy();
    expect(screen.getAllByText(/hetman\?/).length).toBe(2);
    expect(screen.getAllByRole('button', { name: 'Unik' })).toHaveLength(3);
    expect(screen.getAllByRole('button', { name: 'Snajp' })).toHaveLength(2);
  });
});


describe('menu bar', () => {
  it('clicking Village on the bar goes from a building screen back to the village overview', () => {
    useUiStore.getState().navigate('village');
    render(<VillageView />);
    const opener = screen.getAllByRole('button').find((b) => b.closest('tr[id^="building-row-"]') && b.className.includes('decoration-dotted'));
    fireEvent.click(opener!);
    expect(screen.getByRole('button', { name: '← Wioska' })).toBeTruthy();
    act(() => useUiStore.getState().navigate('village'));
    expect(screen.queryByRole('button', { name: '← Wioska' })).toBeNull();
    expect(screen.getByText('Budynki')).toBeTruthy();
  });
});

describe('village illustration', () => {
  it('clicking a building with its own screen opens that screen', () => {
    const g = useGameStore.getState().game!;
    activeVillage(g).buildings.barracks = 3;
    useGameStore.setState({ game: { ...g } });
    useUiStore.getState().navigate('village');
    const { container } = render(<VillageView />);
    const title = [...container.querySelectorAll('svg title')].find((t) => t.textContent?.startsWith('Koszary'));
    fireEvent.click(title!.parentElement!);
    expect(screen.getByRole('button', { name: '← Wioska' })).toBeTruthy();
    expect(screen.getByRole('heading', { name: /Koszary/ })).toBeTruthy();
  });
});

describe('building pages', () => {
  const PAGES: [BuildingId, RegExp][] = [
    ['main', /Kolejka budowy/], ['wood', /Produkcja/], ['clay', /Produkcja/], ['iron', /Produkcja/],
    ['storage', /Pojemność/], ['farm', /Maksymalna populacja/], ['hide', /Ukryte przed łupieżcami/], ['wall', /Bonus obronny/],
  ];

  it('every building has a page with its current and next level', () => {
    const g = useGameStore.getState().game!;
    const v = activeVillage(g);
    Object.assign(v.buildings, { main: 5, wood: 5, clay: 3, iron: 2, storage: 5, farm: 4, hide: 3, wall: 2 });
    useGameStore.setState({ game: { ...g } });
    for (const [id, text] of PAGES) {
      render(<BuildingScreen building={id} onBack={() => {}} />);
      expect(screen.getAllByText(text).length, id).toBeGreaterThan(0);
      cleanup();
    }
    render(<BuildingScreen building="storage" onBack={() => {}} />);
    expect(screen.getByText(formatNumber(storageCapacity(5)))).toBeTruthy();
    expect(screen.getByText(formatNumber(storageCapacity(6)))).toBeTruthy();
  });

  it('clicking the storehouse in the illustration opens the storehouse page', () => {
    useUiStore.getState().navigate('village');
    const { container } = render(<VillageView />);
    const title = [...container.querySelectorAll('svg title')].find((t) => t.textContent?.startsWith('Spichlerz'));
    fireEvent.click(title!.parentElement!);
    expect(screen.getByRole('heading', { name: /Spichlerz/ })).toBeTruthy();
    expect(screen.getAllByText(/Pojemność/).length).toBeGreaterThan(0);
  });
});

describe('pixel art village illustration', () => {

  it('clicking the barracks opens the barracks screen', () => {
    const g = useGameStore.getState().game!;
    activeVillage(g).buildings.barracks = 3;
    useGameStore.setState({ game: { ...g } });
    useUiStore.getState().navigate('village');
    const { container } = render(<VillageView />);
    expect(container.querySelector('svg.scene.pixel')).toBeTruthy();
    const title = [...container.querySelectorAll('svg title')].find((t) => t.textContent?.startsWith('Koszary'));
    fireEvent.click(title!.parentElement!);
    expect(screen.getByRole('heading', { name: /Koszary/ })).toBeTruthy();
  });

  it('clicking the storage opens the storage screen', () => {
    useUiStore.getState().navigate('village');
    const { container } = render(<VillageView />);
    const title = [...container.querySelectorAll('svg title')].find((t) => t.textContent?.startsWith('Spichlerz'));
    fireEvent.click(title!.parentElement!);
    expect(screen.getByRole('heading', { name: /Spichlerz/ })).toBeTruthy();
  });
});

describe('building table in a faster world', () => {
  it('the lumber mill production in the table includes the world speed (like the Production panel)', () => {
    const g = useGameStore.getState().game!;
    g.config.worldSpeed = 8;
    const v = activeVillage(g);
    v.buildings.wood = 2;
    useGameStore.setState({ game: { ...g } });
    useUiStore.getState().navigate('village');
    const { container } = render(<VillageView />);
    const row = container.querySelector('#building-row-wood')!;
    expect(row.textContent).toContain(`${formatNumber(villageProductionPerHour(v, 'wood', g.config))}/h`);
    expect(villageProductionPerHour(v, 'wood', g.config)).toBeGreaterThan(300);
  });
});

describe('map: the card shown on hovering a village', () => {
  function hoverOwnVillage() {
    const g = useGameStore.getState().game!;
    const v = activeVillage(g);
    const { container } = render(<MapView />);
    const tile = container.querySelector(`[data-tile="${v.x}|${v.y}"]`) as HTMLElement;
    const map = container.querySelector('[data-map-surface]') as HTMLElement;
    const at = { clientX: parseFloat(tile.style.left) + 5, clientY: parseFloat(tile.style.top) + 5, pointerType: 'mouse' };
    return { v, map, at };
  }

  it('shows the name, coordinates with the continent and points, and hides shortly after leaving the map', async () => {
    const { v, map, at } = hoverOwnVillage();
    fireEvent.pointerMove(map, at);
    const card = screen.getByRole('tooltip');
    expect(card.textContent).toContain(`${v.name} (${v.x}|${v.y}) K${Math.floor(v.y / 100)}${Math.floor(v.x / 100)}`);
    expect(card.textContent).toContain('Punkty');
    expect(card.textContent).toContain(formatNumber(v.points));
    fireEvent.pointerLeave(map, { pointerType: 'mouse' });
    expect(screen.queryByRole('tooltip')).not.toBeNull();
    await act(() => new Promise((r) => setTimeout(r, 250)));
    expect(screen.queryByRole('tooltip')).toBeNull();
  });

  it('touch does not open the card (a tap opens the menu there)', () => {
    const { map, at } = hoverOwnVillage();
    fireEvent.pointerMove(map, { ...at, pointerType: 'touch' });
    expect(screen.queryByRole('tooltip')).toBeNull();
  });

  it('dragging the map with a finger does not scroll the page, taps still go through', () => {
    const { map } = hoverOwnVillage();
    const move = new Event('touchmove', { bubbles: true, cancelable: true });
    map.firstElementChild!.dispatchEvent(move);
    expect(move.defaultPrevented).toBe(true);
    const start = new Event('touchstart', { bubbles: true, cancelable: true });
    map.dispatchEvent(start);
    expect(start.defaultPrevented).toBe(false);
  });
});

describe('from the server: views from the player state', () => {
  let calls: string[];
  const boot = async (paused = false) => {
    useGameStore.getState().shutdown();
    calls = stubServer(structuredClone(game()), { me: 'h5', paused });
    await useGameStore.getState().boot();
  };
  afterEach(() => {
    useGameStore.getState().shutdown();
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it('every view renders from the state the server sends', async () => {
    await boot();
    expect(useGameStore.getState().phase).toBe('ready');
    const views: [string, ReactElement, RegExp][] = [
      ['village', <VillageView />, /Budynki|Produkcja/], ['map', <MapView />, /Mapa|Moja wioska/],
      ['rally point', <RallyPointView />, /Wyślij wojska|Komendy/], ['reports', <ReportsView />, /Raport|raport/],
      ['farm assistant', <FarmAssistantView />, /Asystent|farm/i], ['overview', <OverviewView />, /Przegląd|wiosk/i],
      ['rumours', <NewsView />, /Pogłoski|wieści/i], ['quests', <QuestsView />, /Zadania|zadani/i],
      ['settings', <SettingsView />, /Ustawienia/], ['header', <Header />, /[A-Za-zĄ-ż]/],
    ];
    for (const [name, el, expected] of views) {
      const { container, unmount } = render(el);
      expect(container.textContent ?? '', name).toMatch(expected);
      unmount();
    }
  });

  it('settings: no world settings, with logout and the lobby', async () => {
    await boot();
    render(<SettingsView />);
    expect(screen.queryByText('Prędkość świata')).toBeNull();
    expect(screen.queryByText(/Import|Eksport|Nowy świat/)).toBeNull();
    expect(screen.getByRole('button', { name: 'Wyloguj' })).toBeTruthy();
    expect(screen.getByRole('link', { name: 'Lobby' }).getAttribute('href')).toBe('/pl/lobby');
  });

  it('settings show the legal notices: author, no warranty and the source code', async () => {
    await boot();
    render(<SettingsView />);
    expect(screen.getByText(/© 2026 Szymon Kocur i współautorzy/)).toBeTruthy();
    expect(screen.getByText(/bez żadnej gwarancji/)).toBeTruthy();
    expect(screen.getByRole('link', { name: /Kod źródłowy/ }).getAttribute('href')).toBe('https://github.com/librehold-game/librehold');
  });

  it('logging out calls logout and goes back to the front page', async () => {
    await boot();
    const go = vi.spyOn(navigation, 'assign').mockImplementation(() => {});
    render(<SettingsView />);
    fireEvent.click(screen.getByRole('button', { name: 'Wyloguj' }));
    await vi.waitFor(() => expect(go).toHaveBeenCalledWith('/pl/'));
    expect(calls).toContain('POST /api/v1/auth/logout');
  });

  it('a paused world: a banner, and the server refuses orders', async () => {
    await boot(true);
    render(<App />);
    expect(await screen.findAllByText('Świat wstrzymany przez administratora')).not.toHaveLength(0);
    const g = useGameStore.getState().game!;
    expect(await useGameStore.getState().build(g.activeVillageId, 'wood')).toBe('Świat wstrzymany przez administratora');
    expect(calls.at(-1)).toBe('POST /api/v1/worlds/1/orders/build');
  });

  it('resuming the world reaches an open game', async () => {
    await boot(true);
    expect(useGameStore.getState().game!.config.paused).toBe(true);
    const sockets = (globalThis as unknown as { __sockets?: { onmessage: ((e: { data: string }) => void) | null }[] }).__sockets!;
    stubServer(structuredClone(game()), { me: 'h5', paused: false });
    sockets[0]!.onmessage?.({ data: JSON.stringify({ type: 'hello', serverTime: 0, worldNow: 0 }) });
    sockets[0]!.onmessage?.({ data: JSON.stringify({ type: 'update', topics: ['state'], serverTime: 0, worldNow: 0 }) });
    await vi.waitFor(() => expect(useGameStore.getState().game!.config.paused).toBe(false));
  });

  it('the navigation shows the world name and a link to the lobby', async () => {
    await boot();
    render(<Nav view="village" onChange={() => {}} />);
    expect(screen.getByText('Café Test World')).toBeTruthy();
    expect(screen.getByRole('link', { name: /Lobby/ }).getAttribute('href')).toBe('/pl/lobby');
  });

  it('the navigation names an instant game by its number', async () => {
    useGameStore.getState().shutdown();
    stubServer(structuredClone(game()), { me: 'h5', gameNumber: 3 });
    await useGameStore.getState().boot();
    render(<Nav view="village" onChange={() => {}} />);
    expect(screen.getByText('Gra #3')).toBeTruthy();
  });

  it('the farm assistant takes its targets from the server', async () => {
    await boot();
    render(<FarmAssistantView />);
    const vid = useGameStore.getState().game!.activeVillageId;
    await vi.waitFor(() => expect(calls).toContain(`GET /api/v1/worlds/1/farm-targets?village=${encodeURIComponent(vid)}`));
  });
});
