// @vitest-environment happy-dom
import { describe, it, expect, afterEach } from 'vitest';
import { readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import { useState } from 'react';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { NumberInput } from '../src/shared/NumberInput';

afterEach(() => cleanup());

function Harness({ start, min, max, step, seen }: { start: number; min?: number; max?: number; step?: number; seen: number[] }) {
  const [value, setValue] = useState(start);
  return (
    <>
      <NumberInput aria-label="n" value={value} min={min} max={max} step={step} onValue={(n) => { seen.push(n); setValue(n); }} />
      <button onClick={() => setValue(7)}>seven</button>
    </>
  );
}

const field = () => screen.getByLabelText('n') as HTMLInputElement;
const type = (text: string) => fireEvent.change(field(), { target: { value: text } });

describe('number input', () => {
  it('can be emptied and typed into without snapping back', () => {
    const seen: number[] = [];
    render(<Harness start={0} min={1} max={40} seen={seen} />);
    fireEvent.focus(field());
    type('');
    expect(field().value).toBe('');
    type('2');
    type('25');
    expect(field().value).toBe('25');
    expect(seen).toEqual([2, 25]);
  });

  it('keeps what was typed out of range until the field is left', () => {
    const seen: number[] = [];
    render(<Harness start={19} min={1} max={40} seen={seen} />);
    fireEvent.focus(field());
    type('5');
    type('50');
    expect(field().value).toBe('50');
    expect(seen).toEqual([5, 40]);
    fireEvent.blur(field());
    expect(field().value).toBe('40');
  });

  it('puts the value back when left empty', () => {
    const seen: number[] = [];
    render(<Harness start={19} seen={seen} />);
    fireEvent.focus(field());
    type('');
    fireEvent.blur(field());
    expect(field().value).toBe('19');
    expect(seen).toEqual([]);
  });

  it('rounds counts down and keeps fractions where the step allows them', () => {
    const counts: number[] = [];
    render(<Harness start={1} seen={counts} />);
    fireEvent.focus(field());
    type('3.7');
    expect(counts).toEqual([3]);
    cleanup();
    const speeds: number[] = [];
    render(<Harness start={1} step={0.5} seen={speeds} />);
    fireEvent.focus(field());
    type('1.5');
    expect(speeds).toEqual([1.5]);
  });

  it('shows a value set from outside once the field is left', () => {
    render(<Harness start={1} seen={[]} />);
    fireEvent.focus(field());
    type('');
    fireEvent.blur(field());
    fireEvent.click(screen.getByText('seven'));
    expect(field().value).toBe('7');
  });

  it('is the only number field in the client', () => {
    const files = (dir: string): string[] => readdirSync(dir, { withFileTypes: true })
      .flatMap((e) => (e.isDirectory() ? files(join(dir, e.name)) : e.name.endsWith('.tsx') ? [join(dir, e.name)] : []));
    const raw = files(join(__dirname, '../src'))
      .filter((f) => !f.endsWith(join('shared', 'NumberInput.tsx')) && readFileSync(f, 'utf8').includes('type="number"'));
    expect(raw).toEqual([]);
  });
});
