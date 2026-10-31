import { useEffect, useState } from 'react';
import { useUiStore, type ViewId } from '../../state/uiStore';
import { useGameStore } from '../../state/gameStore';
import { PixelIcon } from '../pixelIcons';
import { useLocale, useT } from '../../i18n/i18n';
import { worldDisplayName } from '../../i18n/names';
import { pathFor } from '../../account/routes';

export const TABS: { id: ViewId; enabled: boolean; key: string; primary: boolean }[] = [
  { id: 'village', enabled: true, key: '1', primary: true },
  { id: 'map', enabled: true, key: '2', primary: true },
  { id: 'rally', enabled: true, key: '3', primary: true },
  { id: 'farm', enabled: true, key: '4', primary: false },
  { id: 'reports', enabled: true, key: '5', primary: true },
  { id: 'overview', enabled: true, key: '6', primary: false },
  { id: 'news', enabled: true, key: '7', primary: false },
  { id: 'quests', enabled: true, key: '8', primary: false },
  { id: 'ranking', enabled: true, key: '0', primary: false },
  { id: 'settings', enabled: true, key: '9', primary: false },
];

export const PRIMARY_TABS = TABS.filter((t) => t.primary);
export const SECONDARY_TABS = TABS.filter((t) => !t.primary);

type Badges = Partial<Record<ViewId, number>>;

function Badge({ n, id }: { n: number; id: ViewId | 'more' }) {
  return <span className={`ml-1 px-1 rounded text-white text-[10px] ${id === 'quests' ? 'bg-ok' : 'bg-bad'}`}>{n}</span>;
}

export function Nav({ view, onChange, badges = {} }: { view: ViewId; onChange: (v: ViewId) => void; badges?: Badges }) {
  const t = useT();
  const locale = useLocale();
  const [more, setMore] = useState(false);
  const hiddenBadges = SECONDARY_TABS.reduce((sum, t) => sum + (badges[t.id] ?? 0), 0);
  const moreActive = SECONDARY_TABS.some((t) => t.id === view);

  useEffect(() => {
    if (!more) return;
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') setMore(false); };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [more]);

  const pick = (id: ViewId) => { onChange(id); setMore(false); };
  const openReport = useUiStore((s) => s.setReportOpen);
  const worldName = useGameStore((s) => worldDisplayName({ name: s.worldName, gameNumber: s.gameNumber }));
  const world = (
    <span className="ml-auto self-center px-2 text-[12px] font-body whitespace-nowrap">
      {worldName && <span className="font-semibold">{worldName}</span>} <a className="underline" href={pathFor('lobby', locale)}>{t('nav.lobby')}</a>
      {' · '}<button type="button" className="underline" onClick={() => openReport(true)}>{t('report.button')}</button>
    </span>
  );

  return (
    <>
      <nav className="hidden md:flex gap-1 mt-3 pl-2 flex-wrap">
        {TABS.map((tab) => (
          <button
            key={tab.id}
            className={`tab ${view === tab.id ? 'tab-active' : ''}`}
            disabled={!tab.enabled}
            onClick={() => tab.enabled && onChange(tab.id)}
            title={t('nav.shortcut', { key: tab.key })}
          >
            {t(`nav.${tab.id}.label`)}
            {(badges[tab.id] ?? 0) > 0 && <Badge n={badges[tab.id]!} id={tab.id} />}
          </button>
        ))}
        {world}
      </nav>

      {more && (
        <div className="md:hidden fixed inset-0 z-40" onClick={() => setMore(false)} aria-hidden />
      )}
      <nav className="md:hidden" aria-label={t('nav.label')}>
        {more && (
          <div className="fixed inset-x-1 bottom-[calc(54px+env(safe-area-inset-bottom,0px))] z-50 panel p-2 grid grid-cols-3 gap-1">
            {SECONDARY_TABS.map((tab) => (
              <button
                key={tab.id}
                className={`navbtn !text-ink ${view === tab.id ? 'bg-white/40 rounded' : ''}`}
                disabled={!tab.enabled}
                onClick={() => tab.enabled && pick(tab.id)}
              >
                <NavIcon id={tab.id} />
                <span>{t(`nav.${tab.id}.short`)}{(badges[tab.id] ?? 0) > 0 && <Badge n={badges[tab.id]!} id={tab.id} />}</span>
              </button>
            ))}
            <button className="navbtn !text-ink" onClick={() => { setMore(false); openReport(true); }}>
              <NavIcon id="menu" />
              <span>{t('report.button')}</span>
            </button>
          </div>
        )}
        <div className="navbar-bottom fixed inset-x-0 bottom-0 z-50 flex">
          {PRIMARY_TABS.map((tab) => (
            <button
              key={tab.id}
              className={`navbtn ${view === tab.id ? 'navbtn-active' : ''}`}
              disabled={!tab.enabled}
              aria-current={view === tab.id ? 'page' : undefined}
              onClick={() => tab.enabled && pick(tab.id)}
            >
              <NavIcon id={tab.id} />
              <span>{t(`nav.${tab.id}.short`)}{(badges[tab.id] ?? 0) > 0 && <Badge n={badges[tab.id]!} id={tab.id} />}</span>
            </button>
          ))}
          <button
            className={`navbtn ${moreActive || more ? 'navbtn-active' : ''}`}
            aria-expanded={more}
            onClick={() => setMore((m) => !m)}
          >
            <NavIcon id="menu" />
            <span>{t('nav.more')}{hiddenBadges > 0 && <Badge n={hiddenBadges} id="more" />}</span>
          </button>
        </div>
      </nav>
    </>
  );
}

export function NavIcon({ id }: { id: ViewId | 'menu' }) {
  return (
    <span className="navbtn-icon" aria-hidden>
      <PixelIcon name={`nav-${id}`} size={22} />
    </span>
  );
}
