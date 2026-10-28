import type { Account } from './api';
import { Link, usePathname } from './router';
import { pathFor, twinPath } from './routes';
import { useLocale, useT, type Locale } from '../i18n/i18n';
import { LanguageSwitch } from '../i18n/LanguageSwitch';

/** The site's top bar: the pages anyone may read, the language, and the account's own links once it is known. */
export function SiteHeader({ me, onLogout, onSwitch }: {
  me: Account | null | undefined; onLogout: () => void; onSwitch?: (l: Locale) => void;
}) {
  const t = useT();
  const locale = useLocale();
  const pathname = usePathname();
  return (
    <header className="top">
      <Link className="brand" to={pathFor('home', locale)}>{t('app.name')}</Link>
      <nav className="site-nav" aria-label={t('site.header.navLabel')}>
        <Link to={pathFor('howToPlay', locale)}>{t('site.header.howToPlay')}</Link>
        <Link to={pathFor('worlds', locale)}>{t('site.header.worlds')}</Link>
        <Link to={pathFor('faq', locale)}>{t('site.header.faq')}</Link>
        {me === null && <Link to={pathFor('login', locale)}>{t('site.header.login')}</Link>}
      </nav>
      <LanguageSwitch hrefFor={(l) => (l === locale ? pathname : twinPath(pathname))} onChange={onSwitch} />
      {me && (
        <nav aria-label={t('site.header.accountLabel')}>
          <Link to={pathFor('lobby', locale)}>{t('nav.lobby')}</Link>
          {me.isAdmin && <Link to={pathFor('admin', locale)}>{t('nav.admin')}</Link>}
          <Link className="muted" to={pathFor('player', locale, { name: me.username })}>{me.username}</Link>
          <button className="link" onClick={onLogout}>{t('nav.logout')}</button>
        </nav>
      )}
    </header>
  );
}
