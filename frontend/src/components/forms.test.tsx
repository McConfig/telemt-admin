import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { FormSelect } from './FormSelect';
import { DateFormField } from './DateFormField';
import { renderUI } from '../test/render';

describe('формы Gravity UI', () => {
  it('FormSelect отдаёт строку из массива Select', async () => {
    const onUpdate = vi.fn();
    renderUI(
      <FormSelect
        label="Период"
        value="monthly"
        options={[
          { value: 'monthly', content: 'Месяц' },
          { value: 'weekly', content: 'Неделя' },
        ]}
        onUpdate={onUpdate}
      />,
    );
    await userEvent.click(screen.getByRole('combobox', { name: 'Период' }));
    await userEvent.click(screen.getByRole('option', { name: 'Неделя' }));
    expect(onUpdate).toHaveBeenCalledWith('weekly');
  });

  it('DateFormField переводит DateTime в строку API', async () => {
    const onUpdate = vi.fn();
    renderUI(
      <DateFormField
        label="Действует до"
        value="2026-12-31T23:59"
        withTime
        onUpdate={onUpdate}
      />,
    );
    const input = screen.getByRole('textbox', { name: 'Действует до' }) as HTMLInputElement;
    expect(input.value.replace(/[\u200e\u200f\u2066-\u2069]/g, '')).toBe(
      '2026-12-31 23:59',
    );
  });
});
