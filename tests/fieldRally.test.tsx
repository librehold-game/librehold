// @vitest-environment happy-dom
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { setLocale } from '../src/i18n/i18n';
import type { GameState, Village } from '../src/engine/types';
import { useGameStore } from '../src/state/gameStore';
import { RallyPointView } from '../src/ui/views/RallyPointView';
import { activeVillage, makeBotVillage, newTestGame } from './helpers';

const HOUR = 3_600_000;
let g: GameState;
let home: Village;

beforeEach(() => {
  setLocale('en');
  g = newTestGame();
  home = activeVillage(g);
  const bot = makeBotVillage(g);
  g.commands.push(
    { id: 'oi', type: 'intercept', originId: home.id, targetId: '', point: { x: 12.4, y: 7.6 }, units: { lightCav: 5 }, departedAt: g.now, arrivesAt: g.now + HOUR,
      intercept: { commandId: 'x', leg: { originId: bot.id, targetId: home.id, departedAt: g.now, arrivesAt: g.now + 2 * HOUR } } },
    { id: 'rf', type: 'return', originId: '', targetId: home.id, point: { x: 9.5, y: 3 }, units: { axe: 4 }, departedAt: g.now, arrivesAt: g.now + HOUR },
  );
  useGameStore.setState({ game: g, phase: 'ready' });
});
afterEach(() => cleanup());

describe('field armies in the rally point', () => {
  it('own interceptions and returns from the road read properly and cannot be recalled', () => {
    render(<RallyPointView />);
    fireEvent.click(screen.getByRole('button', { name: /^Commands/ }));
    const rows = [...document.querySelectorAll('tbody tr')].map((r) => r.textContent ?? '');
    const intercept = rows.find((r) => r.includes('Interception'))!;
    expect(intercept).toBeTruthy();
    expect(intercept).toContain('12|8');
    expect(rows.join('\n')).not.toContain('rally.cmd');
    const back = rows.find((r) => r.includes('Return'))!;
    expect(back).toContain('10|3');
    expect(screen.queryAllByRole('button', { name: 'Recall' })).toHaveLength(0);
  });
});
