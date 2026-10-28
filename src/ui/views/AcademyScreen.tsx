import { checkMint, type MintBlocker } from '../../engine/conquest';
import { coinsNeededFor, nobleCountOf, nobleLimit } from '../../engine/units';
import { selectActiveVillage, useGameStore } from '../../state/gameStore';
import { CostRow } from '../components/CostRow';
import { formatNumber } from '../format';
import { RecruitScreen } from './RecruitScreen';
import { humanVillageIds, humanCoins, humanNoblesTrained } from '../../engine/players';
import { HUMAN_ID } from '../../engine/types';
import { t, useT } from '../../i18n/i18n';
import { RichText } from '../components/RichText';

function blockerText(b: MintBlocker): string {
  return b.kind === 'academy' ? t('academy.needsAcademy') : t('academy.noResources');
}

export function AcademyScreen() {
  const villageId = useGameStore((s) => selectActiveVillage(s).id);
  const mint = useGameStore((s) => s.mint);
  const config = useGameStore((s) => s.game!.config);
  useT();
  const json = useGameStore((s) => {
    const g = s.game!;
    const v = selectActiveVillage(s);
    const c = checkMint(g, v);
    return JSON.stringify({
      coins: humanCoins(g),
      villages: humanVillageIds(g).length,
      cost: c.cost,
      blockers: c.blockers,
      have: { wood: Math.floor(v.resources.wood), clay: Math.floor(v.resources.clay), iron: Math.floor(v.resources.iron) },
      nobles: nobleCountOf(v, g),
      limit: nobleLimit(g.config, v),
      minted: humanNoblesTrained(g),
      nextCoins: coinsNeededFor(g, HUMAN_ID, 1),
    });
  });
  const m = JSON.parse(json) as {
    coins: number; villages: number; cost: { wood: number; clay: number; iron: number }; blockers: MintBlocker[];
    have: { wood: number; clay: number; iron: number }; nobles: number; limit: number; minted: number; nextCoins: number;
  };
  return (
    <div className="space-y-4">
      {!config.noCoins && (
        <div className="panel p-3 text-[12px] space-y-2">
          <h3 className="text-[15px]">{t('academy.coinsTitle')}</h3>
          <div><RichText text={t('academy.youHave', { coins: m.coins, minted: m.minted })} /></div>
          <div className="flex items-center gap-3 flex-wrap">
            <span>{t('academy.coinCost')}</span>
            <CostRow cost={m.cost} have={m.have} />
            <button className="btn" disabled={m.blockers.length > 0} title={m.blockers.map(blockerText).join('\n') || t('academy.mint')} onClick={() => mint(villageId)}>{t('academy.mint')}</button>
          </div>
          <div className="text-muted"><RichText text={t('academy.nextNoble', { coins: m.nextCoins, villages: m.villages, nobles: m.nobles, limit: m.limit })} /></div>
        </div>
      )}
      <div>
        <h3 className="text-[15px] mb-1">{t('academy.noble')}</h3>
        <RecruitScreen building="academy" />
      </div>
      <p className="text-[11px] text-muted">
        {config.noCoins
          ? t('academy.rulesNoCoins', { min: config.ownedNobleDropMin ?? 20, max: config.ownedNobleDropMax ?? 35 })
          : t('academy.rules', { wood: formatNumber(28000), clay: formatNumber(30000), iron: formatNumber(25000) })}
      </p>
    </div>
  );
}
