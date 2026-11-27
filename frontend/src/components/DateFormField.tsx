import { DateField } from '@gravity-ui/date-components';
import { dateTimeParse } from '@gravity-ui/date-utils';
import { FormField } from './FormField';

export function DateFormField({
  label,
  value,
  onUpdate,
  withTime = false,
  disabled,
  placeholder,
}: {
  label: string;
  value: string;
  onUpdate: (value: string) => void;
  withTime?: boolean;
  disabled?: boolean;
  placeholder?: string;
}) {
  const displayFormat = withTime ? 'YYYY-MM-DD HH:mm' : 'YYYY-MM-DD';
  const apiFormat = withTime ? 'YYYY-MM-DDTHH:mm' : 'YYYY-MM-DD';
  const normalized = withTime ? value.replace('T', ' ').slice(0, 16) : value;
  const date = normalized ? dateTimeParse(normalized, { format: displayFormat }) ?? null : null;

  return (
    <FormField label={label}>
      <DateField
        aria-label={label}
        value={date}
        format={displayFormat}
        hasClear
        disabled={disabled}
        placeholder={placeholder ?? displayFormat}
        onUpdate={(next) => onUpdate(next ? next.format(apiFormat) : '')}
      />
    </FormField>
  );
}
