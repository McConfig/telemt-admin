import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import SettingsPage from './SettingsPage';
import { renderUI } from '../test/render';
import { stubFetch } from '../test/stubFetch';

const catalog = {
  version: 0,
  sections: [
    {
      id: 'risk',
      title: 'Риск',
      description: 'Глобальные пороги',
      settings: [
        {
          key: 'risk.warn_ips',
          title: 'Предупреждение, больше IP',
          description: 'Порог предупреждения',
          kind: 'int',
          status: 'active',
          value: 3,
          default: 3,
          source: 'env',
          min: 0,
          max: 1000,
        },
        {
          key: 'risk.high_ips',
          title: 'Высокий риск, больше IP',
          description: 'Порог высокого риска',
          kind: 'int',
          status: 'active',
          value: 5,
          default: 5,
          source: 'env',
          min: 0,
          max: 1000,
        },
        {
          key: 'risk.home_country',
          title: 'Домашняя страна',
          description: 'Код ISO',
          kind: 'string',
          status: 'active',
          value: 'RU',
          default: 'RU',
          source: 'env',
        },
        {
          key: 'risk.new_countries',
          title: 'Новая страна считается риском',
          description: 'Глобальное значение',
          kind: 'bool',
          status: 'active',
          value: true,
          default: true,
          source: 'env',
        },
        {
          key: 'totp.key',
          title: 'Ключ TOTP',
          description: 'Секрет',
          kind: 'string',
          status: 'readonly',
          secret: true,
          secretSet: true,
        },
      ],
    },
    {
      id: 'notifications',
      title: 'Уведомления',
      description: 'Вебхуки',
      settings: [
        {
          key: 'notifications.webhook_url',
          title: 'URL вебхука',
          description: 'Куда отправлять события',
          kind: 'string',
          status: 'active',
          value: '',
          default: '',
          source: 'env',
        },
        {
          key: 'notifications.webhook_secret',
          title: 'Секрет вебхука',
          description: 'HMAC-подпись',
          kind: 'string',
          status: 'active',
          secret: true,
          secretSet: false,
          value: null,
          default: null,
          source: 'env',
        },
      ],
    },
    {
      id: 'security',
      title: 'Безопасность',
      description: 'Входы',
      settings: [
        {
          key: 'backup.offsite',
          title: 'Копия вне хоста',
          description: 'Копии вне хоста пока нет',
          kind: 'bool',
          status: 'planned',
          plannedRelease: '1.6.0',
          default: false,
        },
      ],
    },
  ],
};

function renderSettings() {
  stubFetch({
    '/api/settings': catalog,
    '/api/settings/webhook/test': { sent: true, status: 204 },
  });
  renderUI(<SettingsPage />);
}

describe('SettingsPage', () => {
  it('показывает active, readonly и planned и сохраняет изменение', async () => {
    renderSettings();
    expect(await screen.findByText('Настройки')).toBeInTheDocument();
    expect(screen.getByText('задан')).toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', { name: 'Безопасность' }));
    expect(screen.getByText('1.6.0')).toBeInTheDocument();
    expect(screen.getByText('Копии вне хоста пока нет')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Риск' }));

    const warn = screen.getByPlaceholderText('по умолчанию: 3');
    await userEvent.clear(warn);
    await userEvent.type(warn, '4');
    await userEvent.click(screen.getByRole('button', { name: 'Сохранить настройки' }));

    const mock = vi.mocked(fetch);
    const call = mock.mock.calls.find(
      ([url, init]) =>
        String(url).endsWith('/api/settings') &&
        (init as RequestInit | undefined)?.method === 'PUT',
    );
    expect(call).toBeTruthy();
    expect(JSON.parse(String((call?.[1] as RequestInit).body))).toMatchObject({
      version: 0,
      values: { 'risk.warn_ips': 4 },
    });
  });

  it('проверяет вебхук и показывает секрет как заданный/пустой', async () => {
    renderSettings();
    await screen.findByText('Настройки');
    await userEvent.click(screen.getByRole('button', { name: 'Безопасность' }));
    expect(screen.getByPlaceholderText('не задан')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Проверить вебхук' }));
    expect(await screen.findByText('Отправлено, статус 204')).toBeInTheDocument();
  });

  it('сбрасывает изменённое поле к значению по умолчанию', async () => {
    renderSettings();
    await screen.findByText('Настройки');
    const warn = screen.getByPlaceholderText('по умолчанию: 3');
    await userEvent.clear(warn);
    await userEvent.type(warn, '9');
    await userEvent.click(screen.getAllByRole('button', { name: 'по умолчанию' })[0]);
    expect(screen.getByPlaceholderText('по умолчанию: 3')).toHaveValue('3');
  });
});
