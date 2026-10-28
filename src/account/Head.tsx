import { createContext, useContext, useEffect } from 'react';
import { applyHead } from './headTags';
import { usePathname } from './router';

export interface HeadData {
  title: string;
  description: string;
  image?: string;
  imageAlt?: string;
  noIndex?: boolean;
  jsonLd?: Record<string, unknown>[];
  notFound?: boolean;
}

/** Collects the page's head during prerender; absent in the browser. */
export const HeadSink = createContext<HeadData | null>(null);

export function Head(props: HeadData): null {
  const sink = useContext(HeadSink);
  const pathname = usePathname();
  if (sink) Object.assign(sink, props);
  const { title, description, noIndex, notFound } = props;
  useEffect(() => {
    if (!sink) applyHead({ title, description, noIndex, notFound }, pathname);
  }, [sink, title, description, noIndex, notFound, pathname]);
  return null;
}
