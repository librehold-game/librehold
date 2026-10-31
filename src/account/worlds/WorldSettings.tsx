import { formatDecimal, formatDurationHuman, formatNumber } from '../../i18n/format';
import { useT } from '../../i18n/i18n';
import type { PublicSettings } from '../api';
import './catalog';

type Row = [key: string, label: string, value: string];

/** Every setting of a world, in groups; the instant-only ones appear only when the world has them. */
export function WorldSettings({ settings: s, instant }: { settings: PublicSettings; instant: boolean }) {
  const t = useT();
  const times = (n: number) => `×${formatDecimal(n)}`;
  const sight = s.noSight ? t('site.worlds.off') : t('site.worlds.tiles', { count: s.sightRadius });
  const pace: Row[] = [
    ['speed', t('site.worlds.setting.speed'), times(s.worldSpeed)],
    ['unitSpeed', t('site.worlds.setting.unitSpeed'), times(s.unitSpeed)],
  ];
  if (instant && s.durationMin) pace.push(['duration', t('site.worlds.setting.duration'), formatDurationHuman(s.durationMin * 60_000)]);
  if (instant && s.humanSeats) pace.push(['seats', t('site.worlds.setting.seats'), String(s.humanSeats)]);
  if (instant && s.dominationShare) pace.push(['dominationShare', t('site.worlds.setting.dominationShare'), `${Math.round(s.dominationShare * 100)}%`]);

  const map: Row[] = [
    ['mapSize', t('site.worlds.setting.mapSize'), `${s.mapSize}×${s.mapSize}`],
    ['barbarianDensity', t('site.worlds.setting.barbarianDensity'), t(`admin.choice.barbarianDensity.${s.barbarianDensity}`)],
    ['botCount', t('site.worlds.setting.botCount'), String(s.botCount)],
    ['botArrival', t('site.worlds.setting.botArrival'), t(`admin.choice.botArrival.${s.botArrival}`)],
  ];
  if (s.rampUpDays) map.push(['rampUpDays', t('site.worlds.setting.rampUpDays'), t('site.worlds.days', { count: s.rampUpDays })]);
  map.push(
    ['difficulty', t('site.worlds.setting.difficulty'), t(`admin.choice.difficulty.${s.difficulty}`)],
    ['botStance', t('site.worlds.setting.botStance'), t(`admin.choice.botStance.${s.botStance}`)],
  );

  const rules: Row[] = [
    ['protection', t('site.worlds.setting.protection'), t('site.worlds.days', { count: s.beginnerProtectionDays })],
    ['lastVillage', t('site.worlds.setting.lastVillage'), t(`site.worlds.lastVillage.${s.lastVillage}`)],
    ['battleLosses', t('site.worlds.setting.battleLosses'), t(`admin.choice.battleLosses.${s.battleLosses}`)],
    ['fieldBattles', t('site.worlds.setting.fieldBattles'), t(s.fieldBattles ? 'site.worlds.yes' : 'site.worlds.no')],
    ['sightRadius', t('site.worlds.setting.sightRadius'), sight],
  ];
  if (instant) {
    rules.push(['coins', t('site.worlds.setting.coins'), t(s.noCoins ? 'site.worlds.off' : 'site.worlds.yes')]);
    if (s.nobleCost) {
      rules.push(['nobleCost', t('site.worlds.setting.nobleCost'), [s.nobleCost.wood, s.nobleCost.clay, s.nobleCost.iron].map(formatNumber).join(' · ')]);
    }
    if (s.ownedNobleDropMax) rules.push(['nobleLoyalty', t('site.worlds.setting.nobleLoyalty'), `${s.ownedNobleDropMin ?? 0}–${s.ownedNobleDropMax}`]);
    if (s.academyExtraNobles) rules.push(['academyNobles', t('site.worlds.setting.academyNobles'), `+${s.academyExtraNobles}`]);
  }

  const modifiers: Row[] = (['resources', 'recruitment', 'building'] as const).map((k) => [
    k, t(`admin.modifier.${k}`), t('site.worlds.modifierValue', { humans: times(s.humanModifiers[k]), bots: times(s.botModifiers[k]) }),
  ]);

  const groups: [string, Row[]][] = [['pace', pace], ['map', map], ['rules', rules], ['modifiers', modifiers]];
  return (
    <>
      {groups.map(([group, rows]) => (
        <div key={group} className="settings-group">
          <h3>{t(`site.worlds.group.${group}`)}</h3>
          <dl className="settings-list">
            {rows.map(([key, label, value]) => <div key={key}><dt>{label}</dt><dd>{value}</dd></div>)}
          </dl>
        </div>
      ))}
    </>
  );
}
