import { afterEach, describe, expect, it } from 'vitest';
import { setLocale } from '../src/i18n/i18n';
import { blockerText, errorMessage, newsText, reportVillageLabel, unitName, villageName, worldDisplayName } from '../src/i18n/names';

const BLOCKER_KINDS = ['maxLevel', 'queueFull', 'requirements', 'research', 'resources', 'population', 'busy', 'limit', 'coins',
  'count', 'noUnits', 'notEnough', 'noTarget', 'self', 'noAttackPower', 'supportTargetBarbarian', 'attackOwnVillage',
  'nobleUnescorted', 'nobleTooFar', 'merchants', 'storage', 'sameVillage', 'academy', 'targetProtected', 'selfProtected',
  'notOwner', 'noMarket', 'empty'];

afterEach(() => setLocale('pl'));

describe('names in both languages', () => {
  it('worldDisplayName names an instant game by its number in the reader\'s language', () => {
    setLocale('pl');
    expect(worldDisplayName({ name: 'Game #41', gameNumber: 41 })).toBe('Gra #41');
    setLocale('en');
    expect(worldDisplayName({ name: 'Game #41', gameNumber: 41 })).toBe('Game #41');
    expect(worldDisplayName({ name: 'Oakwood' })).toBe('Oakwood');
    expect(worldDisplayName({ name: 'Oakwood', gameNumber: null })).toBe('Oakwood');
  });

  it('renders default village names per language', () => {
    setLocale('en');
    expect(villageName({ name: '' })).toBe('Barbarian village');
    expect(villageName({ name: '' }, 'Kalvik')).toBe("Kalvik's village");
    expect(villageName({ name: 'Gród' }, 'Kalvik')).toBe('Gród');
    setLocale('pl');
    expect(villageName({ name: '' })).toBe('Wioska barbarzyńska');
    expect(villageName({ name: '' }, 'Kalvik')).toBe('Wioska Kalvik');
  });

  it('renders unit plurals', () => {
    setLocale('en');
    expect(`5 ${unitName('spear', 5)}`).toBe('5 Spearmen');
    expect(`1 ${unitName('spear', 1)}`).toBe('1 Spearman');
    setLocale('pl');
    expect(`5 ${unitName('spear', 5)}`).toBe('5 Pikinierów');
    expect(`3 ${unitName('ram', 3)}`).toBe('3 Tarany');
    expect(`5 ${unitName('ram', 5)}`).toBe('5 Taranów');
  });

  it('renders every server blocker kind', () => {
    for (const locale of ['en', 'pl'] as const) {
      setLocale(locale);
      for (const kind of BLOCKER_KINDS) expect(blockerText({ kind }), `${locale} ${kind}`).not.toBe(kind);
    }
  });

  it('words server errors with their values', () => {
    setLocale('en');
    expect(errorMessage('tooLate', { ms: 65_000 })).toBe('Too late — the troops would have had to leave 1 min ago');
    expect(errorMessage('cannotDemolish', { building: 'wall' })).toContain('Wall');
    expect(errorMessage('blocked', undefined, [{ kind: 'resources' }, { kind: 'queueFull' }])).toBe("That can't be done: not enough resources, the queue is full");
    expect(errorMessage('somethingNew')).toBe('Something went wrong');
  });

  it('words news and rumours, with barbarians and default village names', () => {
    setLocale('en');
    expect(newsText({ key: 'gossip.conquest.0', params: { a: 'Torvik', b: '', village: '', x: 3, y: 4 } })).toBe('Torvik took 3|4 from the barbarians.');
    expect(newsText({ key: 'gossip.conquest.0', params: { a: 'Torvik', b: 'Velmund', village: 'Gród', x: 3, y: 4 } })).toBe('Torvik took Gród (3|4) from Velmund.');
    expect(newsText({ key: 'news.scheduledFailed', params: { blockers: [{ kind: 'notEnough' }] } })).toBe("A planned send didn't go out: not enough troops in the village.");
    setLocale('pl');
    expect(newsText({ key: 'news.respawn', params: { x: 1, y: 2 } })).toBe('Twoje ostatnie wioski upadły. Zaczynasz od nowa w 1|2.');
  });

  it('labels report villages from their snapshot', () => {
    setLocale('en');
    expect(reportVillageLabel({ id: 'v:3:4', name: '', x: 3, y: 4 })).toBe('Barbarian village (3|4)');
    expect(reportVillageLabel({ id: 'v1', name: '', x: 3, y: 4, ownerId: 'b1', ownerName: 'Torvik' })).toBe("Torvik's village (3|4)");
  });
});
