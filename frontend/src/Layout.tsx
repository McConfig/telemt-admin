import { useState, type ReactNode } from 'react';
import { Link, NavLink } from 'react-router-dom';
import { Button, Label } from '@gravity-ui/uikit';
import { ArrowRightFromSquare, Moon, Sun } from '@gravity-ui/icons';
import type { Me } from './api';
import Footer from './Footer';

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
  const [menuOpen, setMenuOpen] = useState(false);
  const close = () => setMenuOpen(false);
  const ThemeIcon = theme === 'dark' ? Sun : Moon;

  return (
    <div className="app-shell">
      <header className="topbar">
        <span className="brand">TeleMT Admin</span>
        <button
          className="menu-toggle"
          type="button"
          aria-label="Меню"
          onClick={() => setMenuOpen(!menuOpen)}
        >
          {menuOpen ? 'Закрыть' : 'Меню'}
        </button>
        <div className={`topbar-panel${menuOpen ? ' open' : ''}`}>
          <nav onClick={close}>
            <NavLink to="/" end>
              Пользователи
            </NavLink>
            <NavLink to="/audit">Аудит</NavLink>
            {me.user.role === 'admin' && <NavLink to="/admins">Пользователи UI</NavLink>}
          </nav>
          <div className="topbar-user">
            <Link to="/account" onClick={close} title={`Аккаунт, роль: ${me.user.role}`}>
              <Label theme={me.user.role === 'admin' ? 'success' : 'info'} size="s">
                {me.user.name}
              </Label>
            </Link>
            <Button view="flat" aria-label="Переключить тему" title="Тема" onClick={onToggleTheme}>
              <ThemeIcon width={18} height={18} />
            </Button>
            <Button view="flat" aria-label="Выйти" title="Выйти" onClick={onLogout}>
              <ArrowRightFromSquare width={18} height={18} />
            </Button>
          </div>
        </div>
      </header>
      <main className="page">{children}</main>
      <Footer />
    </div>
  );
}
