// @vitest-environment happy-dom
import { describe, it, expect, afterEach } from 'vitest';
import { cleanup, render } from '@testing-library/react';
import { renderToStaticMarkup } from 'react-dom/server';
import { BUILDING_IDS } from '../src/engine/types';
import { useGameStore } from '../src/state/gameStore';
import { PixelVillageScene } from '../src/ui/scene/pixel/PixelVillageScene';
import { Strip } from '../src/ui/scene/pixel/Strip';
import { newDefaultGame, activeVillage } from './helpers';

afterEach(() => cleanup());

const WOOD = { name: 'wood', file: 'anim/wood.png', w: 26, h: 36, frames: 8, x: 10, y: 20 };

function village(mode: 'full' | 'limited' | 'off'): string {
  const g = newDefaultGame();
  const v = activeVillage(g);
  for (const b of BUILDING_IDS) v.buildings[b] = 0;
  Object.assign(v.buildings, { main: 5, iron: 15, farm: 15, wood: 15 });
  v.buildQueue = [];
  useGameStore.setState({ game: g, phase: 'ready' });
  return render(<PixelVillageScene villageId={g.activeVillageId} mode={mode} />).container.innerHTML;
}

describe('frame strips', () => {
  it('a fixed frame shifts the strip by whole frames and does not animate', () => {
    const html = renderToStaticMarkup(<svg><Strip {...WOOD} frame={3} /></svg>);
    expect(html).toContain('translateX(-78px)');
    expect(html).not.toContain('animation');
  });

  it('without a frame the strip loops with steps() and its own keyframes', () => {
    const html = renderToStaticMarkup(<svg><Strip {...WOOD} ms={720} /></svg>);
    expect(html).toContain('steps(8, end)');
    expect(html).toContain('@keyframes px-wood');
    expect(html).toContain('translateX(-208px)');
    expect(html).toContain('data-anim="wood"');
  });

  it('full mode animates stage-2 mine, fields and mill wheel; limited mode does not', () => {
    const full = village('full');
    for (const name of ['iron', 'farm', 'wood']) expect(full).toContain(`data-anim="${name}"`);
    expect(full).not.toContain('buildings/iron-2.png');
    cleanup();
    const limited = village('limited');
    expect(limited).not.toContain('data-anim="iron"');
    expect(limited).toContain('buildings/iron-2.png');
  });
});
