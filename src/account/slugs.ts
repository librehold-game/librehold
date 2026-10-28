export const GUIDE_IDS = ['firstHour', 'raiding', 'defending', 'scouting', 'nobles'] as const;

export type SlugKind = 'guide' | 'unit' | 'building';

export const SLUGS: Record<SlugKind, Record<string, { en: string; pl: string }>> = {
  guide: {
    firstHour: { en: 'first-hour', pl: 'pierwsza-godzina' },
    raiding: { en: 'raiding-barbarians', pl: 'grabienie-barbarzyncow' },
    defending: { en: 'defending-at-night', pl: 'obrona-noca' },
    scouting: { en: 'scouting-and-field-battles', pl: 'zwiad-i-bitwy-w-polu' },
    nobles: { en: 'taking-a-village', pl: 'przejmowanie-wiosek' },
  },
  unit: {
    spear: { en: 'spearman', pl: 'pikinier' },
    sword: { en: 'swordsman', pl: 'miecznik' },
    axe: { en: 'axeman', pl: 'topornik' },
    archer: { en: 'archer', pl: 'lucznik' },
    scout: { en: 'scout', pl: 'zwiadowca' },
    lightCav: { en: 'light-cavalryman', pl: 'lekki-kawalerzysta' },
    mountedArcher: { en: 'mounted-archer', pl: 'lucznik-konny' },
    heavyCav: { en: 'heavy-cavalryman', pl: 'ciezki-kawalerzysta' },
    ram: { en: 'ram', pl: 'taran' },
    catapult: { en: 'catapult', pl: 'katapulta' },
    paladin: { en: 'knight', pl: 'rycerz' },
    noble: { en: 'hetman', pl: 'hetman' },
  },
  building: {
    main: { en: 'town-hall', pl: 'ratusz' },
    barracks: { en: 'barracks', pl: 'koszary' },
    stable: { en: 'stable', pl: 'stajnia' },
    workshop: { en: 'workshop', pl: 'warsztat' },
    academy: { en: 'academy', pl: 'akademia' },
    smithy: { en: 'smithy', pl: 'kuznia' },
    statue: { en: 'pedestal', pl: 'piedestal' },
    market: { en: 'market', pl: 'rynek' },
    wood: { en: 'lumber-mill', pl: 'tartak' },
    clay: { en: 'brickworks', pl: 'cegielnia' },
    iron: { en: 'ironworks', pl: 'huta-zelaza' },
    farm: { en: 'farmstead', pl: 'zagroda' },
    storage: { en: 'storehouse', pl: 'spichlerz' },
    hide: { en: 'hideout', pl: 'schowek' },
    wall: { en: 'wall', pl: 'mur-obronny' },
  },
};
