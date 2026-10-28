import { useState } from 'react';
import { balanceAmount, canAcceptTrader, checkTrade, merchantsAvailable, merchantsTotal, surplus, type TradeBlocker } from '../../engine/market';
import { RESOURCES, type Resource, type ResourceSet } from '../../engine/types';
import { selectActiveVillage, useGameStore } from '../../state/gameStore';
import { Countdown } from '../components/Countdown';
import { ResourceIcon } from '../components/ResourceIcon';
import { formatCoords, formatDuration, formatNumber } from '../format';
import { humanVillageIds } from '../../engine/players';
import { t, useT } from '../../i18n/i18n';
import { villageLabelOf } from '../villageName';

function blockerText(b: TradeBlocker): string {
  switch (b.kind) {
    case 'noMarket': return t('market.noMarket');
    case 'notOwned': return t('market.notOwned');
    case 'self': return t('market.self');
    case 'empty': return t('market.empty');
    case 'resources': return t('market.noResources');
    case 'merchants': return t('market.merchantsShort', { needed: b.needed, available: b.available });
  }
}

export function MarketScreen() {
  const villageId = useGameStore((s) => selectActiveVillage(s).id);
  const others = useGameStore((s) => JSON.stringify(humanVillageIds(s.game!).filter((id) => id !== villageId).map((id) => { const v = s.game!.villages[id]!; return { id, label: `${villageLabelOf(s.game!, v)} ${formatCoords(v.x, v.y)}` }; })));
  const list = JSON.parse(others) as { id: string; label: string }[];
  const [target, setTarget] = useState(list[0]?.id ?? '');
  const [amount, setAmount] = useState<ResourceSet>({ wood: 0, clay: 0, iron: 0 });
  const trade = useGameStore((s) => s.trade);
  useT();
  const accept = useGameStore((s) => s.acceptTraderOffer);
  const info = useGameStore((s) => {
    const g = s.game!;
    const v = selectActiveVillage(s);
    const to = target ? g.villages[target] : undefined;
    const check = to ? checkTrade(g, v.id, target, amount) : null;
    return JSON.stringify({
      total: merchantsTotal(v), free: merchantsAvailable(g, v),
      check, surplus: surplus(v), balance: to ? balanceAmount(v, to) : null,
      trader: g.trader, canAccept: canAcceptTrader(g, v),
      have: { wood: Math.floor(v.resources.wood), clay: Math.floor(v.resources.clay), iron: Math.floor(v.resources.iron) },
    });
  });
  const m = JSON.parse(info) as {
    total: number; free: number; check: ReturnType<typeof checkTrade> | null; surplus: ResourceSet; balance: ResourceSet | null;
    trader: { offer: { give: Resource; giveAmount: number; get: Resource; getAmount: number }; expiresAt: number; accepted: boolean }; canAccept: boolean; have: ResourceSet;
  };
  const setRes = (r: Resource, n: number) => setAmount((a) => ({ ...a, [r]: Math.max(0, Math.floor(n) || 0) }));

  return (
    <div className="space-y-4 text-[12px]">
      <div className="panel p-3 space-y-2">
        <h3 className="text-[15px]">{t('market.merchants', { free: m.free, total: m.total })} <span className="text-muted font-body text-[11px]">{t('market.merchantHint')}</span></h3>
        {list.length === 0 ? (
          <p className="text-muted italic">{t('market.ownOnly')}</p>
        ) : (
          <>
            <label>{t('market.toVillage')} <select className="px-1 border border-wood-border rounded bg-white/50" value={target} onChange={(e) => setTarget(e.target.value)}>{list.map((o) => <option key={o.id} value={o.id}>{o.label}</option>)}</select></label>
            <div className="flex flex-wrap gap-3 items-center">
              {RESOURCES.map((r) => (
                <label key={r} className="inline-flex items-center gap-1"><ResourceIcon r={r} size={14} />
                  <input type="number" min={0} className="w-24 px-1 border border-wood-border rounded bg-white/50 tabular-nums" value={amount[r]} onChange={(e) => setRes(r, Number(e.target.value))} />
                  <span className="text-muted">/ {formatNumber(m.have[r])}</span>
                </label>
              ))}
            </div>
            <div className="flex flex-wrap gap-2 items-center">
              <button className="btn" onClick={() => setAmount(m.surplus)} title={t('market.surplusTitle')}>{t('market.surplus')}</button>
              <button className="btn" onClick={() => m.balance && setAmount(m.balance)} title={t('market.balanceTitle')}>{t('market.balance')}</button>
              <button className="btn" onClick={() => setAmount({ wood: 0, clay: 0, iron: 0 })}>{t('market.clear')}</button>
              {m.check && <span className="text-muted">{t('market.sendInfo', { merchants: m.check.merchants, time: formatDuration(m.check.travelMs) })}</span>}
              <button className="btn ml-auto" disabled={!m.check || m.check.blockers.length > 0} title={m.check?.blockers.map(blockerText).join('\n')} onClick={async () => { if (!(await trade(villageId, target, amount))) setAmount({ wood: 0, clay: 0, iron: 0 }); }}>{t('market.send')}</button>
            </div>
            {m.check && m.check.blockers.length > 0 && <div className="text-muted">{m.check.blockers.map(blockerText).join(' · ')}</div>}
          </>
        )}
      </div>
      <div className="panel p-3 space-y-1">
        <h3 className="text-[15px]">{t('market.trader')} <span className="text-muted font-body text-[11px]">{t('market.nextOffer')} <Countdown until={m.trader.expiresAt} />)</span></h3>
        <div className="flex items-center gap-2 flex-wrap">
          <span>{t('market.give')} <strong className="inline-flex items-center gap-0.5"><ResourceIcon r={m.trader.offer.give} size={14} />{t(`resourceAmount.${m.trader.offer.give}`, { n: formatNumber(m.trader.offer.giveAmount) })}</strong></span>
          <span>{t('market.get')} <strong className="inline-flex items-center gap-0.5"><ResourceIcon r={m.trader.offer.get} size={14} />{t(`resourceAmount.${m.trader.offer.get}`, { n: formatNumber(m.trader.offer.getAmount) })}</strong></span>
          <button className="btn ml-auto" disabled={!m.canAccept} title={m.trader.accepted ? t('market.used') : m.canAccept ? t('market.exchange') : t('market.cannotExchange')} onClick={() => accept(villageId)}>{m.trader.accepted ? t('market.usedShort') : t('market.exchange')}</button>
        </div>
      </div>
    </div>
  );
}
