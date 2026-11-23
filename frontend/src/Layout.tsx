import type { ReactNode } from 'react';
import { NavLink } from 'react-router-dom';
import { Button, Label } from '@gravity-ui/uikit';
import type { Me } from './api';

export default function Layout({
  me,
  onLogout,
  theme,
  onToggleTheme,
  children,
}: {
  me: Me;
  onLogout: () => void;
  theme: 'dark' | 'light';
  onToggleTheme: () => void;
  children: ReactNode;
}) {
  return (
    <>
      <header className="topbar">
        <span className="brand">TeleMT Admin</span>
        <nav>
          <NavLink to="/" end>
            Пользователи
          </NavLink>
          <NavLink to="/audit">Аудит</NavLink>
          <NavLink to="/account">Аккаунт</NavLink>
          {me.user.role === 'admin' && <NavLink to="/admins">Пользователи UI</NavLink>}
        </nav>
        <span className="muted">{me.user.name}</span>
        <Label theme={me.user.role === 'admin' ? 'success' : 'info'} size="s">
          {me.user.role}
        </Label>
        <Button view="flat" onClick={onToggleTheme}>
          {theme === 'dark' ? 'Светлая' : 'Тёмная'}
        </Button>
        <Button view="flat" onClick={onLogout}>
          Выйти
        </Button>
      </header>
      <main className="page">{children}</main>
    </>
  );
}
