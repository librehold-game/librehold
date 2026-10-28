import heroUrl from '../assets/pixel/account/hero.png';

/** Pixel landscape over the login and registration forms: our own art, no game graphics or name. */
export function Hero() {
  return <img className="hero" src={heroUrl} alt="" width={960} height={320} draggable={false} />;
}
