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
    await userEvent.click(screen.getByRole('combobox', { name: 'Тема' }));
    await userEvent.click(await screen.findByRole('option', { name: 'Тёмная контрастная' }));
    expect(onThemeChange).toHaveBeenCalledWith('dark-hc');
    await userEvent.click(screen.getByRole('button', { name: 'Закрыть' }));
    expect(onClose).toHaveBeenCalled();
  });
});
