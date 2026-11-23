import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Alert, Button, Card, Text, TextInput } from '@gravity-ui/uikit';
import { api, ApiError, type Me } from '../api';

export default function NewUserPage({ me }: { me: Me }) {
  const navigate = useNavigate();
  const [name, setName] = useState('');
  const [secret, setSecret] = useState('');
  const [maxIps, setMaxIps] = useState('');
  const [maxConns, setMaxConns] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  if (me.user.role !== 'admin') {
    return <Alert theme="warning" title="Нет прав" message="Создавать пользователей может только admin" />;
  }

  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    setBusy(true);
    setError('');
    try {
      const user = await api.create({
        name: name.trim(),
        secret: secret.trim() || undefined,
        maxUniqueIps: maxIps ? Number(maxIps) : undefined,
        maxTcpConns: maxConns ? Number(maxConns) : undefined,
      });
      navigate(`/users/${encodeURIComponent(user.username)}`);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'не удалось создать');
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="stack" style={{ maxWidth: 520 }}>
      <Text variant="header-2">Новый пользователь</Text>
      <Card view="filled" style={{ padding: 24 }}>
        <form className="stack" onSubmit={submit}>
          {error && <Alert theme="danger" message={error} onClose={() => setError('')} />}
          <TextInput placeholder="Имя" value={name} onUpdate={setName} />
          <TextInput placeholder="Секрет (32 hex, пусто — сгенерировать)" value={secret} onUpdate={setSecret} />
          <TextInput placeholder="Максимум уникальных IP" value={maxIps} onUpdate={setMaxIps} />
          <TextInput placeholder="Максимум соединений" value={maxConns} onUpdate={setMaxConns} />
          <div className="row">
            <Button view="action" type="submit" loading={busy} disabled={!name.trim()}>
              Создать
            </Button>
            <Button view="flat" onClick={() => navigate('/')}>
              Отмена
            </Button>
          </div>
        </form>
      </Card>
    </div>
  );
}
