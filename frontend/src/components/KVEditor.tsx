// Editable list of enable-able key/value rows (params, headers, form fields).
import { Index } from "solid-js";
import { Plus, X } from "lucide-solid";
import { ICON } from "../lib/icons";
import type { KV } from "../lib/api";
import { blankKV } from "../lib/factory";
import VarInput from "./VarInput";

type Props = {
  rows: KV[];
  onChange: (rows: KV[]) => void;
  keyPlaceholder?: string;
  valuePlaceholder?: string;
};

export default function KVEditor(props: Props) {
  const update = (i: number, patch: Partial<KV>) => {
    const next = props.rows.map((r, idx) => (idx === i ? { ...r, ...patch } : r));
    props.onChange(next);
  };
  const remove = (i: number) =>
    props.onChange(props.rows.filter((_, idx) => idx !== i));
  const add = () => props.onChange([...props.rows, blankKV()]);

  return (
    <div class="kv-editor">
      <Index each={props.rows}>
        {(row, i) => (
          <div class="kv-row" classList={{ disabled: !row().enabled }}>
            <input
              type="checkbox"
              checked={row().enabled}
              onChange={(e) => update(i, { enabled: e.currentTarget.checked })}
              title="Enable / disable"
            />
            <VarInput
              class="kv-key"
              placeholder={props.keyPlaceholder ?? "key"}
              value={row().key}
              onChange={(v) => update(i, { key: v })}
            />
            <VarInput
              class="kv-val"
              placeholder={props.valuePlaceholder ?? "value"}
              value={row().value}
              onChange={(v) => update(i, { value: v })}
            />
            <button class="icon-btn" onClick={() => remove(i)} title="Remove">
              <X size={ICON.sm} />
            </button>
          </div>
        )}
      </Index>
      <button class="add-row" onClick={add}>
        <Plus size={ICON.xs} /> Add row
      </button>
    </div>
  );
}
