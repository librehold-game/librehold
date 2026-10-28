/**
 * Sounds synthesised with WebAudio (no audio files).
 * Every effect is short and respects the `prefs.sound` setting.
 */
let ctx: AudioContext | null = null;

function audio(): AudioContext | null {
  if (typeof AudioContext === 'undefined') return null;
  ctx ??= new AudioContext();
  if (ctx.state === 'suspended') void ctx.resume();
  return ctx;
}

function tone(freq: number, startMs: number, durMs: number, type: OscillatorType = 'triangle', gain = 0.08): void {
  const c = audio();
  if (!c) return;
  const osc = c.createOscillator();
  const g = c.createGain();
  const t0 = c.currentTime + startMs / 1000;
  osc.type = type;
  osc.frequency.setValueAtTime(freq, t0);
  g.gain.setValueAtTime(0, t0);
  g.gain.linearRampToValueAtTime(gain, t0 + 0.01);
  g.gain.exponentialRampToValueAtTime(0.0005, t0 + durMs / 1000);
  osc.connect(g).connect(c.destination);
  osc.start(t0);
  osc.stop(t0 + durMs / 1000 + 0.05);
}

/** Construction finished: two rising chimes. */
export function playBuildDone(): void {
  tone(660, 0, 160);
  tone(880, 140, 260);
}

/** Przybycie ataku: dzwon alarmowy (trzy uderzenia). */
export function playAlarm(): void {
  for (let i = 0; i < 3; i++) {
    tone(440, i * 260, 220, 'square', 0.05);
    tone(554, i * 260 + 20, 200, 'sine', 0.04);
  }
}

/** Hetman ceremony: a short fanfare. */
export function playFanfare(): void {
  const seq = [523, 659, 784, 1047];
  seq.forEach((f, i) => tone(f, i * 150, i === seq.length - 1 ? 600 : 180, 'sawtooth', 0.05));
}

/** Quest completed: a single high note. */
export function playQuest(): void {
  tone(988, 0, 120, 'sine', 0.05);
  tone(1319, 110, 240, 'sine', 0.05);
}
