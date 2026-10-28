// @vitest-environment happy-dom
import { afterEach, beforeAll, describe, expect, it } from 'vitest';
import { cleanup, render } from '@testing-library/react';
import { renderToString } from 'react-dom/server';
import { Head, HeadSink, type HeadData } from '../src/account/Head';
import { headTags } from '../src/account/headTags';
import { homeStructuredData } from '../src/account/structuredData';
import { loadAccountCatalogs } from '../src/i18n/catalogs';
import { setLocale } from '../src/i18n/i18n';

beforeAll(() => loadAccountCatalogs());
afterEach(() => {
  cleanup();
  history.replaceState(null, '', '/');
  setLocale('en');
});

const inOrder = (html: string, parts: string[]) => {
  let at = -1;
  for (const p of parts) {
    const i = html.indexOf(p, at + 1);
    expect(i, p).toBeGreaterThan(at);
    at = i;
  }
};

describe('head tags', () => {
  it('indexed page', () => {
    const html = headTags({ title: 'T & co', description: 'D' }, '/privacy');
    inOrder(html, [
      '<title>T &amp; co</title>',
      '<meta name="description" content="D">',
      '<link rel="canonical" href="__SITE_URL__/privacy">',
      '<link rel="alternate" hreflang="en" href="__SITE_URL__/privacy">',
      '<link rel="alternate" hreflang="pl" href="__SITE_URL__/pl/prywatnosc">',
      '<link rel="alternate" hreflang="x-default" href="__SITE_URL__/privacy">',
      '<meta property="og:image" content="__SITE_URL__/og/home.png">',
      '<meta property="og:image:width" content="1200">',
      '<meta property="og:image:height" content="630">',
      '<meta property="og:locale" content="en_US">',
      '<meta property="og:locale:alternate" content="pl_PL">',
      '<meta name="twitter:card" content="summary_large_image">',
    ]);
    expect(html).not.toContain('name="robots"');
  });

  it('noindex from the table or the page', () => {
    expect(headTags({ title: 'L', description: 'D' }, '/lobby')).toContain('<meta name="robots" content="noindex">');
    expect(headTags({ title: 'P', description: 'D', noIndex: true }, '/privacy')).toContain('<meta name="robots" content="noindex">');
  });

  it('Polish home', () => {
    const html = headTags({ title: 'H', description: 'D' }, '/pl/');
    expect(html).toContain('<link rel="canonical" href="__SITE_URL__/pl/">');
    expect(html).toContain('<meta property="og:locale" content="pl_PL">');
  });

  it('not found has no canonical', () => {
    const html = headTags({ title: 'N', description: 'D', notFound: true }, '/nope');
    expect(html).toContain('<meta name="robots" content="noindex">');
    expect(html).not.toContain('canonical');
    expect(html).not.toContain('hreflang');
  });

  it('JSON-LD is escaped', () => {
    const html = headTags({ title: 'J', description: 'D', jsonLd: [{ name: '</script>' }] }, '/');
    expect(html.match(/application\/ld\+json/g)).toHaveLength(1);
    expect(html).toContain('\\u003c/script>');
    expect(html.match(/<\/script>/g)).toHaveLength(1);
  });

  it('home structured data', () => {
    const [site, game] = homeStructuredData('pl');
    expect(site).toMatchObject({ '@type': 'WebSite', name: 'LibreHold', url: '__SITE_URL__/pl/', inLanguage: 'pl' });
    expect(game).toMatchObject({
      '@type': 'VideoGame', name: 'LibreHold', url: '__SITE_URL__/pl/', image: '__SITE_URL__/og/home.png',
      genre: 'Strategy', gamePlatform: 'Web browser', applicationCategory: 'GameApplication', operatingSystem: 'Any',
      inLanguage: ['en', 'pl'], playMode: 'MultiPlayer',
      offers: { '@type': 'Offer', price: '0', priceCurrency: 'EUR' },
      author: { '@type': 'Person', name: 'Szymon Kocur', url: 'https://szymonkocur.com' },
      publisher: { '@type': 'Organization', name: 'TomcatFlow', url: 'https://tomcatflow.com' },
    });
    expect(site).toMatchObject({ publisher: { '@type': 'Organization', name: 'TomcatFlow', url: 'https://tomcatflow.com' } });
    expect(typeof game!.description).toBe('string');
    expect(JSON.stringify(homeStructuredData('en'))).not.toContain('aggregateRating');
  });

  it('Head fills the sink during server render', () => {
    const sink: HeadData = { title: '', description: '' };
    renderToString(
      <HeadSink.Provider value={sink}>
        <Head title="Privacy" description="What we keep" noIndex />
      </HeadSink.Provider>,
    );
    expect(sink).toMatchObject({ title: 'Privacy', description: 'What we keep', noIndex: true });
  });

  it('Head updates the document in the browser', () => {
    history.replaceState(null, '', '/privacy');
    render(<Head title="Privacy page" description="What we keep" />);
    expect(document.title).toBe('Privacy page');
    expect(document.querySelector('meta[name="description"]')?.getAttribute('content')).toBe('What we keep');
    expect(document.querySelector('link[rel="canonical"]')?.getAttribute('href')).toBe(location.origin + '/privacy');
    expect(document.querySelector('link[hreflang="pl"]')?.getAttribute('href')).toBe(location.origin + '/pl/prywatnosc');
  });
});
