// @vitest-environment happy-dom
import { afterEach, describe, expect, it } from 'vitest';
import { cleanup, render } from '@testing-library/react';
import { RichText } from '../src/ui/components/RichText';

afterEach(() => cleanup());

describe('RichText', () => {
  it('draws [icon:name] as the pixel icon, in plain and bold parts alike', () => {
    const { container } = render(<RichText text="[icon:shield] Protection: <b>[icon:coin] 3</b> left" />);
    const icons = [...container.querySelectorAll('img.pixel-icon')].map((i) => i.getAttribute('src'));
    expect(icons).toHaveLength(2);
    expect(icons[0]).toMatch(/shield/);
    expect(icons[1]).toMatch(/coin/);
    expect(container.querySelector('strong img.pixel-icon')).toBeTruthy();
    expect(container.textContent).toBe(' Protection:  3 left');
  });

  it('leaves text without markers as it is, never as HTML', () => {
    const { container } = render(<RichText text="a <i>b</i> [icon c" />);
    expect(container.textContent).toBe('a <i>b</i> [icon c');
    expect(container.querySelector('img')).toBeNull();
  });
});
