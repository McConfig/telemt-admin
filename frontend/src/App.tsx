import { useEffect, useState } from 'react';
import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom';
import { Alert, Button, Loader } from '@gravity-ui/uikit';
import { api, type Me } from './api';
import Layout from './Layout';
import LoginPage from './pages/LoginPage';
import UsersPage from './pages/UsersPage';
import UserPage from './pages/UserPage';
import NewUserPage from './pages/NewUserPage';
import AuditPage from './pages/AuditPage';
import AccountPage from './pages/AccountPage';
import AdminsPage from './pages/AdminsPage';
import ServicesPage from './pages/ServicesPage';
import SettingsPage from './pages/SettingsPage';
import DiagnosticsPage from './pages/DiagnosticsPage';
import { AppProviders } from './providers/AppProviders';
import { normalizeTheme, type AppTheme } from './theme';

export default function App() {
  const [me, setMe] = useState<Me | null | undefined>(undefined);
  const [theme, setTheme] = useState<AppTheme>(() =>
    normalizeTheme(localStorage.getItem('tgadmin-theme')),
  );
  useEffect(() => {
    localStorage.setItem('tgadmin-theme', theme);
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
      <AppProviders theme={theme}>
        <div className="page">
          <Loader size="l" />
        </div>
      </AppProviders>
    );
  }
  if (me === null) {
    return (
      <AppProviders theme={theme}>
        <LoginPage onLogin={setMe} />
      </AppProviders>
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
      <AppProviders theme={theme}>
        <div className="app-shell">
          <main className="page stack">
            <Alert
              theme="danger"
              title="Требуется двухфакторная аутентификация"
              message="Льготный срок истёк. Доступ к интерфейсу ограничен, пока вы не включите 2FA. Настройте её ниже и сохраните резервные коды."
            />
            <AccountPage onMeChange={refreshMe} theme={theme} onThemeChange={setTheme} />
            <div>
              <Button view="outlined" onClick={() => void logout()}>
                Выйти
              </Button>
            </div>
          </main>
        </div>
      </AppProviders>
    );
  }

  return (
    <AppProviders theme={theme}>
    <BrowserRouter>
      <Layout me={me} onLogout={logout}>
        <Routes>
          <Route path="/" element={<UsersPage me={me} />} />
          <Route path="/users/new" element={<NewUserPage me={me} />} />
          <Route path="/users/:name" element={<UserPage me={me} />} />
          <Route path="/audit" element={<AuditPage />} />
          <Route path="/account" element={<AccountPage theme={theme} onThemeChange={setTheme} />} />
          <Route path="/admins" element={<AdminsPage me={me} />} />
          <Route
            path="/services"
            element={
              me.user.role === 'admin' ? <ServicesPage me={me} /> : <Navigate to="/" replace />
            }
          />
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
    </AppProviders>
  );
}
