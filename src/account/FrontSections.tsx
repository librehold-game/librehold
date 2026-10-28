import villageUrl from '../assets/pixel/account/feature-village.webp';
import armyUrl from '../assets/pixel/account/feature-army.webp';
import reportsUrl from '../assets/pixel/account/feature-reports.webp';
import incomingUrl from '../assets/pixel/account/feature-incoming.webp';
import facts from './content/facts.json';
import { Link } from './router';
import { pathFor } from './routes';
import { useLocale, useT } from '../i18n/i18n';

const FEATURES = [
  { id: 'village', src: villageUrl, width: 1200, height: 675 },
  { id: 'army', src: armyUrl, width: 390, height: 690 },
  { id: 'reports', src: reportsUrl, width: 1200, height: 675 },
  { id: 'incoming', src: incomingUrl, width: 1200, height: 675 },
];

/** What the game has, one real screenshot each. */
export function Features() {
  const t = useT();
  return (
    <section className="card features">
      <h2>{t('site.home.features.title')}</h2>
      <ul className="feature-list">
        {FEATURES.map((f) => (
          <li key={f.id} className="feature">
            <img src={f.src} width={f.width} height={f.height} alt={t(`site.home.features.${f.id}.alt`)} loading="lazy" fetchPriority="low" />
            <div>
              <h3>{t(`site.home.features.${f.id}.title`)}</h3>
              <p>{t(`site.home.features.${f.id}.text`)}</p>
            </div>
          </li>
        ))}
      </ul>
    </section>
  );
}

export const FRONT_FAQ = ['free', 'payToWin', 'bots', 'time'];

export function FrontFaq() {
  const t = useT();
  const locale = useLocale();
  return (
    <section className="card front-faq">
      <h2>{t('site.home.faq.title')}</h2>
      <dl>
        {FRONT_FAQ.map((id) => (
          <div key={id}>
            <dt>{t(`site.faq.items.${id}.q`)}</dt>
            <dd>{t(`site.faq.items.${id}.a`, facts)}</dd>
          </div>
        ))}
      </dl>
      <Link to={pathFor('faq', locale)}>{t('site.home.faq.all')}</Link>
    </section>
  );
}
