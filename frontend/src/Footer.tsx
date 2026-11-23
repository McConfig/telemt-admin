import { useEffect, useState } from 'react';
import { api, type Meta } from './api';

export default function Footer() {
  const [meta, setMeta] = useState<Meta | null>(null);
  useEffect(() => {
    api.meta().then(setMeta).catch(() => setMeta(null));
  }, []);
  return (
    <footer className="footer">
      <span>
        TeleMT admin by{' '}
        <a href="https://github.com/McConfig/telemt-admin" target="_blank" rel="noreferrer">
          McConfig (c)
        </a>
      </span>
      <span className="muted">{meta ? meta.version : ''}</span>
    </footer>
  );
}
