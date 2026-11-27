import { Select } from '@gravity-ui/uikit';
import { FormField } from './FormField';

export function FormSelect({
  label,
  value,
  options,
  onUpdate,
  disabled,
  placeholder,
  qa,
}: {
  label: string;
  value: string;
  options: { value: string; content: string }[];
  onUpdate: (value: string) => void;
  disabled?: boolean;
  placeholder?: string;
  qa?: string;
}) {
  return (
    <FormField label={label}>
      <Select
        qa={qa}
        aria-label={label}
        value={value ? [value] : []}
        options={options}
        onUpdate={([next]) => onUpdate(next ?? '')}
        disabled={disabled}
        placeholder={placeholder}
        width="max"
      />
    </FormField>
  );
}
