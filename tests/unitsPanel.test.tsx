// @vitest-environment happy-dom
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { cleanup, render, screen } from '@testing-library/react';
import { setLocale } from '../src/i18n/i18n';
import type { GameState, Village } from '../src/engine/types';
import { useGameStore } from '../src/state/gameStore';
import { villageLabelOf } from '../src/ui/villageName';
import { UnitsPanel } from '../src/ui/views/UnitsPanel';
import { activeVillage, addOwnVillage, makeBotVillage, newTestGame } from './helpers';

const HOUR = 3_600_000;
let g: GameState;
let first: Village;
let second: Village;

beforeEach(() => {
  setLocale('en');
  g = newTestGame();
  first = activeVillage(g);
  second = addOwnVillage(g, first.x + 3, first.y + 2, 'Second');
  g.activeVillageId = second.id;
});
afterEach(() => cleanup());

const show = () => {
  useGameStore.setState({ game: g, phase: 'ready' });
  return render(<UnitsPanel />);
};

describe('units panel', () => {
  it('stationed support is listed per sender village with exact counts', () => {
    second.supportFrom = { [first.id]: { spear: 40 } };
    const { container } = show();
    const group = container.querySelector('[data-support]')!;
    expect(group.textContent).toContain('Support');
    expect(group.textContent).toContain(villageLabelOf(g, first));
    expect(group.textContent).toContain('40');
  });

  it('own support on its way shows as +N arriving', () => {
    g.commands.push({ id: 's1', type: 'support', originId: first.id, targetId: second.id, units: { spear: 25 }, departedAt: g.now, arrivesAt: g.now + HOUR });
    show();
    const row = screen.getAllByTitle('Spearman').map((el) => el.closest('li')).find(Boolean)!;
    expect(row.textContent).toContain('+25 arriving');
  });

  it('foreign support on its way shows only that it is coming', () => {
    const bot = makeBotVillage(g);
    g.commands.push({ id: 's2', type: 'support', originId: bot.id, targetId: second.id, units: { spear: 77 }, departedAt: g.now, arrivesAt: g.now + HOUR });
    const { container } = show();
    expect(container.textContent).toContain('Support is on its way');
    expect(container.textContent).not.toContain('77');
  });
});
