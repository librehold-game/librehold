import { useEffect, useState, type ReactNode } from 'react';
import { api, type Account } from './api';
import { Head } from './Head';
import { Hero } from './Hero';
import { LanguageNote } from './LanguageNote';
import { RoutePage, type PageProps } from './pages';
import { navigate, usePathname, useRoute } from './router';
import { SiteFooter } from './SiteFooter';
import { SiteHeader } from './SiteHeader';
import { pathFor, twinPath, type RouteId } from './routes';
import { setLocale, useT, type Locale } from '../i18n/i18n';

/** Where an account (or nobody) must go instead of the page the address names; null when the page is right. */
function redirect(id: RouteId, locale: Locale, me: Account | null | undefined): string | null {
  if (me === undefined) return null;
  switch (id) {
    case 'home':
      return me && !me.isGuest ? pathFor('lobby', locale) : null;
    case 'login':
    case 'register':
      return me ? pathFor('lobby', me.locale ?? locale) : null;
    case 'lobby':
    case 'admin':
      if (!me) return pathFor('login', locale);
      if (id === 'admin' && !me.isAdmin) return pathFor('lobby', me.locale ?? locale);
      return me.locale && me.locale !== locale ? pathFor(id, me.locale) : null;
    default:
      return null;
  }
}

function AppHead({ id }: { id: 'login' | 'register' | 'lobby' | 'admin' }) {
  const t = useT();
  return <Head title={t(`head.${id}.title`)} description={t('head.home.description')} />;
}

/** The account site: public pages for everyone, the lobby and the admin panel for accounts. */
export function App() {
  const t = useT();
  const route = useRoute();
  const pathname = usePathname();
  const [me, setMe] = useState<Account | null | undefined>(undefined);
  useEffect(() => {
    api.me().then(setMe, () => setMe(null));
  }, []);
  useEffect(() => {
    setLocale(route.locale);
  }, [route.locale]);

  const target = redirect(route.id, route.locale, me);
  useEffect(() => {
    if (target) navigate(target, true);
  }, [target]);

  const loggedIn = (a: Account) => {
    setMe(a);
    navigate(pathFor('lobby', a.locale ?? route.locale));
  };
  const logout = async () => {
    await api.logout().catch(() => undefined);
    setMe(null);
    navigate(pathFor('home', route.locale));
  };
  const switched = (locale: Locale) => {
    if (me) {
      setMe({ ...me, locale });
      if (!me.isGuest) void api.updateMe({ locale }).catch(() => undefined);
    }
    navigate(twinPath(pathname));
  };

  const loading = <p className="muted">{t('loading')}</p>;
  const props: PageProps = { params: route.params, me, onLogin: loggedIn, onClaimed: setMe };
  const routePage = (id: RouteId) => <RoutePage id={id} {...props} />;
  let page: ReactNode;
  switch (route.id) {
    case 'login':
      page = me !== null ? loading : <><AppHead id="login" /><Hero />{routePage('login')}</>;
      break;
    case 'register':
      page = me !== null ? loading : <><AppHead id="register" />{routePage('register')}</>;
      break;
    case 'lobby':
      page = me ? <><AppHead id="lobby" />{routePage('lobby')}</> : loading;
      break;
    case 'admin':
      page = me?.isAdmin ? <><AppHead id="admin" />{routePage('admin')}</> : loading;
      break;
    default:
      page = routePage(route.id);
  }

  return (
    <div className="page">
      <LanguageNote />
      <SiteHeader me={me} onLogout={logout} onSwitch={switched} />
      <main>{page}</main>
      <SiteFooter />
    </div>
  );
}
