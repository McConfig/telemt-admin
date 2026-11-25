import { render } from '@testing-library/react';
import { ThemeProvider } from '@gravity-ui/uikit';
import { MemoryRouter } from 'react-router-dom';
import type { ReactNode } from 'react';

export function renderUI(ui: ReactNode, route = '/') {
  return render(
    <ThemeProvider theme="dark">
      <MemoryRouter initialEntries={[route]}>{ui}</MemoryRouter>
    </ThemeProvider>,
  );
}
