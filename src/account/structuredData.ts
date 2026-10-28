import { translate, type Locale } from '../i18n/i18n';
import { DEFAULT_IMAGE, SITE_URL } from './headTags';
import { AUTHOR, OPERATOR } from './maker';
import { pathFor } from './routes';

const CONTEXT = 'https://schema.org';
const PUBLISHER = { '@type': 'Organization', ...OPERATOR };

export function homeStructuredData(locale: Locale): Record<string, unknown>[] {
  const name = translate(locale, 'app.name');
  const url = SITE_URL + pathFor('home', locale);
  return [
    { '@context': CONTEXT, '@type': 'WebSite', name, url, inLanguage: locale, publisher: PUBLISHER },
    {
      '@context': CONTEXT,
      '@type': 'VideoGame',
      name,
      url,
      description: translate(locale, 'head.home.description'),
      image: SITE_URL + DEFAULT_IMAGE,
      genre: 'Strategy',
      gamePlatform: 'Web browser',
      applicationCategory: 'GameApplication',
      operatingSystem: 'Any',
      inLanguage: ['en', 'pl'],
      playMode: 'MultiPlayer',
      offers: { '@type': 'Offer', price: '0', priceCurrency: 'EUR' },
      author: { '@type': 'Person', ...AUTHOR },
      publisher: PUBLISHER,
    },
  ];
}

export function breadcrumbList(items: { name: string; path: string }[]): Record<string, unknown> {
  return {
    '@context': CONTEXT,
    '@type': 'BreadcrumbList',
    itemListElement: items.map((it, i) => ({ '@type': 'ListItem', position: i + 1, name: it.name, item: SITE_URL + it.path })),
  };
}
