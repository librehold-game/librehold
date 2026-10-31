// @vitest-environment happy-dom
import { afterEach, describe, expect, it } from 'vitest';
import { cleanup, render, screen } from '@testing-library/react';
import { setLocale } from '../src/i18n/i18n';
import { questTitle } from '../src/i18n/names';
import { useGameStore } from '../src/state/gameStore';
import { QuestsView } from '../src/ui/views/QuestsView';
import { newTestGame } from './helpers';

afterEach(() => { cleanup(); setLocale('pl'); });

describe('quest bars show the server\'s progress', () => {
  it('a quest with progress shows its bar; one without shows none', () => {
    setLocale('en');
    const g = newTestGame();
    g.quests = { completed: ['spear10'], active: ['wood1', 'first_attack'], claimed: [], progress: { wood1: 0.5 } };
    useGameStore.setState({ game: g, phase: 'ready' });
    render(<QuestsView />);
    const panel = (id: string) => screen.getByText(questTitle(id), { exact: false }).closest('.panel')!;
    expect(panel('wood1').textContent).toContain('50%');
    expect(panel('first_attack').textContent).not.toMatch(/%/);
    expect(document.body.textContent).not.toContain('NaN');
  });
});
