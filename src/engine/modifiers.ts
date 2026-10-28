import { HUMAN_ID, type SideModifiers, type Village, type WorldConfig } from './types';

const NEUTRAL: SideModifiers = { resources: 1, recruitment: 1, building: 1 };

/** Multipliers for the side a village belongs to: human, bots or barbarians (always 1). */
export function modifiersFor(cfg: WorldConfig, v: Village): SideModifiers {
  if (v.ownerId === null) return NEUTRAL;
  return v.ownerId === HUMAN_ID ? cfg.humanModifiers : cfg.botModifiers;
}
