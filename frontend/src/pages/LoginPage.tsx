import { useState } from 'react';
import { Alert, Button, Card, Text, TextInput } from '@gravity-ui/uikit';
import { api, ApiError, type Me } from '../api';

export default function LoginPage({ onLogin }: { onLogin: (me: Me) => void }) {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    setBusy(true);
    setError('');
    try {
      onLogin(await api.login(username, password));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'не удалось войти');
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="page" style={{ maxWidth: 420, paddingTop: 80 }}>
      <Card view="filled" style={{ padding: 24 }}>
        <form className="stack" onSubmit={submit}>
          <Text variant="header-2">Вход в TeleMT Admin</Text>
          {error && <Alert theme="danger" message={error} onClose={() => setError('')} />}
          <TextInput
            placeholder="Имя"
            value={username}
            onUpdate={setUsername}
            autoComplete="username"
          />
          <TextInput
            placeholder="Пароль"
            type="password"
            value={password}
            onUpdate={setPassword}
            autoComplete="current-password"
          />
          <Button view="action" type="submit" loading={busy} disabled={!username || !password}>
            Войти
          </Button>
        </form>
      </Card>
    </div>
  );
}
