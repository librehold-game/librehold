import { useEffect, useState } from 'react';
import '../../i18n/hostCatalog';
import { useT } from '../../i18n/i18n';

const QUIET = 4;

/** The text as a QR code drawn in the panel's ink; the encoder loads only when a code is shown. */
export function QrCode({ text }: { text: string }) {
  const t = useT();
  const [cells, setCells] = useState<{ size: number; dark: [number, number][] } | null>(null);
  useEffect(() => {
    let live = true;
    void import('qrcode-generator').then(({ default: qrcode }) => {
      const qr = qrcode(0, 'M');
      qr.addData(text);
      qr.make();
      const size = qr.getModuleCount();
      const dark: [number, number][] = [];
      for (let r = 0; r < size; r++) for (let c = 0; c < size; c++) if (qr.isDark(r, c)) dark.push([c, r]);
      if (live) setCells({ size, dark });
    });
    return () => { live = false; };
  }, [text]);
  if (!cells) return null;
  const side = cells.size + 2 * QUIET;
  return (
    <svg className="qr" viewBox={`0 0 ${side} ${side}`} role="img" aria-label={t('host.qrLabel')} shapeRendering="crispEdges">
      {cells.dark.map(([x, y]) => <rect key={`${x},${y}`} x={x + QUIET} y={y + QUIET} width={1} height={1} fill="currentColor" />)}
    </svg>
  );
}
