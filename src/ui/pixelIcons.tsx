const URLS = import.meta.glob<string>('../assets/pixel/ui/icons/*.png', { eager: true, query: '?url', import: 'default' });

/** URL of an icon exported by `npm run assets:pixel-ui` (name without extension, e.g. `unit-spear`, `nav-map`). */
export function pixelIcon(name: string): string {
  const url = URLS[`../assets/pixel/ui/icons/${name}.png`];
  if (!url) throw new Error(`Missing pixel icon: ${name}`);
  return url;
}

/**
 * The icons are 32 px pixel art: at 24 px or more nearest-neighbour keeps them crisp, below that it drops whole rows of
 * detail unevenly, so small icons are smoothed instead.
 */
export function PixelIcon({ name, size = 18, label }: { name: string; size?: number; label?: string }) {
  return (
    <img
      src={pixelIcon(name)}
      width={size}
      height={size}
      alt={label ?? ''}
      title={label}
      aria-hidden={label ? undefined : true}
      className="pixel-icon inline-block"
      style={{ imageRendering: size >= 24 ? 'pixelated' : 'auto' }}
    />
  );
}
