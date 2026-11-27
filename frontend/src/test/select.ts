import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

export async function chooseSelect(label: string, option: string) {
  const trigger = screen.getByRole('combobox', { name: label });
  await userEvent.click(trigger);
  let item;
  try {
    item = await screen.findByRole('option', { name: option }, { timeout: 1000 });
  } catch {
    await userEvent.click(trigger);
    item = await screen.findByRole('option', { name: option }, { timeout: 3000 });
  }
  await userEvent.click(item);
}
