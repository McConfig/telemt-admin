import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import AuditPage from './AuditPage';
import { renderUI } from '../test/render';
import { stubFetch } from '../test/stubFetch';
import { chooseSelect } from '../test/select';

function events(count: number) {
  return Array.from({ length: count }, (_, index) => ({
    at: `2026-11-${String((index % 25) + 1).padStart(2, '0')}T10:00:00Z`,
    actor: `admin${index}`,
    action: 'user.create',
    target: `client${index}`,
    result: 'ok',
    ip: '192.0.2.1',
  }));
}

function auditUrls() {
  return vi
    .mocked(fetch)
    .mock.calls.map(([url]) => String(url))
    .filter((url) => url.includes('/api/audit?'));
}

describe('AuditPage — расширенные сценарии', () => {
  it('листает страницы и показывает мобильные карточки', async () => {
    vi.spyOn(window, 'matchMedia').mockReturnValue({
      matches: true,
      media: '(max-width: 760px)',
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    } as unknown as MediaQueryList);
    stubFetch({ '/api/audit': events(50) });
    renderUI(<AuditPage />);
    expect((await screen.findAllByText('user.create')).length).toBeGreaterThan(0);
    expect(screen.getByRole('link', { name: 'Экспорт CSV' })).toHaveAttribute(
      'href',
      '/api/audit.csv',
    );

    await userEvent.click(screen.getByRole('button', { name: 'Вперёд' }));
    expect(await screen.findByText('страница 2')).toBeInTheDocument();
    await waitFor(() =>
      expect(auditUrls().some((url) => url.includes('offset=50'))).toBe(true),
    );

    await userEvent.click(screen.getByRole('button', { name: 'Назад' }));
    expect(await screen.findByText('страница 1')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Вперёд' }));
    await screen.findByText('страница 2');
  });

  it('фильтрует по датам и действию', async () => {
    stubFetch({ '/api/audit': events(1) });
    renderUI(<AuditPage />);
    await screen.findByText('user.create');

    expect(screen.getByRole('textbox', { name: 'С' })).toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: 'По' })).toBeInTheDocument();
    await chooseSelect('Действие', 'user.rotate');
    await waitFor(() => {
      const urls = auditUrls();
      expect(urls.some((url) => url.includes('action=user.rotate'))).toBe(true);
    });
  });

  it('показывает пустое состояние и ошибку загрузки', async () => {
    vi.spyOn(window, 'matchMedia').mockReturnValue({
      matches: true,
      media: '(max-width: 760px)',
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    } as unknown as MediaQueryList);
    stubFetch({ '/api/audit': [] });
    const view = renderUI(<AuditPage />);
    expect(await screen.findByText('Событий нет')).toBeInTheDocument();
    view.unmount();

    stubFetch({
      '/api/audit': () =>
        new Response(JSON.stringify({ error: { code: 'x', message: 'аудит недоступен' } }), {
          status: 500,
          headers: { 'Content-Type': 'application/json' },
        }),
    });
    renderUI(<AuditPage />);
    expect(await screen.findByText('аудит недоступен')).toBeInTheDocument();
  });
});
