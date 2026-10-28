import { BUILDINGS } from '../../data/buildings.data';
import { CONQUEST_BALANCE } from '../../data/conquest.data';
import { COMBAT_BALANCE, UNITS } from '../../data/units.data';
import { formatNumber } from '../../i18n/format';
import facts from './facts.json';

const academyNeeds = (b: string) => BUILDINGS.academy.requirements.find((r) => r.building === b)!.level;

/** The numbers the guides quote: the server's (facts.json, pinned by a Go test) and the game data's. */
export function guideParams(): Record<string, string | number> {
  const C = CONQUEST_BALANCE;
  const raw: Record<string, number> = {
    ...facts,
    scoutMinutesPerField: UNITS.scout.speed,
    lightCavMinutesPerField: UNITS.lightCav.speed,
    lightCavCarry: UNITS.lightCav.haul,
    axeCarry: UNITS.axe.haul,
    spearCarry: UNITS.spear.haul,
    spearDefCav: UNITS.spear.defenseCavalry,
    scoutDefenseFactor: COMBAT_BALANCE.scoutDefenseFactor,
    nobleDropMin: C.nobleDropMin,
    nobleDropMax: C.nobleDropMax,
    maxLoyalty: C.maxLoyalty,
    loyaltyAfterConquest: C.loyaltyAfterConquest,
    nobleMaxDistance: C.nobleMaxDistance,
    plunderLevelsMin: C.plunderLevelsMin,
    plunderLevelsMax: C.plunderLevelsMax,
    coinWood: C.coinBaseCost.wood,
    coinClay: C.coinBaseCost.clay,
    coinIron: C.coinBaseCost.iron,
    academyMain: academyNeeds('main'),
    academyMarket: academyNeeds('market'),
    academySmithy: academyNeeds('smithy'),
  };
  return Object.fromEntries(Object.entries(raw).map(([k, v]) => [k, formatNumber(v)]));
}
