import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it } from 'vitest';
import { LocaleProvider, LOCALE_STORAGE_KEY, proxyDescription, useLocale } from './index';
import { renderUI } from '../test/render';

function Probe() {
  const { locale, setLocale, t } = useLocale();
  return (
    <div>
      <span data-testid="locale">{locale}</span>
      <span data-testid="fallback">{t('missing.key', 'запасной текст')}</span>
      <button onClick={() => setLocale('en')}>english</button>
    </div>
  );
}

describe('i18n', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('русский словарь приоритетнее английской схемы telemt', () => {
    const text = proxyDescription('general.links.public_host', 'English description', 'ru');
    expect(text).toContain('Переопределение общедоступного имени');
  });

  it('неизвестный путь откатывается к описанию схемы', () => {
    expect(proxyDescription('unknown.path', 'Schema text', 'ru')).toBe('Schema text');
  });

  it('для английского языка остаётся описание схемы', () => {
    expect(proxyDescription('general.links.public_host', 'Schema text', 'en')).toBe('Schema text');
  });

  it('провайдер хранит язык и отдаёт fallback', async () => {
    renderUI(
      <LocaleProvider>
        <Probe />
      </LocaleProvider>,
    );
    expect(screen.getByTestId('locale')).toHaveTextContent('ru');
    expect(screen.getByTestId('fallback')).toHaveTextContent('запасной текст');
    await userEvent.click(screen.getByRole('button', { name: 'english' }));
    expect(screen.getByTestId('locale')).toHaveTextContent('en');
    expect(localStorage.getItem(LOCALE_STORAGE_KEY)).toBe('en');
  });
});
