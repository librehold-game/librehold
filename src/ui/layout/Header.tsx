import { useState } from 'react';
import { PixelIcon } from '../pixelIcons';
import { useGameStore, selectActiveVillage } from '../../state/gameStore';
import { selectActiveFarmCap, selectActivePoints, selectActivePopUsed, selectActiveStorageCap } from '../../state/selectors';
import { formatCoords, formatNumber } from '../format';
import { PopIcon } from '../components/ResourceIcon';
import { ResourceBar } from './ResourceBar';
import { WorldClock } from './WorldClock';
import { humanVillageIds, humanCoins } from '../../engine/players';
import { useT } from '../../i18n/i18n';
import { LanguageSwitch } from '../../i18n/LanguageSwitch';
import { villageLabelOf } from '../villageName';
import { InstantBar } from '../instant/InstantBar';

export function Header() {
  const t = useT();
  const name = useGameStore((s) => villageLabelOf(s.game!, selectActiveVillage(s)));
  const x = useGameStore((s) => selectActiveVillage(s).x);
  const y = useGameStore((s) => selectActiveVillage(s).y);
  const points = useGameStore(selectActivePoints);
  const popUsed = useGameStore(selectActivePopUsed);
  const popCap = useGameStore(selectActiveFarmCap);
  const bonusKind = useGameStore((s) => selectActiveVillage(s).bonus?.kind ?? null);
  const villageId = useGameStore((s) => selectActiveVillage(s).id);
  const villagesJson = useGameStore((s) => JSON.stringify(humanVillageIds(s.game!).map((id) => {
    const v = s.game!.villages[id]!;
    return { id, name: villageLabelOf(s.game!, v), x: v.x, y: v.y };
  })));
  const villages = JSON.parse(villagesJson) as { id: string; name: string; x: number; y: number }[];
  const setActive = useGameStore((s) => s.setActiveVillage);
  const switchVillage = useGameStore((s) => s.switchVillage);
  const rename = useGameStore((s) => s.renameVillage);
  const coins = useGameStore((s) => (s.game!.config.noCoins ? 0 : humanCoins(s.game!)));
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState('');

  const storageCap = useGameStore(selectActiveStorageCap);

  const popRatio = popCap > 0 ? popUsed / popCap : 1;
  const multi = villages.length > 1;

  const nameEl = editing ? (
    <form onSubmit={(e) => { e.preventDefault(); rename(villageId, draft); setEditing(false); }} className="inline">
      <input autoFocus className="px-1 border border-wood-border rounded bg-white/60 font-body text-[13px]" value={draft} onChange={(e) => setDraft(e.target.value)} onBlur={() => { rename(villageId, draft); setEditing(false); }} />
    </form>
  ) : (
    <strong className="cursor-text" title={t('header.rename')} onClick={() => { setDraft(name); setEditing(true); }}>{name}</strong>
  );

  return (
    <header className="panel">
      <div className="panel-header flex flex-wrap items-center gap-x-3 gap-y-1">
        <span className="text-gold" aria-hidden>⚜</span>
        <span className="flex items-center gap-1">
          {t('header.village')}
          {nameEl}
          {!name.includes(formatCoords(x, y)) && <span>({formatCoords(x, y)})</span>}
          {bonusKind && <span className="text-gold" title={t('header.bonus')}>★</span>}
        </span>
        {multi && (
          <span className="flex items-center gap-1 font-body text-[12px]">
            <button className="btn !px-1.5" title={t('header.prevVillage')} onClick={() => switchVillage(-1)}>‹</button>
            <select className="px-1 max-w-[8rem] sm:max-w-none border border-wood-border rounded bg-white/60" value={villageId} onChange={(e) => setActive(e.target.value)}>
              {villages.map((row) => <option key={row.id} value={row.id}>{row.name.includes(`(${row.x}|${row.y})`) ? row.name : `${row.name} (${row.x}|${row.y})`}</option>)}
            </select>
            <button className="btn !px-1.5" title={t('header.nextVillage')} onClick={() => switchVillage(1)}>›</button>
            <span className="text-muted hidden sm:inline">{t('header.villageCount', { count: villages.length })}</span>
          </span>
        )}
        {coins > 0 && <span className="font-body text-[12px]" title={t('header.coins')}><PixelIcon name="coin" size={16} /> {coins}</span>}
        <LanguageSwitch className="ml-auto font-body text-[11px] gap-0.5" buttonClassName="btn !px-1 !py-0 opacity-70" onChange={(locale) => useGameStore.getState().setLocale(locale)} />
      </div>
      <div className="px-2 sm:px-3 py-1.5 sm:py-2 flex flex-wrap items-center gap-x-3 sm:gap-x-6 gap-y-1">
        <ResourceBar />
        <span
          className={`flex items-center gap-1 ${popRatio >= 0.9 ? 'text-bad font-semibold' : ''}`}
          title={t('header.population')}
        >
          <PopIcon />
          {formatNumber(popUsed)}/{formatNumber(popCap)}
        </span>
        <span className="flex items-center gap-1" title={t('header.points')}>
          <PixelIcon name="star" size={16} />
          {t('header.pointsShort', { n: formatNumber(points) })}
        </span>
        <WorldClock />
      </div>
      <InstantBar />
    </header>
  );
}
