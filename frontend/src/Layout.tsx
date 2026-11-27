import { useEffect, useState, type ReactNode } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import { Alert } from '@gravity-ui/uikit';
import { AsideHeader, FooterItem } from '@gravity-ui/navigation';
import {
  ArrowRightFromSquare,
  FileText,
  Gear,
  Key,
  Person,
  Persons,
  Server,
  ShieldCheck,
  Stethoscope,
  Sun,
} from '@gravity-ui/icons';
import type { Me } from './api';
import type { AppTheme } from './theme';
import { ThemeSelect } from './components/ThemeSelect';
import { ThemeDialog } from './components/ThemeDialog';
import Footer from './Footer';
import { useMedia } from './useMedia';

const MENU_PATHS: Record<string, string> = {
  clients: '/',
  audit: '/audit',
  admins: '/admins',
  services: '/services',
  settings: '/settings',
  diagnostics: '/diagnostics',
};

export default function Layout({
  me,
  onLogout,
  theme,
  onThemeChange,
  children,
}: {
  me: Me;
  onLogout: () => void;
  theme: AppTheme;
  onThemeChange: (theme: AppTheme) => void;
  children: ReactNode;
}) {
  const navigate = useNavigate();
  const location = useLocation();
  const [themeDialogOpen, setThemeDialogOpen] = useState(false);
  const isMobile = useMedia('(max-width: 960px)');
  const [compact, setCompact] = useState(() => {
    const saved = localStorage.getItem('tgadmin-sidebar-compact');
    return saved === 'true';
  });
  const effectiveCompact = isMobile || compact;

  useEffect(() => {
    localStorage.setItem('tgadmin-sidebar-compact', String(compact));
  }, [compact]);

  const isAdmin = me.user.role === 'admin';
  const current = (id: string) => {
    switch (id) {
      case 'clients':
        return location.pathname === '/' || location.pathname.startsWith('/users');
      case 'audit':
        return location.pathname.startsWith('/audit');
      case 'admins':
        return location.pathname.startsWith('/admins');
      case 'services':
        return location.pathname.startsWith('/services');
      case 'settings':
        return location.pathname.startsWith('/settings');
      case 'diagnostics':
        return location.pathname.startsWith('/diagnostics');
      default:
        return false;
    }
  };

  const menuItems = [
    { id: 'clients', title: 'Клиенты', icon: Persons },
    { id: 'audit', title: 'Аудит', icon: FileText },
    ...(isAdmin
      ? [
          { id: 'admins', title: 'Пользователи UI', icon: ShieldCheck },
          { id: 'services', title: 'Сервисные аккаунты', icon: Key },
          { id: 'settings', title: 'Настройки', icon: Gear },
          { id: 'diagnostics', title: 'Диагностика', icon: Stethoscope },
        ]
      : []),
  ].map((item) => ({
    ...item,
    current: current(item.id),
    onItemClick: () => navigate(MENU_PATHS[item.id] ?? '/'),
  }));

  return (
    <>
      <AsideHeader
        logo={{
          text: 'TeleMT Admin',
          icon: Server,
          onClick: () => navigate('/'),
        }}
        compact={effectiveCompact}
        onChangeCompact={(next) => !isMobile && setCompact(next)}
        menuItems={menuItems}
        menuDensity="compact"
        menuOverflow="scroll"
        aboveMenuContent={
          effectiveCompact ? null : (
            <div style={{ padding: '8px 12px' }}>
              <ThemeSelect theme={theme} onThemeChange={onThemeChange} />
            </div>
          )
        }
        renderFooter={() => (
          <>
            <FooterItem
              id="account"
              title={me.user.name}
              icon={Person}
              compact={effectiveCompact}
              onItemClick={() => navigate('/account')}
            />
            {effectiveCompact && (
              <FooterItem
                id="theme"
                title="Тема"
                icon={Sun}
                compact={effectiveCompact}
                onItemClick={() => setThemeDialogOpen(true)}
              />
            )}
            <FooterItem
              id="logout"
              title="Выйти"
              icon={ArrowRightFromSquare}
              compact={effectiveCompact}
              onItemClick={onLogout}
            />
          </>
        )}
        renderContent={() => (
          <main className="page">
            {me.require2fa?.enabled && !me.require2fa.enforced && (
              <Alert
                theme="warning"
                message={
                  me.require2fa.daysLeft > 0
                    ? `2FA не включена. Через ${me.require2fa.daysLeft} дн. доступ к интерфейсу будет ограничен до её включения. Настройте её в разделе «Аккаунт».`
                    : '2FA не включена. Сегодня истекает льготный срок, после чего доступ будет ограничен. Настройте её в разделе «Аккаунт».'
                }
              />
            )}
            {children}
            <Footer />
          </main>
        )}
      />
      <ThemeDialog
        open={themeDialogOpen}
        theme={theme}
        onThemeChange={onThemeChange}
        onClose={() => setThemeDialogOpen(false)}
      />
    </>
  );
}
