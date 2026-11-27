import { useState } from 'react';
import { Alert, Button, Card, Text, TextInput } from '@gravity-ui/uikit';
import { api, ApiError, type Me } from '../api';
import Footer from '../Footer';

export default function LoginPage({ onLogin }: { onLogin: (me: Me) => void }) {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [step, setStep] = useState<'password' | 'totp'>('password');
  const [code, setCode] = useState('');
  const [recovery, setRecovery] = useState(false);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  const submitPassword = async (event: React.FormEvent) => {
    event.preventDefault();
    setBusy(true);
    setError('');
    try {
      const result = await api.login(username, password);
      if ('totpRequired' in result) {
        setStep('totp');
        setCode('');
      } else {
        onLogin(result);
      }
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'не удалось войти');
    } finally {
      setBusy(false);
    }
  };

  const submitCode = async (event: React.FormEvent) => {
    event.preventDefault();
    setBusy(true);
    setError('');
    try {
      onLogin(await api.loginTotp(code));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'не удалось проверить код');
      setBusy(false);
    }
  };

  return (
    <div className="app-shell">
      <main className="page" style={{ maxWidth: 420, paddingTop: 80, width: '100%' }}>
        <Card view="filled" style={{ padding: 24 }}>
          {step === 'password' ? (
            <form className="stack" onSubmit={submitPassword}>
              <Text variant="header-2">Вход в TeleMT Admin</Text>
              {error && <Alert theme="danger" message={error} onClose={() => setError('')} />}
              <TextInput
                placeholder="Имя"
                value={username}
                onUpdate={setUsername}
                autoComplete="username"
                controlProps={{ 'aria-label': 'Имя пользователя' }}
              />
              <TextInput
                placeholder="Пароль"
                type="password"
                value={password}
                onUpdate={setPassword}
                autoComplete="current-password"
                controlProps={{ 'aria-label': 'Пароль' }}
              />
              <Button view="action" type="submit" loading={busy} disabled={!username || !password}>
                Войти
              </Button>
            </form>
          ) : (
            <form className="stack" onSubmit={submitCode}>
              <Text variant="header-2">Второй фактор</Text>
              <Text className="muted">
                {recovery
                  ? 'Введите один из резервных кодов. Каждый работает один раз.'
                  : 'Введите шестизначный код из приложения-аутентификатора.'}
              </Text>
              {error && <Alert theme="danger" message={error} onClose={() => setError('')} />}
              <TextInput
                placeholder={recovery ? 'XXXX-XXXX' : '000000'}
                value={code}
                onUpdate={setCode}
                autoComplete="one-time-code"
                controlProps={{ 'aria-label': recovery ? 'Резервный код' : 'Код TOTP' }}
                autoFocus
              />
              <Button view="action" type="submit" loading={busy} disabled={code.trim().length < 6}>
                Подтвердить
              </Button>
              <div className="row" style={{ justifyContent: 'space-between' }}>
                <Button
                  view="flat"
                  type="button"
                  onClick={() => {
                    setRecovery(!recovery);
                    setCode('');
                    setError('');
                  }}
                >
                  {recovery ? 'Код из приложения' : 'Использовать резервный код'}
                </Button>
                <Button
                  view="flat"
                  type="button"
                  onClick={() => {
                    setStep('password');
                    setCode('');
                    setRecovery(false);
                    setError('');
                  }}
                >
                  Назад
                </Button>
              </div>
            </form>
          )}
        </Card>
      </main>
      <Footer />
    </div>
  );
}
