import { useEffect, useState } from 'react';
import { preload } from 'react-dom';
import posterUrl from '../assets/pixel/account/clip-poster.webp';
import smallPosterUrl from '../assets/pixel/account/clip-poster-480.webp';
import mp4Url from '../assets/pixel/account/clip.mp4';
import webmUrl from '../assets/pixel/account/clip.webm';
import { useT } from '../i18n/i18n';

/**
 * Ten seconds of the real game. The poster loads first and stays (it is the page's largest paint); once the page has
 * loaded, the loop plays over it, except for readers who asked for less motion.
 */
export function Clip() {
  const t = useT();
  const [moving, setMoving] = useState(false);
  useEffect(() => {
    if (window.matchMedia?.('(prefers-reduced-motion: reduce)').matches) return;
    const start = () => setMoving(true);
    if (document.readyState === 'complete') {
      start();
      return;
    }
    window.addEventListener('load', start, { once: true });
    return () => window.removeEventListener('load', start);
  }, []);
  const alt = t('site.home.hero.clipAlt');
  // The poster is the page's largest paint: fetch the one for this screen first, from the head.
  preload(smallPosterUrl, { as: 'image', media: '(max-width: 640px)', fetchPriority: 'high' });
  preload(posterUrl, { as: 'image', media: '(min-width: 641px)', fetchPriority: 'high' });
  return (
    <div className="clip">
      <picture>
        <source media="(max-width: 640px)" srcSet={smallPosterUrl} />
        <img src={posterUrl} width={960} height={540} alt={moving ? '' : alt} fetchPriority="high" />
      </picture>
      {moving && (
        <video autoPlay muted loop playsInline poster={posterUrl} width={960} height={540} aria-label={alt}>
          <source src={webmUrl} type="video/webm" />
          <source src={mp4Url} type="video/mp4" />
        </video>
      )}
    </div>
  );
}
