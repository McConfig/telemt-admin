import { Provider } from '@gravity-ui/uikit';
import type { ReactNode } from 'react';
import type { AppTheme } from '../theme';

export function AppProviders({
  theme,
  children,
}: {
  theme: AppTheme;
  children: ReactNode;
}) {
  return (
    <Provider theme={theme} lang="ru">
      {children}
    </Provider>
  );
}
