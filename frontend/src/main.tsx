import React from 'react';
import { createRoot } from 'react-dom/client';
import { ThemeProvider } from '@gravity-ui/uikit';
import '@gravity-ui/uikit/styles/styles.css';
import './styles.css';
import App from './App';

createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <ThemeProvider theme="dark">
      <App />
    </ThemeProvider>
  </React.StrictMode>,
);
