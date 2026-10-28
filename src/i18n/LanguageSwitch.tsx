import type { MouseEvent } from 'react';
import { LOCALES, rememberLocale, useLocale, useT, type Locale } from './i18n';

/** EN and PL, as buttons or, with `hrefFor`, as links to the page's twin; the active one is marked. */
export function LanguageSwitch({ onChange, hrefFor, className = '', buttonClassName = 'lang-switch-btn' }: {
  onChange?: (l: Locale) => void; hrefFor?: (l: Locale) => string; className?: string; buttonClassName?: string;
}) {
  const t = useT();
  const active = useLocale();
  const choose = (l: Locale) => {
    if (l === active) return;
    rememberLocale(l);
    onChange?.(l);
  };
  return (
    <span className={`lang-switch ${className}`} role="group" aria-label={t('language.label')}>
      {LOCALES.map((l) => {
        const cls = `${buttonClassName} ${l === active ? 'lang-active' : ''}`;
        if (hrefFor) {
          const onClick = (e: MouseEvent<HTMLAnchorElement>) => {
            if (e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
            e.preventDefault();
            choose(l);
          };
          return (
            <a key={l} href={hrefFor(l)} hrefLang={l} lang={l} className={cls}
              aria-current={l === active ? 'page' : undefined} onClick={onClick}>
              {t(`language.${l}`)}
            </a>
          );
        }
        return (
          <button key={l} type="button" className={cls} aria-pressed={l === active} onClick={() => choose(l)}>
            {t(`language.${l}`)}
          </button>
        );
      })}
    </span>
  );
}
