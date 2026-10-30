import firstHourStart from '../../assets/pixel/account/guides/first-hour-start.webp';
import firstHourConquest from '../../assets/pixel/account/guides/first-hour-conquest.webp';
import raidingFarm from '../../assets/pixel/account/guides/raiding-farm.webp';
import raidingReport from '../../assets/pixel/account/guides/raiding-report.webp';
import nightIncoming from '../../assets/pixel/account/guides/night-incoming.webp';
import nightSupport from '../../assets/pixel/account/guides/night-support.webp';
import scoutingArmyCard from '../../assets/pixel/account/guides/scouting-army-card.webp';
import scoutingFieldReport from '../../assets/pixel/account/guides/scouting-field-report.webp';
import noblesAcademy from '../../assets/pixel/account/guides/nobles-academy.webp';
import noblesConquest from '../../assets/pixel/account/guides/nobles-conquest.webp';
import type { GUIDE_IDS } from '../slugs';
import type { BuildingId, UnitId } from '../../shared/encyclopedia/facts';

export type GuideId = (typeof GUIDE_IDS)[number];
export interface GuideShot { id: string; src: string; width: number; height: number; after: string }
export interface GuideDef {
  id: GuideId; sections: string[]; shots: GuideShot[];
  related: ({ kind: 'unit'; id: UnitId } | { kind: 'building'; id: BuildingId })[];
}

const shot = (id: string, src: string, after: string): GuideShot => ({ id, src, width: 1200, height: 675, after });
const unit = (id: UnitId) => ({ kind: 'unit' as const, id });
const building = (id: BuildingId) => ({ kind: 'building' as const, id });

export const GUIDES: GuideDef[] = [
  {
    id: 'firstHour', sections: ['start', 'firstConquest', 'growing', 'ending'],
    shots: [shot('first-hour-start', firstHourStart, 'start'), shot('first-hour-conquest', firstHourConquest, 'firstConquest')],
    related: [unit('axe'), unit('noble'), building('main'), building('academy')],
  },
  {
    id: 'raiding', sections: ['why', 'targets', 'army', 'assistant', 'risks'],
    shots: [shot('raiding-farm', raidingFarm, 'assistant'), shot('raiding-report', raidingReport, 'army')],
    related: [unit('lightCav'), unit('axe'), unit('spear'), building('storage'), building('hide')],
  },
  {
    id: 'defending', sections: ['incoming', 'noBonus', 'defence', 'dodge'],
    shots: [shot('night-incoming', nightIncoming, 'incoming'), shot('night-support', nightSupport, 'defence')],
    related: [unit('spear'), unit('sword'), unit('archer'), building('wall'), building('hide')],
  },
  {
    id: 'scouting', sections: ['scouts', 'sight', 'field', 'turnBack'],
    shots: [shot('scouting-army-card', scoutingArmyCard, 'sight'), shot('scouting-field-report', scoutingFieldReport, 'field')],
    related: [unit('scout'), unit('lightCav'), building('smithy')],
  },
  {
    id: 'nobles', sections: ['academy', 'coins', 'loyalty', 'train'],
    shots: [shot('nobles-academy', noblesAcademy, 'academy'), shot('nobles-conquest', noblesConquest, 'loyalty')],
    related: [unit('noble'), building('academy'), building('main'), building('market'), building('smithy')],
  },
];
