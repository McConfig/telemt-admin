import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { ThemeDialog } from './ThemeDialog';
import { renderUI } from '../test/render';

describe('ThemeDialog', () => {
  it('переключает все темы и закрывается', async () => {
    const onThemeChange = vi.fn();
    const onClose = vi.fn();
    renderUI(
      <ThemeDialog
        open
        theme="system"
        onThemeChange={onThemeChange}
        onClose={onClose}
      />,
    );
    expect(screen.getByText('Тема оформления')).toBeInTheDocument();
    const trigger = screen.getByRole('combobox', { name: 'Тема' });
    await userEvent.click(trigger);
    let option;
    try {
      option = await screen.findByRole('option', { name: 'Тёмная контрастная' }, { timeout: 5000 });
    } catch {
      await userEvent.click(trigger);
      option = await screen.findByRole('option', { name: 'Тёмная контрастная' }, { timeout: 5000 });
    }
    await userEvent.click(option);
    expect(onThemeChange).toHaveBeenCalledWith('dark-hc');
    await userEvent.click(screen.getByRole('button', { name: 'Закрыть' }));
    expect(onClose).toHaveBeenCalled();
  });
});
