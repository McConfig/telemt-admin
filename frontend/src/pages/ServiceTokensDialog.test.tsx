import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import ServiceTokensDialog from './ServiceTokensDialog';
import { renderUI } from '../test/render';
import { stubFetch } from '../test/stubFetch';
import type { IssuedServiceToken, ServiceAccount, ServiceToken } from '../api';

const service: ServiceAccount = {
  name: 'monitoring',
  description: 'Prometheus',
  scope: 'read',
  disabled: false,
  tokens: 1,
  createdAt: '2026-11-26T10:00:00Z',
  updatedAt: '2026-11-26T10:00:00Z',
  createdBy: 'admin',
};
const token: ServiceToken = {
  id: '0123456789abcdef',
  label: 'ci',
  prefix: 'tgadm_0123456789abcdef',
  createdAt: '2026-11-26T10:00:00Z',
  createdBy: 'admin',
};
const issued: IssuedServiceToken = {
  id: token.id,
  token: 'tgadm_0123456789abcdef_secret',
  label: token.label,
  prefix: token.prefix,
  createdAt: token.createdAt,
};

describe('ServiceTokensDialog', () => {
  it('выпускает токен и показывает его один раз', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { writeText },
    });
    let tokens = [token];
    stubFetch({
      '/api/services/monitoring/tokens': (_url: URL, init?: RequestInit) => {
        if (init?.method === 'POST') {
          tokens = [...tokens, token];
          return issued;
        }
        if (init?.method === 'DELETE') {
          tokens = [];
          return new Response(null, { status: 204 });
        }
        return tokens;
      },
    });
    renderUI(
      <ServiceTokensDialog service={service} open onClose={() => {}} onChanged={() => {}} />,
    );
    expect(await screen.findByText('ci')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Выпустить токен' }));
    await userEvent.click(screen.getByRole('button', { name: 'Выпустить' }));
    expect(await screen.findByText(issued.token)).toBeInTheDocument();
    expect(screen.getByText(/показывается один раз/)).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Скопировать' }));
    expect(writeText).toHaveBeenCalledWith(issued.token);
    expect(await screen.findByText('скопировано')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Готово' }));
    expect(screen.queryByText(issued.token)).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Выпустить токен' }));
    await userEvent.click(screen.getByRole('button', { name: 'Отмена' }));
    await userEvent.click(screen.getByRole('button', { name: 'Закрыть' }));
  });

  it('показывает предупреждение для отключённого сервиса', async () => {
    stubFetch({
      '/api/services/monitoring/tokens': [],
    });
    renderUI(
      <ServiceTokensDialog
        service={{ ...service, disabled: true }}
        open
        onClose={() => {}}
        onChanged={() => {}}
      />,
    );
    expect(await screen.findByText(/Сервис отключён/)).toBeInTheDocument();
  });

  it('отзывает токен после подтверждения', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    let tokens = [token];
    stubFetch({
      '/api/services/monitoring/tokens': (_url: URL, init?: RequestInit) => {
        if (init?.method === 'DELETE') {
          tokens = [];
          return new Response(null, { status: 204 });
        }
        return tokens;
      },
    });
    renderUI(
      <ServiceTokensDialog service={service} open onClose={() => {}} onChanged={() => {}} />,
    );
    await screen.findByText('ci');
    await userEvent.click(screen.getByRole('button', { name: 'Отозвать ci' }));
    await waitFor(() => expect(screen.queryByText('ci')).not.toBeInTheDocument());
  });
});
