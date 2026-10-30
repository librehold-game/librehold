// @vitest-environment happy-dom
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import { setLocale } from '../src/i18n/i18n';
import { unitName } from '../src/i18n/names';
import type { GameState, Village } from '../src/engine/types';
import { useGameStore } from '../src/state/gameStore';
import { useUiStore } from '../src/state/uiStore';
import { BuildingScreen } from '../src/ui/views/BuildingScreen';
import { RecruitScreen } from '../src/ui/views/RecruitScreen';
import { ResearchScreen } from '../src/ui/views/ResearchScreen';
import { UnitsPanel } from '../src/ui/views/UnitsPanel';
import { activeVillage, newTestGame } from './helpers';

let g: GameState;
let v: Village;

beforeEach(() => {
  setLocale('en');
  g = newTestGame();
  v = activeVillage(g);
  Object.assign(v.buildings, { main: 5, barracks: 1, smithy: 0 });
  useGameStore.setState({ game: g, phase: 'ready' });
  useUiStore.setState({ info: [] });
});
afterEach(() => cleanup());

const named = (unit: Parameters<typeof unitName>[0]) => ({ name: new RegExp(unitName(unit)) });

describe('opening the details window', () => {
  it('a locked unit in the recruit list shows what it still needs, and its name opens the window', () => {
    render(<RecruitScreen building="barracks" />);
    const row = screen.getAllByRole('button', named('axe'))[0]!.closest('tr')!;
    expect(within(row).getByText('Smithy 1 (0)').className).toContain('req-missing');
    expect(within(row).getByText('Barracks 1').className).toContain('req-met');
    fireEvent.click(screen.getAllByRole('button', named('axe'))[0]!);
    expect(useUiStore.getState().info).toEqual([{ kind: 'unit', id: 'axe' }]);
  });

  it('a unit in the Smithy list opens the window', () => {
    render(<ResearchScreen />);
    fireEvent.click(screen.getAllByRole('button', named('lightCav'))[0]!);
    expect(useUiStore.getState().info).toEqual([{ kind: 'unit', id: 'lightCav' }]);
  });

  it('a building page has a Details button', () => {
    render(<BuildingScreen building="barracks" onBack={() => {}} />);
    fireEvent.click(screen.getByRole('button', { name: 'Details' }));
    expect(useUiStore.getState().info).toEqual([{ kind: 'building', id: 'barracks' }]);
  });

  it('a unit icon in the units panel opens the window', () => {
    v.units.spear = 5;
    render(<UnitsPanel />);
    fireEvent.click(screen.getAllByRole('button', named('spear'))[0]!);
    expect(useUiStore.getState().info).toEqual([{ kind: 'unit', id: 'spear' }]);
  });

  it('a unit whose building is only queued stays locked with a red chip', () => {
    v.buildQueue = [{ id: 'q', building: 'smithy', targetLevel: 1, kind: 'build', startedAt: g.now, completesAt: g.now + 1000 }];
    render(<RecruitScreen building="barracks" />);
    const row = screen.getAllByRole('button', named('axe'))[0]!.closest('tr')!;
    expect(within(row).getByText('Smithy 1 (0)').className).toContain('req-missing');
  });
});
