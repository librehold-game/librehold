// @vitest-environment happy-dom
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import { BUILDINGS } from '../src/data/buildings.data';
import { setLocale } from '../src/i18n/i18n';
import { BUILDING_IDS, type BuildingId, type GameState, type Village } from '../src/engine/types';
import { useGameStore } from '../src/state/gameStore';
import { useUiStore } from '../src/state/uiStore';
import { BuildingsTable } from '../src/ui/views/BuildingsTable';
import { activeVillage, newTestGame } from './helpers';

let g: GameState;
let v: Village;

const setLevels = (levels: Partial<Record<BuildingId, number>>) => {
  for (const id of BUILDING_IDS) v.buildings[id] = levels[id] ?? 0;
};
const show = () => {
  useGameStore.setState({ game: g, phase: 'ready' });
  return render(<BuildingsTable onOpen={() => {}} />);
};
const locked = () => (screen.queryByText(/Not yet available/)?.closest('details') ?? null) as HTMLDetailsElement | null;

beforeEach(() => {
  setLocale('en');
  g = newTestGame();
  v = activeVillage(g);
  v.buildQueue = [];
  useUiStore.setState({ info: [] });
  useUiStore.getState().setPrefs({ lockedOpen: true });
});
afterEach(() => cleanup());

describe('buildings that are not yet available', () => {
  it('lists each with what is missing and what the missing one needs', () => {
    setLevels({ main: 4, barracks: 3 });
    show();
    const row = within(locked()!).getByText('Stable').closest('tr')!;
    expect(within(row).getByText('Barracks 5 (3)').className).toContain('req-missing');
    expect(within(row).getByText('Smithy 5 (0)').className).toContain('req-missing');
    expect(row.textContent).toContain('Smithy needs Town Hall 5 (4)');
    const smithy = within(locked()!).getByText('Smithy').closest('tr')!;
    expect(within(smithy).getByText('Barracks 1').className).toContain('req-met');
  });

  it('a requirement met by a queued build already counts', () => {
    setLevels({ main: 5, barracks: 3, smithy: 5 });
    v.buildQueue = [{ id: 'q', building: 'barracks', targetLevel: 5, kind: 'build', startedAt: g.now, completesAt: g.now + 1000 }];
    show();
    expect(within(locked()!).queryByText('Stable')).toBeNull();
    expect(screen.getByText('Stable')).toBeTruthy();
  });

  it('is absent when every building is available', () => {
    setLevels(Object.fromEntries(BUILDING_IDS.map((id) => [id, BUILDINGS[id].maxLevel])));
    show();
    expect(locked()).toBeNull();
  });

  it('a name opens the details window', () => {
    setLevels({ main: 4, barracks: 3 });
    show();
    fireEvent.click(within(locked()!).getByRole('button', { name: 'Stable' }));
    expect(useUiStore.getState().info).toEqual([{ kind: 'building', id: 'stable' }]);
  });

  it('remembers whether the list is folded', () => {
    setLevels({ main: 4, barracks: 3 });
    show();
    const details = locked()!;
    details.open = false;
    fireEvent(details, new Event('toggle'));
    expect(useUiStore.getState().prefs.lockedOpen).toBe(false);
  });

  it('a building not built yet opens its window from the main table too', () => {
    setLevels({ main: 5, barracks: 1 });
    show();
    fireEvent.click(screen.getAllByRole('button', { name: 'Smithy' })[0]!);
    expect(useUiStore.getState().info).toEqual([{ kind: 'building', id: 'smithy' }]);
  });
});
