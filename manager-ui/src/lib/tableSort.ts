// Shared table sorting: makes every data column sortable without hand-writing a
// comparator per column. `sortable(columns)` attaches a type-aware sorter to any
// column that has a `dataIndex` and no explicit `sorter`, so tables across the
// app sort consistently (numbers numerically, dates chronologically, strings
// naturally — with nulls last-ish and digit-aware string ordering).
import type { ColumnsType, ColumnType } from "antd/es/table";

/** Null-safe comparator: numbers, booleans, ISO dates, then natural strings. */
export function compareValues(a: unknown, b: unknown): number {
  if (a == null && b == null) return 0;
  if (a == null) return -1;
  if (b == null) return 1;
  if (typeof a === "number" && typeof b === "number") return a - b;
  if (typeof a === "boolean" && typeof b === "boolean") return Number(a) - Number(b);

  const as = String(a);
  const bs = String(b);
  // Chronological when both look like dates (ISO-ish), not lexicographic.
  if (/^\d{4}-\d{2}-\d{2}/.test(as) && /^\d{4}-\d{2}-\d{2}/.test(bs)) {
    const ad = Date.parse(as);
    const bd = Date.parse(bs);
    if (!Number.isNaN(ad) && !Number.isNaN(bd)) return ad - bd;
  }
  return as.localeCompare(bs, undefined, { numeric: true, sensitivity: "base" });
}

/** Reads a column's value off a row, supporting nested dataIndex arrays. */
function valueAt<T>(row: T, dataIndex: string | number | readonly (string | number)[]): unknown {
  if (Array.isArray(dataIndex)) {
    return dataIndex.reduce<unknown>(
      (acc, key) => (acc == null ? acc : (acc as Record<string | number, unknown>)[key]),
      row,
    );
  }
  return (row as Record<string | number, unknown>)[dataIndex as string | number];
}

/**
 * `NoInfer` keeps T tied to the table's row type (from the contextual
 * `columns` prop) rather than being inferred from the column literals.
 *
 * Returns the columns with a sorter added to every `dataIndex` column that does
 * not already define one. Columns without a dataIndex (pure render/action cells)
 * are left untouched — give those an explicit `sorter` where it makes sense.
 */
export function sortable<T>(columns: ColumnsType<NoInfer<T>>): ColumnsType<T> {
  return (columns as ColumnsType<T>).map((col) => {
    const c = col as ColumnType<T>;
    if (c.dataIndex == null || c.sorter) return col;
    const di = c.dataIndex as string | number | readonly (string | number)[];
    return {
      ...c,
      sorter: (a: T, b: T) => compareValues(valueAt(a, di), valueAt(b, di)),
    } as ColumnType<T>;
  });
}
