import '../../i18n/hostCatalog';
import { t } from '../../i18n/i18n';
import type { HostForm, HostLimits } from '../api';

export const DURATIONS = [30, 45, 60, 75, 90, 105, 120];
export const LATE_JOIN = Array.from({ length: 13 }, (_, i) => i * 5);
export const DOMINATION = [0, 50, 60, 70, 80, 90];
export const DIFFICULTIES = ['easy', 'normal', 'hard', 'brutal'];
export const STANCES = ['peacefulRetaliate', 'peacefulReputation', 'normal', 'allAgainstHumans'];

/** The form as it opens: the instant game's settings with the admin's limits. */
export function defaultHostForm(limits: HostLimits, hostName: string): HostForm {
  return {
    name: t('host.defaultName', { host: hostName }), durationMin: 60, seats: limits.maxSeats, bots: Math.min(3, limits.maxBots),
    worldSpeed: 100, unitSpeed: 100, startVillage: 'grown', barbarians: 'normal', difficulty: 'easy', botStance: 'normal',
    fieldBattles: true, sightRadius: 5, protectionMin: 10, dominationPct: 60, lateJoinMin: 10, registeredOnly: false,
  };
}
