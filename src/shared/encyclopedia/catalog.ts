// The encyclopedia's own words and the game's names for units and buildings (only those parts of the game catalogue).
import { registerCatalog } from '../../i18n/i18n';
import enEncyclopedia from '../../i18n/catalog/en/encyclopedia.json';
import plEncyclopedia from '../../i18n/catalog/pl/encyclopedia.json';
import { building as enBuilding, unit as enUnit } from '../../i18n/catalog/en/game.json';
import { building as plBuilding, unit as plUnit } from '../../i18n/catalog/pl/game.json';

registerCatalog('en', enEncyclopedia);
registerCatalog('pl', plEncyclopedia);
registerCatalog('en', { unit: enUnit, building: enBuilding });
registerCatalog('pl', { unit: plUnit, building: plBuilding });
