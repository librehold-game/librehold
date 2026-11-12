import { useState, type InputHTMLAttributes } from "react";

type Props = Omit<
  InputHTMLAttributes<HTMLInputElement>,
  "type" | "value" | "onChange" | "min" | "max" | "step"
> & {
  value: number;
  onValue: (n: number) => void;
  min?: number;
  max?: number;
  step?: number;
};

/**
 * A number field that shows exactly what the player types while it has focus. Parsing and clamping on every
 * keystroke would turn an emptied field back into its minimum at once, so only a complete number is passed on,
 * clamped, and the text is tidied when the field is left.
 */
export function NumberInput({
  value,
  onValue,
  min,
  max,
  step,
  onFocus,
  onBlur,
  ...rest
}: Props) {
  const [draft, setDraft] = useState<string | null>(null);
  const whole = step === undefined || Number.isInteger(step);
  return (
    <input
      {...rest}
      type="number"
      inputMode={whole ? "numeric" : "decimal"}
      min={min}
      max={max}
      step={step}
      value={draft ?? value}
      onFocus={(e) => {
        setDraft(String(value));
        onFocus?.(e);
      }}
      onBlur={(e) => {
        setDraft(null);
        onBlur?.(e);
      }}
      onChange={(e) => {
        const text = e.target.value;
        if (draft !== null) setDraft(text);
        if (text.trim() === "") return;
        let n = Number(text);
        if (!Number.isFinite(n)) return;
        if (whole) n = Math.floor(n);
        if (min !== undefined) n = Math.max(min, n);
        if (max !== undefined) n = Math.min(max, n);
        onValue(n);
      }}
    />
  );
}
