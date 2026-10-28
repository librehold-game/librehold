import { useEffect, useMemo, useRef, useState } from 'react';
import '../../i18n/timelapseCatalog';
import { formatDuration } from '../../i18n/format';
import { useT } from '../../i18n/i18n';
import { worldDisplayName } from '../../i18n/names';
import { call, type ArchivedGame } from '../api';
import { buildTimelapse, frameAt, type Timelapse } from './model';
import { LENGTHS, advance, effectMsFor, initialPlayback, type Length, type PlaybackState } from './playback';
import { canRecord, recordTimelapse } from './record';
import { drawFrame, terrainCanvas } from './render';
import type { TimelineFile } from './types';

const CANVAS = 540;
const LAYOUT = { x: 0, y: 0, size: CANVAS };

const reducedMotion = () => typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches;

/** A finished game replayed on its map: who held which village, the conquests and the battles, in half a minute to two. */
export function TimelapsePlayer({ gameRef, game }: { gameRef: string; game: ArchivedGame }) {
  const t = useT();
  const [file, setFile] = useState<TimelineFile | null>(null);
  const [failed, setFailed] = useState(false);
  useEffect(() => {
    setFile(null);
    setFailed(false);
    call<TimelineFile>('GET', `/api/v1/public/games/${gameRef}/timeline`).then(setFile, () => setFailed(true));
  }, [gameRef]);
  const tl = useMemo(() => (file ? buildTimelapse(file, game.rows) : null), [file, game.rows]);

  if (failed) return <p className="muted">{t('timelapse.unavailable')}</p>;
  if (!tl) return <p className="muted">{t('timelapse.loading')}</p>;
  return <Player tl={tl} game={game} />;
}

function Player({ tl, game }: { tl: Timelapse; game: ArchivedGame }) {
  const t = useT();
  const [p, setP] = useState<PlaybackState>(() => initialPlayback(reducedMotion()));
  const canvas = useRef<HTMLCanvasElement>(null);
  const frame = useMemo(() => frameAt(tl, p.t, effectMsFor(p.length, tl.endAt)), [tl, p.t, p.length]);

  useEffect(() => {
    if (!p.playing) return;
    let live = true;
    let last: number | null = null;
    const tick = (now: number) => {
      if (!live) return;
      if (last !== null) {
        const dt = now - last;
        setP((s) => advance(s, dt, tl.endAt));
      }
      last = now;
      requestAnimationFrame(tick);
    };
    requestAnimationFrame(tick);
    return () => { live = false; };
  }, [p.playing, tl.endAt]);

  useEffect(() => {
    const ctx = canvas.current?.getContext('2d');
    if (ctx) drawFrame(ctx, tl, frame, LAYOUT, terrainCanvas(tl.mapSize));
  }, [tl, frame]);

  return (
    <figure className="timelapse">
      <canvas ref={canvas} width={CANVAS} height={CANVAS} role="img" aria-label={t('timelapse.map')} />
      <figcaption>
        <p role="timer" className="timelapse-clock">
          {t('timelapse.clock', { time: formatDuration(p.t), end: formatDuration(tl.endAt) })}
        </p>
        <input type="range" min={0} max={tl.endAt} step={1000} value={p.t} aria-label={t('timelapse.scrub')}
          onChange={(e) => setP((s) => ({ ...s, t: Number(e.target.value), playing: false }))} />
        <div className="timelapse-controls">
          {p.playing
            ? <button className="btn" onClick={() => setP((s) => ({ ...s, playing: false }))}>{t('timelapse.pause')}</button>
            : <button className="btn" onClick={() => setP((s) => ({ ...s, playing: true, t: s.t >= tl.endAt ? 0 : s.t }))}>{t('timelapse.play')}</button>}
          <button className="btn" onClick={() => setP((s) => ({ ...s, t: 0, playing: true }))}>{t('timelapse.restart')}</button>
          <label>
            {t('timelapse.length')}{' '}
            <select value={p.length} onChange={(e) => setP((s) => ({ ...s, length: Number(e.target.value) as Length }))}>
              {LENGTHS.map((n) => <option key={n} value={n}>{t('timelapse.seconds', { n })}</option>)}
            </select>
          </label>
        </div>
        <Legend tl={tl} legend={frame.legend} />
        <VideoDownload tl={tl} game={game} />
      </figcaption>
    </figure>
  );
}

function Legend({ tl, legend }: { tl: Timelapse; legend: { owner: number; villages: number }[] }) {
  const t = useT();
  return (
    <ul className="timelapse-legend" aria-label={t('timelapse.legend')}>
      {legend.map(({ owner, villages }) => {
        const o = tl.owners[owner - 1]!;
        return (
          <li key={owner}>
            <span className="swatch" style={{ background: o.color }} aria-hidden="true" />
            {o.name ? o.name : <span className="muted">{t('archive.formerPlayer')}</span>}
            {o.bot && <span className="bot-tag">{t('archive.bot')}</span>}
            <span className="muted"> · {villages}</span>
          </li>
        );
      })}
    </ul>
  );
}

const fileName = (name: string) => name.toLowerCase().replace(/[^\p{L}\p{N}]+/gu, '-').replace(/^-|-$/g, '');

/** The replay as a square clip to share: recorded in the browser, offered as a file when done. */
function VideoDownload({ tl, game }: { tl: Timelapse; game: ArchivedGame }) {
  const t = useT();
  const [length, setLength] = useState<Length>(30);
  const [pct, setPct] = useState<number | null>(null);
  const [clip, setClip] = useState<{ url: string; ext: string } | null>(null);
  const recording = useRef<AbortController | null>(null);
  useEffect(() => () => recording.current?.abort(), []);
  useEffect(() => () => { if (clip) URL.revokeObjectURL(clip.url); }, [clip]);
  if (!canRecord()) return null;

  const title = worldDisplayName(game);
  const named = (name: string, bot: boolean) => `${name || t('archive.formerPlayer')}${bot ? ` (${t('archive.bot')})` : ''}`;
  const start = () => {
    const ac = new AbortController();
    recording.current = ac;
    setClip(null);
    setPct(0);
    const info = { title, winner: t('timelapse.wonBy', { winner: named(game.winnerName, game.winnerIsBot) }) };
    const names = (owner: number) => { const o = tl.owners[owner - 1]; return o ? named(o.name, o.bot) : ''; };
    recordTimelapse(tl, info, names, { length, onProgress: (share) => setPct(Math.round(share * 100)), signal: ac.signal })
      .then(({ blob, ext }) => { setClip({ url: URL.createObjectURL(blob), ext }); setPct(null); }, () => setPct(null));
  };
  return (
    <div className="timelapse-controls">
      <label>
        {t('timelapse.videoLength')}{' '}
        <select value={length} disabled={pct !== null} onChange={(e) => setLength(Number(e.target.value) as Length)}>
          {LENGTHS.map((n) => <option key={n} value={n}>{t('timelapse.seconds', { n })}</option>)}
        </select>
      </label>
      <button className="btn" onClick={start} disabled={pct !== null}>{t('timelapse.download')}</button>
      {pct !== null && <span role="status" className="muted">{t('timelapse.recording', { pct })}</span>}
      {clip && <a className="btn" href={clip.url} download={`${fileName(title)}-timelapse.${clip.ext}`}>{t('timelapse.save')}</a>}
    </div>
  );
}
