export interface PlayerRow {
  id: string; name: string; color: string; points: number; villages: number; att: number; def: number; sup: number; quit: boolean;
  isBot: boolean;
}
export interface DailyRow { id: string; name: string; att: number; def: number; loot: number; farm: number; isBot: boolean }
export interface ConquestRow {
  t: number; x: number; y: number; toId: string; to: string; fromId: string | null; from: string | null; toIsBot: boolean; fromIsBot: boolean;
}

export interface Profile {
  player: { id: string; name: string; color: string; status: string; points: number; villageIds: string[]; isBot: boolean };
  villages: { id: string; name: string; x: number; y: number; points: number }[];
  history: { hourly: number[]; daily: number[] };
}
