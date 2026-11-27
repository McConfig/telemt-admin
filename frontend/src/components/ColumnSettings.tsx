import { useEffect, useState, type Dispatch, type SetStateAction } from 'react';
import { Button, Checkbox, Divider, Icon, Popup } from '@gravity-ui/uikit';
import { Ellipsis } from '@gravity-ui/icons';
import type { ColumnDef } from '@gravity-ui/table/tanstack';
import type { Table, VisibilityState } from '@tanstack/react-table';

export function usePersistentColumnVisibility(
  userName: string | undefined,
  tableId: string,
  defaults: VisibilityState = {},
  legacyKey?: string,
): [VisibilityState, Dispatch<SetStateAction<VisibilityState>>] {
  const key = `tgadmin-columns:${userName ?? 'anonymous'}:${tableId}`;
  const [visibility, setVisibility] = useState<VisibilityState>(() => {
    try {
      const raw = localStorage.getItem(key) ?? (legacyKey ? localStorage.getItem(legacyKey) : null);
      return raw ? (JSON.parse(raw) as VisibilityState) : defaults;
    } catch {
      return defaults;
    }
  });
  useEffect(() => {
    localStorage.setItem(key, JSON.stringify(visibility));
  }, [key, visibility]);
  return [visibility, setVisibility];
}

export function settingsColumn<T>(labels: Record<string, string>): ColumnDef<T> {
  return {
    id: 'settings',
    header: ({ table }) => <ColumnSettings table={table} labels={labels} />,
    size: 44,
    enableHiding: false,
    meta: { hideInSettings: true },
    cell: () => null,
  };
}

function ColumnSettings<T>({ table, labels }: { table: Table<T>; labels: Record<string, string> }) {
  const [open, setOpen] = useState(false);
  const [anchor, setAnchor] = useState<HTMLElement | null>(null);
  const columns = table
    .getAllLeafColumns()
    .filter(
      (column) =>
        column.id !== 'select' && column.id !== 'settings' && column.getCanHide(),
    );

  return (
    <>
      <span ref={setAnchor}>
        <Button
          view="flat"
          size="s"
          aria-label="Настроить колонки"
          title="Настроить колонки"
          onClick={() => setOpen(!open)}
        >
          <Icon data={Ellipsis} size={16} />
        </Button>
      </span>
      <Popup open={open} anchorElement={anchor} onOpenChange={setOpen} placement="bottom-end">
        <div className="column-settings">
          {columns.map((column) => (
            <Checkbox
              key={column.id}
              checked={column.getIsVisible()}
              onUpdate={(checked) => column.toggleVisibility(checked)}
              content={labels[column.id] ?? column.id}
            />
          ))}
          <Divider />
          <Button view="flat" size="s" onClick={() => table.resetColumnVisibility()}>
            Сбросить
          </Button>
        </div>
      </Popup>
    </>
  );
}
