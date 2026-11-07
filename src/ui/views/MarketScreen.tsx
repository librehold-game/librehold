import { useState } from 'react';
import { balanceAmount, canAcceptTrader, checkTrade, merchantsAvailable, merchantsTotal, surplus, tradeTravelMs, tradeTravelMsByCoords, type TradeBlocker } from '../../engine/market';
import { distance } from '../../engine/world';
import { HUMAN_ID, RESOURCES, type GameState, type Resource, type ResourceSet, type TradeOffer } from '../../engine/types';
import { acceptBlockers, myOffers, normalizeDelivery, offerActions, publicBoard } from '../../engine/trade';
import { selectActiveVillage, useGameStore } from '../../state/gameStore';
import { Countdown } from '../components/Countdown';
import { ResourceIcon } from '../components/ResourceIcon';
import { formatCoords, formatDuration, formatDurationHuman, formatNumber } from '../format';
import { humanVillageIds } from '../../engine/players';
import { t, useT } from '../../i18n/i18n';
import { villageLabelOf } from '../villageName';
import { NumberInput } from '../../shared/NumberInput';
import { BotTag } from '../components/BotTag';
import { TRADE_BALANCE } from '../../data/world.config';
import { ResourceList } from '../components/ResourceList';
import { MovementsSection } from './MovementsPanel';

/** The most identical copies one post may create. Each copy is a separate order, so this bounds the work a single
 * click can ask of the server; it also keeps the form honest about an accidental extra zero. */
const POST_COPIES_MAX = 50;

function blockerText(b: TradeBlocker): string {
  switch (b.kind) {
    case 'noMarket': return t('market.noMarket');
    case 'notOwned': return t('market.notOwned');
    case 'self': return t('market.self');
    case 'empty': return t('market.empty');
    case 'resources': return t('market.noResources');
    case 'merchants': return t('market.merchantsShort', { needed: b.needed, available: b.available });
    case 'marketTooSmall': return t('market.marketTooSmall', { needed: b.needed, total: b.total });
    case 'tradingOff': return t('market.tradingOff');
  }
}

/** What the market says to a player who presses Accept while the deal cannot go through. */
function blockedMessage(blockers: TradeBlocker[]): string {
  return blockers.map(blockerText).join(' · ');
}

/** Other players' villages are not in `state.villages`; recover their tile from the id ("v:x:y"). */
export function parseVillageIdCoords(id: string): { x: number; y: number } | null {
  const m = /^v:(\d+):(\d+)$/.exec(id);
  if (!m) return null;
  return { x: Number(m[1]), y: Number(m[2]) };
}

/** One row per distinct deal in the trade-offers panel: identical offers (same sender, same delivery, same amounts)
 * collapsed into a single line. Accepting takes one copy; withdrawing takes as many as the player asks for. */
interface OfferGroup {
  offer: TradeOffer;
  ids: string[];
  travelMs: number | null;
  ratio: number;
}

/** Groups the offers of my own list by deal, keeping every copy. */
function groupOffers(g: GameState, offers: TradeOffer[], villageId: string): OfferGroup[] {
  // Keyed by deal so each offer is hashed once; the map also keeps first-seen order, which is the order the rows show.
  const byKey = new Map<string, OfferGroup>();
  for (const o of offers) {
    const travelMs = offerTravelMs(g, o, villageId);
    const key = dealKey(o, travelMs);
    const gr = byKey.get(key);
    if (gr) gr.ids.push(o.id);
    else {
      const give = RESOURCES.reduce((n, r) => n + o.give[r], 0);
      const want = RESOURCES.reduce((n, r) => n + o.want[r], 0);
      byKey.set(key, { offer: o, ids: [o.id], travelMs, ratio: give > 0 ? want / give : 0 });
    }
  }
  return [...byKey.values()];
}

/** The merchant trip a deal needs, or null when it cannot be known yet (my own public post has no taker). */
function offerTravelMs(g: GameState, offer: TradeOffer, villageId: string): number | null {
  const origin = g.villages[offer.originId] ?? parseVillageIdCoords(offer.originId);
  if (offer.fromId === HUMAN_ID) {
    // My own post: who takes it is not known yet, so there is no trip to show.
    return null;
  }
  const dest = g.villages[villageId];
  return origin && dest ? tradeTravelMsByCoords(origin, dest, g.config.unitSpeed) : null;
}

/** A row of the trade-offers table. It reads the store itself so both offer lists can share it. */
function OfferRow({ group, villageId, showOrigin }: { group: OfferGroup; villageId: string; showOrigin: boolean }) {
  const game = useGameStore((s) => s.game);
  const acceptOffer = useGameStore((s) => s.acceptTradeOffer);
  const cancelOffer = useGameStore((s) => s.cancelTradeOffer);
  const pushError = useGameStore((s) => s.pushError);
  const [pickVillage, setPickVillage] = useState('');
  const [withdrawing, setWithdrawing] = useState(1);
  if (!game) return null;
  const g = game;
  const me = HUMAN_ID;
  const offer = group.offer;
  const ownIds = (g.players[me]?.villageIds ?? []).filter((id) => g.villages[id]);
  const available = group.ids.length;
  const withdrawCount = Math.min(Math.max(1, withdrawing), available);
  // My own offers are all my own name, so the second column carries the village that posts them instead — the market
  // that supplies the merchants and owns the per-village cap. Coordinates too, since a player's villages can share a
  // name. Someone else's offer names the player.
  const originVillage = g.villages[offer.originId];
  const originCoords = originVillage ?? parseVillageIdCoords(offer.originId);
  const originLabel = originVillage ? `${villageLabelOf(g, originVillage)} ${formatCoords(originVillage.x, originVillage.y)}`
    : originCoords ? formatCoords(originCoords.x, originCoords.y) : '?';
  // The max delivery limit I set when posting, not the merchant trip: the trip is meaningless on my own offer (a public
  // post has no taker yet, so it would read as a dash). 0 means no limit, as the post form's field says.
  const maxDelivery = offer.maxDeliveryMs > 0
    ? formatDurationHuman(offer.maxDeliveryMs)
    : t('market.deliveryNone');

  // A deal settles at whichever of my villages takes it.
  const pick = pickVillage || ownIds[0] || '';

  const actions = offerActions(offer, me);
  const blockers = actions.includes('accept') && pick ? acceptBlockers(g, offer, pick) : [];

  return (
    <tr>
      <td data-label={t('market.thOffer')}><ResourceList set={offer.give} /></td>
      <td data-label={t('market.thFor')}><ResourceList set={offer.want} tone={(r) => {
        const v = g.villages[pick];
        if (!v || actions.includes('cancel')) return '';
        return offer.want[r] <= Math.floor(v.resources[r]) ? 'text-green-700' : 'text-red-700';
      }} /></td>
      <td data-label={showOrigin ? t('market.thFromVillage') : t('market.thFrom')} className="font-semibold whitespace-nowrap">
        {showOrigin ? originLabel : (
          <>
            {g.players[offer.fromId]?.name ?? '?'}{g.players[offer.fromId]?.isHuman === false && <BotTag />}
          </>
        )}
        {available > 1 && <span className="text-muted font-body"> ×{available}</span>}
      </td>
      <td data-label={showOrigin ? t('market.thMaxDelivery') : t('market.thTime')} className="whitespace-nowrap tabular-nums text-muted">
        {showOrigin ? maxDelivery : group.travelMs !== null ? formatDurationHuman(group.travelMs) : '—'}
      </td>
      <td data-label={t('market.thRatio')} className="tabular-nums text-muted">{(Math.round(group.ratio * 100) / 100).toString()}</td>
      <td data-label={t('market.thExpires')} className="whitespace-nowrap tabular-nums text-muted"><Countdown until={offer.expiresAt} /></td>
      <td className="text-right whitespace-nowrap stack-inline action-end">
        <span className="inline-flex items-center gap-1 justify-end">
          {actions.includes('accept') && (
            <>
              {ownIds.length > 1 && (
                <select className="px-1 border border-wood-border rounded bg-white/50" value={pick} onChange={(e) => setPickVillage(e.target.value)}>
                  {ownIds.map((id) => <option key={id} value={id}>{villageLabelOf(g, g.villages[id]!)}</option>)}
                </select>
              )}
              <button className="btn" disabled={!pick} title={blockers.map(blockerText).join('\n')}
                onClick={() => (blockers.length > 0 ? pushError(blockedMessage(blockers)) : acceptOffer(offer.id, pick))}>
                {t('market.accept')}
              </button>
            </>
          )}
          {actions.includes('cancel') && (
            <>
              {available > 1 && (
                <NumberInput min={1} max={available} step={1} value={withdrawCount}
                  className="w-12 px-1 border border-wood-border rounded bg-white/50 tabular-nums text-right"
                  onValue={(n) => setWithdrawing(n)} />
              )}
              <button className="btn" onClick={() => group.ids.slice(0, withdrawCount).forEach((id) => cancelOffer(id))}>
                {available > 1 ? t('market.withdrawMany', { n: withdrawCount }) : t('market.withdraw')}
              </button>
            </>
          )}
        </span>
      </td>
    </tr>
  );
}

/** The offers I have posted, plus the form that posts a new one. */
function OffersPanel({ villageId }: { villageId: string }) {
  const game = useGameStore((s) => s.game);
  const createOffer = useGameStore((s) => s.createTradeOffer);
  const pushError = useGameStore((s) => s.pushError);
  useT();
  const [form, setForm] = useState<{ maxHours: number; copies: number; give: ResourceSet; want: ResourceSet }>({
    maxHours: 0, copies: 1, give: { wood: 0, clay: 0, iron: 0 }, want: { wood: 0, clay: 0, iron: 0 },
  });

  if (!game) return null;
  const g = game;

  /** Posts the deal `n` times: identical copies, so several takers can each take a smaller piece. Each goes through
   * the ordinary create order, so the cap and the resource reservation apply to every copy on its own; a refusal
   * stops the run, since every later copy would be refused for the same reason. */
  const [postedMsg, setPostedMsg] = useState('');
  const postMany = async (n: number) => {
    const args = {
      originId: villageId,
      give: form.give, want: form.want, maxDeliveryMs: Math.round(form.maxHours * 3_600_000),
    };
    let done = 0;
    for (let i = 0; i < n; i++) {
      if (await createOffer(args)) break;
      done++;
    }
    setPostedMsg(done === n ? '' : t('market.postedSome', { done, n }));
  };
  const setGive = (r: Resource, n: number) => setForm((f) => ({ ...f, give: { ...f.give, [r]: Math.max(0, Math.floor(n) || 0) } }));
  const setWant = (r: Resource, n: number) => setForm((f) => ({ ...f, want: { ...f.want, [r]: Math.max(0, Math.floor(n) || 0) } }));

  const postBlockers: string[] = [];
  if (RESOURCES.reduce((n, r) => n + form.give[r] + form.want[r], 0) === 0) postBlockers.push(t('market.empty'));
  if (RESOURCES.some((r) => form.give[r] > 0 && form.want[r] > 0)) postBlockers.push(t('market.sameResource'));

  // One post may go up several times, so several takers can each take a smaller piece. The cap counts *distinct*
  // deals of this village (server: PublicPerVillage per origin), so more copies of a deal already out are free of
  // it — mirror that here so the form does not block a legitimate copy.
  // Bounded: every copy is its own order, so an accidental "1000" would fire a thousand serial requests. No rule
  // caps this otherwise (copies are free of the cap), so the form has to.
  const copies = Math.min(POST_COPIES_MAX, Math.max(1, Math.floor(form.copies) || 1));
  // The delivery limit is part of a deal's identity on the server (dealKey), so a different limit is a new post:
  // leaving it out here would promise a free copy of a post the server counts against the cap.
  const deliveryMs = normalizeDelivery(Math.round(form.maxHours * 3_600_000));
  const sameDeal = (o: TradeOffer) => o.fromId === HUMAN_ID && o.originId === villageId
    && o.maxDeliveryMs === deliveryMs
    && RESOURCES.every((r) => o.give[r] === form.give[r] && o.want[r] === form.want[r]);
  // Same key the server counts (postKey: amounts, delivery limit), kept to this village because its merchants
  // deliver and so its own posts are what fill its slots.
  const myDeals = new Set(g.tradeOffers.filter((o) => o.fromId === HUMAN_ID && o.originId === villageId)
    .map((o) => `${RESOURCES.map((r) => `${o.give[r]}:${o.want[r]}`).join(',')}|${o.maxDeliveryMs}`)).size;
  const freeCopies = g.tradeOffers.some(sameDeal);
  const atCap = myDeals >= TRADE_BALANCE.publicPerVillage;
  /** The refusal a new deal meets once this village's slots are taken; a copy of a deal already out is still free. */
  const capText = t('market.boardFullHint', { max: TRADE_BALANCE.publicPerVillage });
  if (atCap && !freeCopies) postBlockers.push(capText);
  // Every copy reserves its share up front, so the village must hold the whole batch at once.
  const village = g.villages[villageId];
  if (village && RESOURCES.some((r) => form.give[r] * 1 > Math.floor(village.resources[r]))) {
    postBlockers.push(t('market.notEnoughGiven'));
  }
  const mine = groupOffers(g, myOffers(g), villageId);
  // The second column names the village that posts the offer, since the player would just repeat my own name on
  // every row; the fourth shows the delivery limit I set, because a post I made has no taker yet to time a trip to.
  const columns = (second: string, fourth: string) => (
    <tr>
      <th>{t('market.thOffer')}</th>
      <th>{t('market.thFor')}</th>
      <th>{second}</th>
      <th>{fourth}</th>
      <th>{t('market.thRatio')}</th>
      <th>{t('market.thExpires')}</th>
      <th className="text-right">{t('market.thAccept')}</th>
    </tr>
  );

  return (
    <div className="panel p-3 space-y-2">
      <h3 className="text-[15px]">{t('market.offersTitle')} <span className="text-muted font-body text-[11px]">{t('market.dealHint')}</span></h3>
      {mine.length > 0 && (
        <div className="space-y-1">
          <div className="overflow-x-auto">
            <table className="table-game table-stack">
              <thead>{columns(t('market.thFromVillage'), t('market.thMaxDelivery'))}</thead>
              <tbody>{mine.map((gr) => <OfferRow key={gr.ids[0]} group={gr} villageId={villageId} showOrigin />)}</tbody>
            </table>
          </div>
        </div>
      )}
      {mine.length === 0 ? (
        <p className="text-muted italic">{t('market.offersEmpty')}</p>
      ) : null}
      <details className={g.tradeOffers.length === 0 ? '' : 'pt-1'}
        onToggle={(e) => { if (e.currentTarget.open && atCap && !freeCopies) pushError(capText); }}>
        <summary className="cursor-pointer font-medium">{t('market.postOffer')}</summary>
        <div className="mt-2 space-y-2">
          <label>{t('market.deliveryLabel')}{' '}
            <NumberInput min={0} step={1} className="w-16 px-1 border border-wood-border rounded bg-white/50 tabular-nums"
              value={form.maxHours} onValue={(n) => setForm((f) => ({ ...f, maxHours: n }))} />{' '}
            {t('market.boardHours')} <span className="text-muted text-[11px]">{t('market.deliveryUnlimited')}</span>
          </label>
          <div className="space-y-1">
            <div className="flex flex-wrap gap-3 items-center">
              <span className="w-24 shrink-0">{t('market.give')}</span>
              {RESOURCES.map((r) => (
                <label key={r} className="inline-flex items-center gap-1"><ResourceIcon r={r} size={14} />
                  <NumberInput min={0} className="w-20 px-1 border border-wood-border rounded bg-white/50 tabular-nums" value={form.give[r]} onValue={(n) => setGive(r, n)} />
                </label>
              ))}
            </div>
            <div className="flex flex-wrap gap-3 items-center">
              <span className="w-24 shrink-0">{t('market.get')}</span>
              {RESOURCES.map((r) => (
                <label key={r} className="inline-flex items-center gap-1"><ResourceIcon r={r} size={14} />
                  <NumberInput min={0} className="w-20 px-1 border border-wood-border rounded bg-white/50 tabular-nums" value={form.want[r]} onValue={(n) => setWant(r, n)} />
                </label>
              ))}
            </div>
          </div>
          <label className="inline-flex items-center gap-1.5">
            {t('market.postHowMany')}{' '}
            <NumberInput min={1} max={POST_COPIES_MAX} step={1}
              className="w-14 px-1 border border-wood-border rounded bg-white/50 tabular-nums"
              value={form.copies} onValue={(n) => setForm((f) => ({ ...f, copies: n }))} />
            {freeCopies && <span className="text-muted text-[11px]">{t('market.copiesFree')}</span>}
          </label>
          <span className="inline-flex items-center gap-2">
            <button className="btn" disabled={postBlockers.length > 0} title={postBlockers.join('\n')}
              onClick={async () => { await postMany(copies); }}>
              {copies > 1 ? t('market.postMany', { n: copies }) : t('market.post')}
            </button>
            {postBlockers.length > 0 && <span className="text-muted">{postBlockers.join(' · ')}</span>}
            {postedMsg && <span className="text-bad">{postedMsg}</span>}
          </span>
        </div>
      </details>
    </div>
  );
}

interface BoardRow {
  offer: TradeOffer;
  name: string;
  isBot: boolean;
  travelMs: number | null;
  ratio: number; // what you receive per unit you give, e.g. 0.7
}

/** A row of the global board: identical offers (same player, same delivery time) collapsed into one line,
 * with every copy kept so a taker can pick up more than one at once. */
interface BoardGroup {
  offer: TradeOffer;
  ids: string[];
  name: string;
  isBot: boolean;
  travelMs: number | null;
  ratio: number;
}

/** The global board: public offers from everyone in range as a filterable table. Duplicate offers by the
 * same player at the same delivery time are grouped into one row that can be taken N times over. */
function BoardPanel() {
  const game = useGameStore((s) => s.game);
  useT();
  const [page, setPage] = useState(0);
  const [search, setSearch] = useState('');
  const [resFilter, setResFilter] = useState<'any' | Resource>('any');
  const [giveFilter, setGiveFilter] = useState<'any' | Resource>('any');
  const [posterFilter, setPosterFilter] = useState<'any' | 'human' | 'bot'>('any');
  const [maxHours, setMaxHours] = useState('');
  const [minRatio, setMinRatio] = useState('');
  const [sortBy, setSortBy] = useState<'time' | 'expiry' | 'ratio'>('time');

  if (!game) return null;
  const g = game;
  const me = HUMAN_ID;
  const ownIds = (g.players[me]?.villageIds ?? []).filter((id) => g.villages[id]);
  const defaultVillage = ownIds[0] ?? '';

  let rows: BoardRow[] = publicBoard(g).map((o) => {
    const giveTotal = RESOURCES.reduce((n, r) => n + o.give[r], 0);
    const wantTotal = RESOURCES.reduce((n, r) => n + o.want[r], 0);
    const poster = g.players[o.fromId];
    return { offer: o, name: poster?.name ?? '?', isBot: poster?.isHuman === false, travelMs: offerTravelMs(g, o, defaultVillage), ratio: giveTotal > 0 ? wantTotal / giveTotal : 0 };
  });

  const q = search.trim().toLowerCase();
  if (q) rows = rows.filter((r) => r.name.toLowerCase().includes(q));
  if (resFilter !== 'any') rows = rows.filter((r) => r.offer.want[resFilter] > 0);
  if (giveFilter !== 'any') rows = rows.filter((r) => r.offer.give[giveFilter] > 0);
  if (posterFilter !== 'any') rows = rows.filter((r) => r.isBot === (posterFilter === 'bot'));
  const maxMs = Number(maxHours) * 3_600_000;
  if (maxHours && !Number.isNaN(maxMs)) rows = rows.filter((r) => r.travelMs !== null && r.travelMs <= maxMs);
  if (minRatio && !Number.isNaN(Number(minRatio))) rows = rows.filter((r) => r.ratio >= Number(minRatio));

  const byTime = (a: BoardRow, b: BoardRow) => (a.travelMs ?? Infinity) - (b.travelMs ?? Infinity);
  rows.sort(sortBy === 'time' ? byTime : sortBy === 'expiry' ? (a, b) => a.offer.expiresAt - b.offer.expiresAt : (a, b) => a.ratio - b.ratio || byTime(a, b));

  // One row per distinct deal: same seller, same delivery time, same amounts. Copies stay grouped so the row
  // can offer "how many of these do I take".
  const byKey = new Map<string, BoardGroup>();
  for (const r of rows) {
    const key = dealKey(r.offer, r.travelMs);
    const gr = byKey.get(key);
    if (gr) gr.ids.push(r.offer.id);
    else byKey.set(key, { offer: r.offer, ids: [r.offer.id], name: r.name, isBot: r.isBot, travelMs: r.travelMs, ratio: r.ratio });
  }
  const groups = [...byKey.values()];

  const perPage = 10;
  const pages = Math.max(1, Math.ceil(groups.length / perPage));
  const p = Math.min(page, pages - 1);
  const pageGroups = groups.slice(p * perPage, (p + 1) * perPage);

  return (
    <div className="panel p-3 space-y-2">
      <h3 className="text-[15px]">{t('market.boardTitle')} <span className="text-muted font-body text-[11px]">{t('market.boardHint')}</span></h3>
      <div className="flex flex-wrap items-center gap-x-4 gap-y-1.5">
        <label className="inline-flex items-center gap-1.5">
          {t('market.boardSearch')}
          <input type="text" value={search} onChange={(e) => { setSearch(e.target.value); setPage(0); }} placeholder={g.players[me]?.name ?? ''}
            className="w-32 px-1 border border-wood-border rounded bg-white/50" />
        </label>
        <label className="inline-flex items-center gap-1.5">
          {t('market.boardPoster')}
          <select className="px-1 border border-wood-border rounded bg-white/50" value={posterFilter} onChange={(e) => { setPosterFilter(e.target.value as typeof posterFilter); setPage(0); }}>
            <option value="any">{t('market.boardPosterAny')}</option>
            <option value="human">{t('market.boardPosterHuman')}</option>
            <option value="bot">{t('market.boardPosterBot')}</option>
          </select>
        </label>
        <label className="inline-flex items-center gap-1.5">
          {t('market.boardWants')}
          <select className="px-1 border border-wood-border rounded bg-white/50" value={resFilter} onChange={(e) => { setResFilter(e.target.value as 'any' | Resource); setPage(0); }}>
            <option value="any">{t('market.boardAny')}</option>
            {RESOURCES.map((r) => <option key={r} value={r}>{t(`resource.${r}`)}</option>)}
          </select>
        </label>
        <label className="inline-flex items-center gap-1.5">
          {t('market.boardGives')}
          <select className="px-1 border border-wood-border rounded bg-white/50" value={giveFilter} onChange={(e) => { setGiveFilter(e.target.value as 'any' | Resource); setPage(0); }}>
            <option value="any">{t('market.boardAny')}</option>
            {RESOURCES.map((r) => <option key={r} value={r}>{t(`resource.${r}`)}</option>)}
          </select>
        </label>
        <label className="inline-flex items-center gap-1.5">
          {t('market.boardMaxTime')}
          <NumberInput value={Number(maxHours) || 0} onValue={(n) => { setMaxHours(n === 0 ? '' : String(n)); setPage(0); }}
            min={0} step={1} className="w-14 px-1 border border-wood-border rounded bg-white/50 tabular-nums" />
          {t('market.boardHours')}
        </label>
        <label className="inline-flex items-center gap-1.5">
          {t('market.boardMinRatio')}
          <NumberInput value={Number(minRatio) || 0} onValue={(n) => { setMinRatio(n === 0 ? '' : String(n)); setPage(0); }}
            min={0} step={0.1} className="w-14 px-1 border border-wood-border rounded bg-white/50 tabular-nums" />
        </label>
        <label className="inline-flex items-center gap-1.5">
          {t('market.boardOrderBy')}
          <select className="px-1 border border-wood-border rounded bg-white/50" value={sortBy} onChange={(e) => setSortBy(e.target.value as typeof sortBy)}>
            <option value="time">{t('market.sortTime')}</option>
            <option value="expiry">{t('market.sortExpiry')}</option>
            <option value="ratio">{t('market.sortRatio')}</option>
          </select>
        </label>
      </div>
      {groups.length === 0 ? (
        <p className="text-muted italic">{t('market.boardEmpty')}</p>
      ) : (
        <>
          <table className="table-game table-stack">
            <thead>
              <tr>
                <th>{t('market.thOffer')}</th>
                <th>{t('market.thFor')}</th>
                <th>{t('market.thPlayer')}</th>
                <th>{t('market.thTime')}</th>
                <th className="text-right">{t('market.thAvailable')}</th>
                <th>{t('market.thRatio')}</th>
                <th className="text-right">{t('market.thAccept')}</th>
              </tr>
            </thead>
            <tbody>
              {pageGroups.map((gr) => (
                <BoardTableRow group={gr} key={gr.ids[0]} />
              ))}
            </tbody>
          </table>
          {pages > 1 && (
            <div className="flex items-center justify-end gap-2 mt-1">
              <button className="btn" disabled={p === 0} onClick={() => setPage(p - 1)}>{t('market.pagePrev')}</button>
              <span className="text-muted font-body text-[10px]">{p + 1} / {pages}</span>
              <button className="btn" disabled={p >= pages - 1} onClick={() => setPage(p + 1)}>{t('market.pageNext')}</button>
            </div>
          )}
        </>
      )}
    </div>
  );
}

/** Identity of one deal, ignoring how many copies of it are out: same poster, same delivery, same amounts. */
function dealKey(offer: TradeOffer, travelMs: number | null): string {
  // The origin and the delivery limit belong in the key: they are what the server's postKey uses to tell two deals
  // apart, so identical amounts from different villages, or with different limits, are separate offers (different
  // merchants, different per-village cap) and must not collapse into one row with a single ×N and one limit shown.
  return `${offer.fromId}|${offer.originId}|${offer.maxDeliveryMs}|${travelMs ?? -1}|` +
    RESOURCES.map((r) => `${offer.give[r]}:${offer.want[r]}`).join(',');
}

/** One table row of the global board: the deal, how many copies are on offer, and a take-N input that is checked
 * against my resources and free merchants for the whole batch before anything is sent. */
function BoardTableRow({ group }: { group: BoardGroup }) {
  const game = useGameStore((s) => s.game);
  const acceptOffer = useGameStore((s) => s.acceptTradeOffer);
  const pushError = useGameStore((s) => s.pushError);
  const [pickVillage, setPickVillage] = useState('');
  const [copies, setCopies] = useState(1);
  if (!game) return null;
  const g = game;
  const ownIds = (g.players[HUMAN_ID]?.villageIds ?? []).filter((id) => g.villages[id]);
  const pick = pickVillage || ownIds[0] || '';
  const available = group.ids.length;
  const take = Math.min(Math.max(1, copies), available);
  const blockers = pick ? acceptBlockers(g, group.offer, pick, take) : [];
  // Both sides scale with the batch: what arrives and what my village pays, the latter tinted by affordability.
  const giveTotal: ResourceSet = { wood: group.offer.give.wood * take, clay: group.offer.give.clay * take, iron: group.offer.give.iron * take };
  const wantTotal: ResourceSet = { wood: group.offer.want.wood * take, clay: group.offer.want.clay * take, iron: group.offer.want.iron * take };

  return (
    <tr>
      <td data-label={t('market.thOffer')}><ResourceList set={giveTotal} /></td>
      <td data-label={t('market.thFor')}><ResourceList set={wantTotal} tone={(r) => {
        const v = g.villages[pick];
        if (!v) return '';
        return wantTotal[r] <= Math.floor(v.resources[r]) ? 'text-green-700' : 'text-red-700';
      }} /></td>
      <td data-label={t('market.thPlayer')} className="font-semibold">
        {/* One inline unit, so the bot tag stays on the name's line instead of dropping under it on a narrow card. */}
        <span className="inline-flex items-baseline whitespace-nowrap">{group.name}{group.isBot && <BotTag />}</span>
      </td>
      <td data-label={t('market.thTime')} className="whitespace-nowrap tabular-nums text-muted">{group.travelMs !== null ? formatDurationHuman(group.travelMs) : '—'}</td>
      <td data-label={t('market.thAvailable')} className="tabular-nums">×{available}</td>
      <td data-label={t('market.thRatio')} className="tabular-nums text-muted">{(Math.round(group.ratio * 100) / 100).toString()}</td>
      <td className="text-right whitespace-nowrap stack-inline action-end">
        {ownIds.length > 1 && (
          <select className="px-1 border border-wood-border rounded bg-white/50 mr-1" value={pick} onChange={(e) => setPickVillage(e.target.value)}>
            {ownIds.map((id) => <option key={id} value={id}>{villageLabelOf(g, g.villages[id]!)}</option>)}
          </select>
        )}
        <NumberInput value={take} onValue={setCopies} min={1} max={available} step={1}
          className="w-12 px-1 border border-wood-border rounded bg-white/50 tabular-nums mr-1 text-right" />
        <button className="btn" disabled={!pick} title={blockers.map(blockerText).join('\n')}
          onClick={() => (blockers.length > 0 ? pushError(blockedMessage(blockers)) : acceptOffer(group.ids.slice(0, take), pick))}>
          {t('market.accept')}
        </button>
      </td>
    </tr>
  );
}

type PanelInfo = {
  total: number; free: number; check: ReturnType<typeof checkTrade> | null; surplus: ResourceSet; balance: ResourceSet | null;
  trader: { offer: { give: Resource; giveAmount: number; get: Resource; getAmount: number }; expiresAt: number; accepted: boolean };
  canAccept: boolean; have: ResourceSet;
};

/** The plain transfer form: move resources between your own villages. It is not player trading, so it also works on a
 * world where player trading is switched off — the server allows it for the same reason. */
function TransferFields({ villageId, info: m, list, target, setTarget, amount, setAmount }: {
  villageId: string; info: PanelInfo; list: { id: string; label: string }[]; target: string;
  setTarget: (id: string) => void; amount: ResourceSet; setAmount: (fn: (a: ResourceSet) => ResourceSet) => void;
}) {
  const t = useT();
  const trade = useGameStore((s) => s.trade);
  const pushError = useGameStore((s) => s.pushError);
  const setRes = (r: Resource, n: number) => setAmount((a) => ({ ...a, [r]: Math.max(0, Math.floor(n) || 0) }));
  if (list.length === 0) return <p className="text-muted italic">{t('market.ownOnly')}</p>;
  return (
    <>
      <label>{t('market.toVillage')} <select className="px-1 border border-wood-border rounded bg-white/50" value={target} onChange={(e) => setTarget(e.target.value)}>{list.map((o) => <option key={o.id} value={o.id}>{o.label}</option>)}</select></label>
      <div className="flex flex-wrap gap-3 items-center">
        {RESOURCES.map((r) => (
          <label key={r} className="inline-flex items-center gap-1"><ResourceIcon r={r} size={14} />
            <NumberInput min={0} className="w-24 px-1 border border-wood-border rounded bg-white/50 tabular-nums" value={amount[r]} onValue={(n) => setRes(r, n)} />
            <span className="text-muted">/ {formatNumber(m.have[r])}</span>
          </label>
        ))}
      </div>
      <div className="flex flex-wrap gap-2 items-center">
        <button className="btn" onClick={() => setAmount(() => m.surplus)} title={t('market.surplusTitle')}>{t('market.surplus')}</button>
        <button className="btn" onClick={() => m.balance && setAmount(() => m.balance!)} title={t('market.balanceTitle')}>{t('market.balance')}</button>
        <button className="btn" onClick={() => setAmount(() => ({ wood: 0, clay: 0, iron: 0 }))}>{t('market.clear')}</button>
        {m.check && <span className="text-muted">{t('market.sendInfo', { merchants: m.check.merchants, time: formatDuration(m.check.travelMs) })}</span>}
        {/* Like Accept: pressable while the transfer is refused, with the reason in the toast rather than as a line
            under the row, which is long enough to be cut off on a phone. */}
        <button className="btn ml-auto" disabled={!m.check} title={m.check?.blockers.map(blockerText).join('\n')}
          onClick={async () => {
            const reasons = m.check?.blockers ?? [];
            if (reasons.length > 0) { pushError(blockedMessage(reasons)); return; }
            if (!(await trade(villageId, target, amount))) setAmount(() => ({ wood: 0, clay: 0, iron: 0 }));
          }}>
          {t('market.send')}
        </button>
      </div>
    </>
  );
}

/** The market's own recurring offer. A fixed-rate shop rather than a deal with another player, so it stays open on a
 * world with player trading switched off. */
function TravellingTrader({ villageId, trader, canAccept }: {
  villageId: string; trader: PanelInfo['trader']; canAccept: boolean;
}) {
  const t = useT();
  const accept = useGameStore((s) => s.acceptTraderOffer);
  return (
    <div className="panel p-3 space-y-1">
      <h3 className="text-[15px]">{t('market.trader')} <span className="text-muted font-body text-[11px]">{t('market.nextOffer')} <Countdown until={trader.expiresAt} />)</span></h3>
      <div className="flex items-center gap-2 flex-wrap">
        <span>{t('market.give')} <strong className="inline-flex items-center gap-0.5"><ResourceIcon r={trader.offer.give} size={14} />{t(`resourceAmount.${trader.offer.give}`, { n: formatNumber(trader.offer.giveAmount) })}</strong></span>
        <span>{t('market.get')} <strong className="inline-flex items-center gap-0.5"><ResourceIcon r={trader.offer.get} size={14} />{t(`resourceAmount.${trader.offer.get}`, { n: formatNumber(trader.offer.getAmount) })}</strong></span>
        <button className="btn ml-auto" disabled={!canAccept} title={trader.accepted ? t('market.used') : canAccept ? t('market.exchange') : t('market.cannotExchange')} onClick={() => accept(villageId)}>{trader.accepted ? t('market.usedShort') : t('market.exchange')}</button>
      </div>
    </div>
  );
}

export function MarketScreen() {
  const villageId = useGameStore((s) => selectActiveVillage(s).id);
  const others = useGameStore((s) => JSON.stringify(humanVillageIds(s.game!).filter((id) => id !== villageId).map((id) => { const v = s.game!.villages[id]!; return { id, label: `${villageLabelOf(s.game!, v)} ${formatCoords(v.x, v.y)}` }; })));
  const list = JSON.parse(others) as { id: string; label: string }[];
  const tradingOn = useGameStore((s) => s.game!.config.tradingEnabled !== false);
  const [target, setTarget] = useState(list[0]?.id ?? '');
  const [amount, setAmount] = useState<ResourceSet>({ wood: 0, clay: 0, iron: 0 });
  useT();
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

  // With trading switched off the market looks like it did before players could trade with each other: the merchants
  // line, moving resources between your own villages, and the travelling trader. Only the board and the offers that
  // came with it are gone, with one line saying why.
  if (!tradingOn) {
    return (
      <div className="space-y-4 text-[12px]">
        <div className="panel p-3 space-y-2">
          <h3 className="text-[15px]">{t('market.merchants', { free: m.free, total: m.total })} <span className="text-muted font-body text-[11px]">{t('market.merchantHint')}</span></h3>
          <p className="text-muted italic">{t('market.tradingOff')}</p>
          <TransferFields villageId={villageId} info={m} list={list} target={target} setTarget={setTarget} amount={amount} setAmount={setAmount} />
        </div>
        <TravellingTrader villageId={villageId} trader={m.trader} canAccept={m.canAccept} />
      </div>
    );
  }

  return (
    <div className="space-y-4 text-[12px]">
      <div className="panel p-3 space-y-2">
        <h3 className="text-[15px]">{t('market.merchants', { free: m.free, total: m.total })} <span className="text-muted font-body text-[11px]">{t('market.merchantHint')}</span></h3>
        <TransferFields villageId={villageId} info={m} list={list} target={target} setTarget={setTarget} amount={amount} setAmount={setAmount} />
      </div>
      <OffersPanel villageId={villageId} />
      <BoardPanel />
      <TravellingTrader villageId={villageId} trader={m.trader} canAccept={m.canAccept} />
      {/* Only merchant shipments: a march has nothing to do with the market, so the same panel is filtered to trades. */}
      <MovementsSection scope="trade" heading={t('market.shipments')} />
    </div>
  );
}
