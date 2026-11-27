import React from 'react';
import { createRoot } from 'react-dom/client';
import '@gravity-ui/uikit/styles/styles.css';
import './styles.css';
import App from './App';
import { LocaleProvider } from './i18n';
import {settings} from '@gravity-ui/date-utils';

async function bootstrap() {
  await settings.loadLocale('ru');
  createRoot(document.getElementById('root')!).render(
    <React.StrictMode>
      <LocaleProvider>
        <App />
      </LocaleProvider>
    </React.StrictMode>,
  );
}

void bootstrap();
