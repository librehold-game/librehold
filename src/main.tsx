import React from 'react';
import { createRoot } from 'react-dom/client';
import './index.css';
import { App } from './ui/App';
import { ErrorBoundary } from './ui/components/ErrorBoundary';
import { GodView } from './ui/watch/GodView';
import { loadGameCatalogs } from './i18n/catalogs';
import { detectLocale, setLocale } from './i18n/i18n';

loadGameCatalogs();
setLocale(detectLocale());

// A player world's host watches it with ?watch=1: the god view, with no player and no player store.
const watching = new URLSearchParams(window.location.search).get('watch') === '1';

createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <ErrorBoundary>
      {watching ? <GodView /> : <App />}
    </ErrorBoundary>
  </React.StrictMode>,
);

// With ?dev=1 the store is exposed on the console (debugging / e2e tests).
import { useGameStore } from './state/gameStore';
if (useGameStore.getState().devMode) {
  (window as unknown as { __lh: typeof useGameStore }).__lh = useGameStore;
}
