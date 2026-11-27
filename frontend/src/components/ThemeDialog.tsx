import { Dialog, Text } from '@gravity-ui/uikit';
import type { AppTheme } from '../theme';
import { ThemeSelect } from './ThemeSelect';

export function ThemeDialog({
  open,
  theme,
  onThemeChange,
  onClose,
}: {
  open: boolean;
  theme: AppTheme;
  onThemeChange: (theme: AppTheme) => void;
  onClose: () => void;
}) {
  return (
    <Dialog open={open} onOpenChange={(next) => !next && onClose()} maxWidth="s">
      <Dialog.Header caption="Тема оформления" />
      <Dialog.Body>
        <div className="stack">
          <Text className="muted">
            Выберите светлую, тёмную или контрастную тему. Режим «Как в системе» следует
            настройке операционной системы.
          </Text>
          <ThemeSelect theme={theme} onThemeChange={onThemeChange} />
        </div>
      </Dialog.Body>
      <Dialog.Footer onClickButtonCancel={onClose} textButtonCancel="Закрыть" />
    </Dialog>
  );
}
