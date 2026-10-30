// @vitest-environment happy-dom
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { act, cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import { setLocale } from '../src/i18n/i18n';
import { formatDuration } from '../src/i18n/format';
import { researchTimeMs } from '../src/engine/research';
import type { GameState } from '../src/engine/types';
import { useGameStore } from '../src/state/gameStore';
import { useUiStore } from '../src/state/uiStore';
import { InfoWindow } from '../src/ui/info/InfoWindow';
import { activeVillage, addOwnVillage, newTestGame } from './helpers';

let g: GameState;

beforeEach(() => {
  setLocale('en');
  g = newTestGame();
  useGameStore.setState({ game: g, phase: 'ready' });
  useUiStore.setState({ info: [] });
});
afterEach(() => cleanup());

const open = (ref: Parameters<ReturnType<typeof useUiStore.getState>['openInfo']>[0]) => {
  render(<InfoWindow />);
  act(() => useUiStore.getState().openInfo(ref));
  return screen.getByRole('dialog');
};

describe('info window', () => {
  it('walks from a building to a unit it unlocks and back, and closes on Escape', () => {
    const dialog = open({ kind: 'building', id: 'stable' });
    expect(dialog.textContent).toContain('To unlock in this village');
    expect(dialog.textContent).toContain('Barracks 5: not built');
    fireEvent.click(within(dialog).getByRole('button', { name: 'Light cavalryman' }));
    expect(screen.getByRole('dialog').textContent).toContain('Stats by research level');
    fireEvent.click(screen.getByRole('button', { name: 'Back' }));
    expect(screen.getByRole('dialog').textContent).toContain('Unlocks');
    fireEvent.keyDown(document, { key: 'Escape' });
    expect(screen.queryByRole('dialog')).toBeNull();
  });

  it("shows this world's research times, not the encyclopedia's", () => {
    g.config.worldSpeed = 10;
    const dialog = open({ kind: 'unit', id: 'lightCav' });
    expect(dialog.textContent).toContain(formatDuration(researchTimeMs('lightCav', 2, 10)));
    expect(dialog.textContent).not.toContain(formatDuration(researchTimeMs('lightCav', 2, 1)));
    expect(dialog.textContent).toContain('speed ×10');
  });

  it('a world without coins has no coin row and prices the noble by the world', () => {
    g.config.nobleCost = { wood: 1, clay: 2, iron: 3 };
    let dialog = open({ kind: 'unit', id: 'noble' });
    expect(within(dialog).getByText('Coin')).toBeTruthy();
    cleanup();
    useUiStore.setState({ info: [] });
    g.config.noCoins = true;
    dialog = open({ kind: 'unit', id: 'noble' });
    expect(within(dialog).queryByText('Coin')).toBeNull();
    expect(within(dialog).getByText('1')).toBeTruthy();
    expect(within(dialog).getByText('2')).toBeTruthy();
    expect(within(dialog).getByText('3')).toBeTruthy();
  });

  it("follows the active village while it is open", () => {
    const second = addOwnVillage(g, activeVillage(g).x + 3, activeVillage(g).y + 2, 'Second');
    second.buildings.barracks = 5;
    const dialog = open({ kind: 'building', id: 'stable' });
    expect(dialog.textContent).toContain('Barracks 5: not built');
    act(() => useGameStore.setState({ game: { ...g, activeVillageId: second.id } }));
    expect(screen.getByRole('dialog').textContent).toContain('Barracks 5 ✓');
  });

  it('a unit needs the finished building: a queued one is not green', () => {
    Object.assign(activeVillage(g).buildings, { main: 5, barracks: 1, smithy: 0 });
    activeVillage(g).buildQueue = [{ id: 'q', building: 'smithy', targetLevel: 1, kind: 'build', startedAt: g.now, completesAt: g.now + 1000 }];
    const dialog = open({ kind: 'unit', id: 'axe' });
    expect(dialog.textContent).toContain('Smithy 1: not built');
    expect(dialog.textContent).not.toContain('Smithy 1 ✓');
  });

  it('moves focus into the window, keeps it there through a link and gives it back on close', () => {
    render(<button>opener</button>);
    const opener = screen.getByText('opener');
    opener.focus();
    const dialog = open({ kind: 'building', id: 'stable' });
    expect(dialog.contains(document.activeElement)).toBe(true);
    fireEvent.click(within(dialog).getByRole('button', { name: 'Light cavalryman' }));
    expect(screen.getByRole('dialog').contains(document.activeElement)).toBe(true);
    fireEvent.click(screen.getByRole('button', { name: 'Back' }));
    expect(screen.getByRole('dialog').contains(document.activeElement)).toBe(true);
    fireEvent.keyDown(document, { key: 'Escape' });
    expect(document.activeElement).toBe(opener);
  });

  it('the wall has a picture of its own size, not one that collapses without site styles', () => {
    const dialog = open({ kind: 'building', id: 'wall' });
    const sprite = dialog.querySelector('.wall-sprite') as HTMLElement;
    expect(parseInt(sprite.style.width, 10)).toBeGreaterThan(0);
    expect(parseInt(sprite.style.height, 10)).toBeGreaterThan(0);
  });

  it('says what the research percentages apply to and measures them against level 1', () => {
    const dialog = open({ kind: 'unit', id: 'lightCav' });
    expect(within(dialog).getByText('Bonus to attack and defence')).toBeTruthy();
    expect(within(dialog).getByText('+25%')).toBeTruthy();
    expect(within(dialog).getByText('+40%')).toBeTruthy();
    expect(dialog.textContent).not.toContain('(+25%)');
    expect(dialog.textContent).toContain('compared with level 1');
  });
});
