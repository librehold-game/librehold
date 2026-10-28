// @vitest-environment happy-dom
import type { ReactElement } from 'react';
import { describe, expect, it } from 'vitest';
import { act, render, screen } from '@testing-library/react';
import { renderToString } from 'react-dom/server';
import { lazyRoute, preloadRoute, provideRoute, RoutePage, type PageProps } from '../src/account/pages';

const props: PageProps = { params: {}, me: null, onLogin: () => undefined, onClaimed: () => undefined };

describe('route pages', () => {
  it('a lazy route shows its fallback, then the page', async () => {
    let resolve!: (m: { default: () => ReactElement }) => void;
    lazyRoute('changelog', () => new Promise((r) => { resolve = r; }));
    const { container } = render(<RoutePage id="changelog" {...props} />);
    expect(container.querySelector('.page-loading[aria-busy="true"]')).not.toBeNull();
    await act(async () => resolve({ default: () => <p>What changed</p> }));
    expect(await screen.findByText('What changed')).toBeTruthy();
    expect(container.querySelector('.page-loading')).toBeNull();
  });

  it('a provided route renders in renderToString', () => {
    provideRoute('faq', () => <p>Questions and answers</p>);
    expect(renderToString(<RoutePage id="faq" {...props} />)).toContain('Questions and answers');
  });

  it('after preloadRoute a lazy route renders synchronously', async () => {
    lazyRoute('terms', async () => ({ default: () => <p>The rules of the site</p> }));
    await preloadRoute('terms');
    expect(renderToString(<RoutePage id="terms" {...props} />)).toContain('The rules of the site');
  });

  it('a route without a page shows the not found page', () => {
    provideRoute('notFound', () => <p>Nothing here</p>);
    expect(renderToString(<RoutePage id="howToPlay" {...props} />)).toContain('Nothing here');
  });
});
