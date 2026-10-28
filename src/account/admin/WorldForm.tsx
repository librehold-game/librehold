import { useT } from '../../i18n/i18n';
import { FIELDS, MODIFIERS, type FieldSpec, type SideModifiers, type WorldConfig } from '../worldConfig';

function Modifiers({ id, spec, value, onChange }: { id: string; spec: FieldSpec; value: SideModifiers; onChange: (v: SideModifiers) => void }) {
  const t = useT();
  return (
    <fieldset className="field">
      <legend>{t(`admin.field.${spec.key}`)}</legend>
      <div className="row">
        {MODIFIERS.map((k) => (
          <span key={k} className="row">
            <label htmlFor={`${id}-${k}`}>{t(`admin.modifier.${k}`)}</label>
            <input id={`${id}-${k}`} type="number" min={spec.min} max={spec.max} step={spec.step} value={value[k]}
              onChange={(e) => onChange({ ...value, [k]: Number(e.target.value) })} />
          </span>
        ))}
      </div>
    </fieldset>
  );
}

/**
 * The settings of a world. Creating, every field is editable and the creation-only ones are marked; editing
 * a running world, those are shown but locked.
 */
export function WorldForm({ idPrefix, value, onChange, mode }: {
  idPrefix: string; value: WorldConfig; onChange: (v: WorldConfig) => void; mode: 'create' | 'edit';
}) {
  const t = useT();
  const choiceLabel = (spec: FieldSpec, v: string) => (spec.key === 'mapSize' ? `${v} × ${v}` : t(`admin.choice.${spec.key}.${v}`));
  return (
    <div className="grid">
      {FIELDS.map((spec) => {
        const id = `${idPrefix}-${spec.key}`;
        const locked = mode === 'edit' && spec.creationOnly;
        const set = (v: unknown) => onChange({ ...value, [spec.key]: v });
        if (spec.key === 'humanModifiers' || spec.key === 'botModifiers') {
          return <Modifiers key={spec.key} id={id} spec={spec} value={value[spec.key]} onChange={set} />;
        }
        if (spec.flag) {
          return (
            <div key={spec.key} className="field">
              <label htmlFor={id}>
                <input id={id} type="checkbox" checked={!!value[spec.key]} disabled={locked} onChange={(e) => set(e.target.checked)} />
                {' '}{t(`admin.field.${spec.key}`)}
              </label>
            </div>
          );
        }
        const current = (value[spec.key] ?? spec.min) as string | number;
        return (
          <div key={spec.key} className="field">
            <label htmlFor={id}>
              {t(`admin.field.${spec.key}`)}
              {spec.creationOnly && <small className="muted"> — {t('admin.creationOnly')}</small>}
            </label>
            {spec.choices ? (
              <select id={id} value={String(current)} disabled={locked}
                onChange={(e) => set(typeof current === 'number' ? Number(e.target.value) : e.target.value)}>
                {spec.choices.map((v) => <option key={v} value={v}>{choiceLabel(spec, v)}</option>)}
              </select>
            ) : (
              <input id={id} type="number" min={spec.min} max={spec.max} step={spec.step} value={current} disabled={locked}
                onChange={(e) => set(Number(e.target.value))} />
            )}
          </div>
        );
      })}
    </div>
  );
}

/** The live settings that differ between two configs, as a PATCH body. */
export function liveChanges(before: WorldConfig, after: WorldConfig): Partial<WorldConfig> {
  const out: Record<string, unknown> = {};
  for (const spec of FIELDS) {
    if (!spec.creationOnly && JSON.stringify(before[spec.key]) !== JSON.stringify(after[spec.key])) out[spec.key] = after[spec.key];
  }
  return out as Partial<WorldConfig>;
}
