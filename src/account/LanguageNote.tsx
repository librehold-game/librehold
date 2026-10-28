import { useEffect, useState } from 'react';
import { translate, useLocale, type Locale } from '../i18n/i18n';
import { Link, usePathname } from './router';
import { twinPath } from './routes';

const CLOSED_KEY = 'languageNoteClosed';

function browserLocale(): Locale {
  return navigator.language?.toLowerCase().startsWith('pl') ? 'pl' : 'en';
}

function closed(): boolean {
  try {
    return localStorage.getItem(CLOSED_KEY) === '1';
  } catch {
    return false;
  }
}

/** Points a visitor without a language choice to the page in their browser's language. Browser-only, after mount. */
export function LanguageNote() {
  const page = useLocale();
  const pathname = usePathname();
  const [reader, setReader] = useState<Locale | null>(null);
  useEffect(() => {
    const chosen = /(?:^|;\s*)lang=(en|pl)/.test(document.cookie);
    const l = browserLocale();
    setReader(!chosen && !closed() && l !== page ? l : null);
  }, [page]);
  if (!reader) return null;
  const close = () => {
    try {
      localStorage.setItem(CLOSED_KEY, '1');
    } catch {
      /* private mode: closed for this page view only */
    }
    setReader(null);
  };
  return (
    <aside className="language-note" lang={reader}>
      <Link to={twinPath(pathname)} hrefLang={reader}>{translate(reader, 'languageNote.text')}</Link>
      <button type="button" className="link" onClick={close}>{translate(reader, 'languageNote.close')}</button>
    </aside>
  );
}
