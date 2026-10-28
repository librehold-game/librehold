import { useLocale, useT } from '../i18n/i18n';
import type { Account } from './api';
import { Clip } from './Clip';
import facts from './content/facts.json';
import { FrontFaq, Features } from './FrontSections';
import { Head } from './Head';
import { LiveStrip } from './LiveStrip';
import { OpenWorlds } from './OpenWorlds';
import { PlayButton } from './PlayButton';
import { Link } from './router';
import { pathFor } from './routes';
import { StatsBlock } from './StatsBlock';
import { homeStructuredData } from './structuredData';

/** The public front page: the game and its numbers first, then who is playing now, what the game has and a short FAQ. */
export function LandingPage({ me }: { me: Account | null | undefined }) {
  const t = useT();
  const locale = useLocale();
  const play = { signedIn: !!me, continueGame: !!me?.isGuest, waiting: me === undefined };
  return (
    <div className="landing">
      <Head title={t('head.home.title')} description={t('head.home.description')} jsonLd={homeStructuredData(locale)} />
      <section className="front-hero">
        <div className="card pitch">
          <h1>{t('site.home.hero.title')}</h1>
          <p>{t('site.home.hero.subhead', facts)}</p>
          <PlayButton {...play} />
          <p className="muted">{t('site.home.hero.reassure')}</p>
          <Link className="secondary" to={pathFor('register', locale)}>{t('site.home.hero.createAccount')}</Link>
          <p className="muted">{t('site.home.hero.account')}</p>
        </div>
        <Clip />
      </section>
      <LiveStrip />
      <Features />
      <OpenWorlds me={me} />
      <StatsBlock />
      <FrontFaq />
    </div>
  );
}
