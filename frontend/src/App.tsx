import { useEffect, useState } from 'react';
import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom';
import { Loader } from '@gravity-ui/uikit';
import { api, type Me } from './api';
import Layout from './Layout';
import LoginPage from './pages/LoginPage';
import UsersPage from './pages/UsersPage';
import UserPage from './pages/UserPage';
import NewUserPage from './pages/NewUserPage';
import AuditPage from './pages/AuditPage';
import AccountPage from './pages/AccountPage';

export default function App() {
  const [me, setMe] = useState<Me | null | undefined>(undefined);

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
      <div className="page">
        <Loader size="l" />
      </div>
    );
  }
  if (me === null) {
    return <LoginPage onLogin={setMe} />;
  }
  const logout = async () => {
    try {
      await api.logout();
    } finally {
      setMe(null);
    }
  };

  return (
    <BrowserRouter>
      <Layout me={me} onLogout={logout}>
        <Routes>
          <Route path="/" element={<UsersPage me={me} />} />
          <Route path="/users/new" element={<NewUserPage me={me} />} />
          <Route path="/users/:name" element={<UserPage me={me} />} />
          <Route path="/audit" element={<AuditPage />} />
          <Route path="/account" element={<AccountPage />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </Layout>
    </BrowserRouter>
  );
}
