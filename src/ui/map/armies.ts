/**
 * Armies the map draws, built from the commands the player state carries. Foreign armies never carry their units
 * here, whatever the state holds: the map must not become a way to see them.
 */
import { humanVillageIds } from '../../engine/players';
import { UNITS } from '../../data/units.data';
import { UNIT_IDS, coordsOfVillageId, type Command, type GameState, type UnitId } from '../../engine/types';
import { legEnds, positionAt } from '../../engine/fieldGeometry';
import { BIOME, SUB_X, SUB_Y, biomeGrid } from './biomes';

export type ArmyKind = 'ownAttack' | 'ownSupport' | 'ownReturn' | 'ownIntercept' | 'incomingAttack' | 'incomingSupport'
  | 'threat' | 'passingAttack' | 'passingSupport' | 'passingReturn' | 'trade';

export interface MapArmy {
  id: string;
  kind: ArmyKind;
  originId: string;
  targetId: string;
  /** Village centres in tile coordinates. */
  from: { x: number; y: number };
  to: { x: number; y: number };
  departedAt: number;
  arrivesAt: number;
  units: Partial<Record<UnitId, number>> | null;
}

export const CROWD = { maxPerArmy: 240, maxOnScreen: 600, genericFigures: 8 } as const;

export const FOREIGN_KINDS: ReadonlySet<ArmyKind> = new Set(['incomingAttack', 'incomingSupport', 'threat', 'passingAttack', 'passingSupport', 'passingReturn']);

/** Foreign armies that read as a threat carry torches; the others shields. */
export const TORCH_KINDS: ReadonlySet<ArmyKind> = new Set(['incomingAttack', 'threat', 'passingAttack']);

/** Other players' armies the player may meet on the road. */
export const INTERCEPTABLE_KINDS: ReadonlySet<ArmyKind> = new Set(['incomingAttack', 'incomingSupport', 'passingAttack', 'passingSupport', 'passingReturn']);

/** Whether a foreign army is within sight of one of the player's villages now (the server sends only those). */
function inSight(g: GameState, c: Command, mine: ReadonlySet<string>): boolean {
  const at = positionAt(c, g.now);
  if (!at) return false;
  const r = (g.config.sightRadius ?? 0) > 0 ? g.config.sightRadius! : 1;
  for (const id of mine) {
    const v = coordsOfVillageId(id);
    if (v && Math.hypot(at.x - v.x, at.y - v.y) <= r + 1e-9) return true;
  }
  return false;
}

function kindOf(g: GameState, c: Command, mine: ReadonlySet<string>): ArmyKind | null {
  if (c.type === 'trade') return mine.has(c.originId) ? 'trade' : null;
  if (c.type === 'intercept') return mine.has(c.originId) ? 'ownIntercept' : 'threat';
  if (c.type === 'return' && mine.has(c.targetId)) return 'ownReturn';
  if (c.type !== 'return' && mine.has(c.originId)) return c.type === 'attack' ? 'ownAttack' : 'ownSupport';
  if (mine.has(c.targetId)) return c.type === 'attack' ? 'incomingAttack' : c.type === 'support' ? 'incomingSupport' : null;
  if (!inSight(g, c, mine)) return null;
  return c.type === 'attack' ? 'passingAttack' : c.type === 'support' ? 'passingSupport' : 'passingReturn';
}

/**
 * A command as the map draws it, placed from the command itself (the client may not hold the villages at its ends);
 * null when an end is unknown or off a map of mapSize. Foreign and trade armies carry no units.
 */
export function mapArmy(c: Pick<Command, 'id' | 'type' | 'originId' | 'targetId' | 'point' | 'departedAt' | 'arrivesAt'> & { units?: Command['units'] },
  kind: ArmyKind, mapSize: number): MapArmy | null {
  const ends = legEnds(c);
  const inside = (p: { x: number; y: number }) => p.x >= 0 && p.y >= 0 && p.x < mapSize && p.y < mapSize;
  if (!ends || !inside(ends.from) || !inside(ends.to)) return null;
  const o = ends.from, t = ends.to;
  return {
    id: c.id, kind, originId: c.originId, targetId: c.targetId,
    from: { x: o.x + 0.5, y: o.y + 0.5 }, to: { x: t.x + 0.5, y: t.y + 0.5 },
    departedAt: c.departedAt, arrivesAt: c.arrivesAt,
    units: FOREIGN_KINDS.has(kind) || kind === 'trade' || !c.units ? null : { ...c.units },
  };
}

export function armiesOnMap(g: GameState): MapArmy[] {
  const mine = new Set(humanVillageIds(g));
  const out: MapArmy[] = [];
  for (const c of g.commands) {
    const kind = kindOf(g, c, mine);
    const a = kind && mapArmy(c, kind, g.config.mapSize);
    if (a) out.push(a);
  }
  return out;
}

export function armyPosition(a: MapArmy, now: number): { x: number; y: number; p: number } {
  const p = Math.max(0, Math.min(1, (now - a.departedAt) / Math.max(1, a.arrivesAt - a.departedAt)));
  return { x: a.from.x + (a.to.x - a.from.x) * p, y: a.from.y + (a.to.y - a.from.y) * p, p };
}

/** One figure per unit for a small army; past 24 every doubling adds 16 more, so a big army still looks bigger. */
export function figuresFor(count: number): number {
  if (count <= 0) return 0;
  if (count <= 24) return Math.round(count);
  return Math.round(24 + 16 * Math.log2(count / 24));
}

export function crowdOf(units: Partial<Record<UnitId, number>>, cap: number = CROWD.maxPerArmy): { unit: UnitId; figures: number }[] {
  const crowd = UNIT_IDS.filter((u) => (units[u] ?? 0) > 0).map((unit) => ({ unit, figures: figuresFor(units[unit]!) }));
  const sum = crowd.reduce((s, c) => s + c.figures, 0);
  if (sum <= cap) return crowd;
  return crowd.map((c) => ({ unit: c.unit, figures: Math.max(1, Math.floor((c.figures * cap) / sum)) }));
}

export type Ground = 'meadow' | 'forest' | 'mountain' | 'water';

const GROUND = { [BIOME.grass]: 'meadow', [BIOME.water]: 'water', [BIOME.forest]: 'forest', [BIOME.mountain]: 'mountain' } as const;

const noVillages = () => false;
const grounds = new WeakMap<(x: number, y: number) => boolean, Map<string, Ground>>();

/**
 * The ground at a map position: the biome of the nearest terrain vertex as the map draws it, with village clearings,
 * smoothed edges and specks, so an army never takes to a boat on drawn land. Pass the map's `isVillage`.
 */
export function groundAt(seed: number, x: number, y: number, isVillage: (x: number, y: number) => boolean = noVillages): Ground {
  const vx = Math.round(x * SUB_X), vy = Math.round(y * SUB_Y);
  let cache = grounds.get(isVillage);
  if (!cache) grounds.set(isVillage, (cache = new Map()));
  const key = `${seed}|${vx}|${vy}`;
  let ground = cache.get(key);
  if (!ground) {
    const g = biomeGrid(seed, vx, vy, 1, 1, isVillage);
    ground = g.cleared[0] ? 'meadow' : GROUND[g.biome[0] as keyof typeof GROUND];
    cache.set(key, ground);
  }
  return ground;
}

/** The unit an own army is shown by: a hetman if it has one, else the type carrying most attack. */
export function leadUnit(units: Partial<Record<UnitId, number>>): UnitId {
  if ((units.noble ?? 0) > 0) return 'noble';
  let best: UnitId = 'spear', power = -1;
  for (const u of UNIT_IDS) {
    const p = (units[u] ?? 0) * Math.max(1, UNITS[u].attack);
    if (p > power && (units[u] ?? 0) > 0) { best = u; power = p; }
  }
  return best;
}
