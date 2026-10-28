/** A finished game's timeline as the server packs it (Go game.TimelineFile). */
export interface TimelineOwner { playerId: string; color: string; bot?: boolean }

export interface TimelineFile {
  v: 1;
  mapSize: number;
  endAt: number; // world ms of the end
  owners: TimelineOwner[]; // owner n in events is owners[n - 1]; 0 = barbarian
  barbarians: [number, number][]; // generated villages at world time 0
  // [t,0,x,y,owner] village · [t,1,x,y,owner] owner change · [t,2,fx,fy,x,y,attacker,defender,size]
  events: number[][];
}
