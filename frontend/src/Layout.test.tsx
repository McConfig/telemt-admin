import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import Layout from './Layout';
import { me } from './test/data';
import { renderUI } from './test/render';
import { stubFetch } from './test/stubFetch';

describe('Layout', () => {
  it('показывает меню, тему, выход и футер', async () => {
    stubFetch({ '/api/meta': { version: 'v9.9.9', repo: 'https://example.com/repo' } });
    const onLogout = vi.fn();
    const onToggleTheme = vi.fn();
    renderUI(
      <Layout me={me} onLogout={onLogout} theme="dark" onToggleTheme={onToggleTheme}>
        <div>содержимое</div>
      </Layout>,
    );
    expect(screen.getByText('содержимое')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Меню' }));
    expect(screen.getByText('Пользователи UI')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Переключить тему' }));
    expect(onToggleTheme).toHaveBeenCalled();
    await userEvent.click(screen.getByRole('button', { name: 'Выйти' }));
    expect(onLogout).toHaveBeenCalled();
    expect(await screen.findByText(/v9.9.9/)).toBeInTheDocument();
  });
});
