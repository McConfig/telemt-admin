import { useEffect, useState } from 'react';
import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom';
import { Alert, Button, Loader, ThemeProvider } from '@gravity-ui/uikit';
import { api, type Me } from './api';
import Layout from './Layout';
import LoginPage from './pages/LoginPage';
import UsersPage from './pages/UsersPage';
import UserPage from './pages/UserPage';
import NewUserPage from './pages/NewUserPage';
import AuditPage from './pages/AuditPage';
import AccountPage from './pages/AccountPage';
import AdminsPage from './pages/AdminsPage';
import SettingsPage from './pages/SettingsPage';
import DiagnosticsPage from './pages/DiagnosticsPage';

export default function App() {
  const [me, setMe] = useState<Me | null | undefined>(undefined);
  const [theme, setTheme] = useState<'dark' | 'light'>(
    () => (localStorage.getItem('tgadmin-theme') === 'light' ? 'light' : 'dark'),
  );
  useEffect(() => {
    localStorage.setItem('tgadmin-theme', theme);
    document.documentElement.dataset.theme = theme;
  }, [theme]);

  useEffect(() => {
    let alive = true;
    api
      .me()
      .then((value) => alive && setMe(value))
      .catch(() => alive && setMe(null));
    const onUnauthorized = () => setMe(null);
    window.addEventListener('tgadmin:unauthorized', onUnauthorized);
    return () => {
      alive = false;
      window.removeEventListener('tgadmin:unauthorized', onUnauthorized);
    };
  }, []);

  if (me === undefined) {
    return (
      <ThemeProvider theme={theme}>
        <div className="page">
          <Loader size="l" />
        </div>
      </ThemeProvider>
    );
  }
  if (me === null) {
    return (
      <ThemeProvider theme={theme}>
        <LoginPage onLogin={setMe} />
      </ThemeProvider>
    );
  }
  const logout = async () => {
    try {
      await api.logout();
    } finally {
      setMe(null);
    }
  };

  const refreshMe = () => {
    api
      .me()
      .then(setMe)
      .catch(() => setMe(null));
  };

  if (me.require2fa?.enforced) {
    return (
      <ThemeProvider theme={theme}>
        <div className="app-shell">
          <main className="page stack">
            <Alert
              theme="danger"
              title="Требуется двухфакторная аутентификация"
              message="Льготный срок истёк. Доступ к интерфейсу ограничен, пока вы не включите 2FA. Настройте её ниже и сохраните резервные коды."
            />
            <AccountPage onMeChange={refreshMe} />
            <div>
              <Button view="outlined" onClick={() => void logout()}>
                Выйти
              </Button>
            </div>
          </main>
        </div>
      </ThemeProvider>
    );
  }

  return (
    <ThemeProvider theme={theme}>
    <BrowserRouter>
      <Layout me={me} onLogout={logout} theme={theme} onToggleTheme={() => setTheme(theme === 'dark' ? 'light' : 'dark')}>
        <Routes>
          <Route path="/" element={<UsersPage me={me} />} />
          <Route path="/users/new" element={<NewUserPage me={me} />} />
          <Route path="/users/:name" element={<UserPage me={me} />} />
          <Route path="/audit" element={<AuditPage />} />
          <Route path="/account" element={<AccountPage />} />
          <Route path="/admins" element={<AdminsPage me={me} />} />
          <Route
            path="/settings"
            element={me.user.role === 'admin' ? <SettingsPage /> : <Navigate to="/" replace />}
          />
          <Route
            path="/diagnostics"
            element={me.user.role === 'admin' ? <DiagnosticsPage /> : <Navigate to="/" replace />}
          />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </Layout>
    </BrowserRouter>
    </ThemeProvider>
  );
}
