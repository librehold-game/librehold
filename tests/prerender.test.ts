import { describe, expect, it } from 'vitest';
import { headText, prerenderUrls, render } from '../src/account/entry-server';

describe('prerender', () => {
  it('front page renders in both languages', () => {
    const en = render('/');
    expect(en.html).toContain('Village wars in your browser');
    expect(en.head.jsonLd).toHaveLength(2);
    expect(en.head.notFound).toBeFalsy();
    expect(render('/pl/').html).toContain('Wojna o wioski w przeglądarce');
  });

  it('pages not built yet and unknown addresses are not found', () => {
    for (const url of ['/how-to-play/nope', '/units/dragon', '/nope']) expect(render(url).head.notFound, url).toBe(true);
  });

  it('image preloads are hoisted out of the body', () => {
    const { html, hoisted } = render('/');
    expect(html.startsWith('<link')).toBe(false);
    expect(hoisted).toContain('rel="preload"');
  });

  it('prerenderUrls lists every prerendered address', () => {
    const urls = prerenderUrls();
    for (const u of ['/', '/pl/', '/privacy', '/pl/prywatnosc', '/units/spearman', '/pl/jednostki/pikinier', '/how-to-play/first-hour']) {
      expect(urls).toContain(u);
    }
    expect(urls).not.toContain('/lobby');
    expect(urls.some((u) => u.includes(':'))).toBe(false);
  });

  it('headText has the site name and the head copy in both languages', () => {
    const text = headText();
    expect(text.en['app.name']).toBe('LibreHold');
    expect(text.pl['head.count.players']).toEqual({ one: '# gracz', few: '# gracze', many: '# graczy', other: '# gracza' });
    expect(text.en['head.game.ended.title']).toBe('{name} — won by {winner} · {players}, {conquests}');
  });
});
