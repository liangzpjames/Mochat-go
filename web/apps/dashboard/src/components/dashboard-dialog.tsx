import { Button, Drawer, Modal } from 'antd';
import { useEffect, useRef } from 'react';
import type { RefObject, ReactNode } from 'react';

type DashboardDialogProps = {
  open: boolean;
  title: ReactNode;
  children: ReactNode;
  onCancel: () => void;
  onConfirm?: () => void;
  confirmText?: string;
  cancelText?: string;
  confirmLoading?: boolean;
  confirmDisabled?: boolean;
  danger?: boolean;
  mode?: 'modal' | 'drawer';
  width?: number | string;
  triggerRef?: RefObject<HTMLElement | null>;
  footer?: ReactNode | null;
};

export function DashboardDialog({
  open,
  title,
  children,
  onCancel,
  onConfirm,
  confirmText = '保存',
  cancelText = '取消',
  confirmLoading = false,
  confirmDisabled = false,
  danger = false,
  mode = 'modal',
  width,
  triggerRef,
  footer,
}: DashboardDialogProps) {
  const wasOpen = useRef(open);
  useEffect(() => {
    if (!open) return undefined;
    const timer = window.setTimeout(() => {
      const dialogs = document.querySelectorAll<HTMLElement>('.dashboard-dialog');
      const dialog = dialogs.item(dialogs.length - 1);
      const firstField = dialog?.querySelector<HTMLElement>(
        '[autofocus], input:not([disabled]), select:not([disabled]), textarea:not([disabled])',
      );
      (firstField ?? dialog)?.focus();
    }, 0);
    return () => window.clearTimeout(timer);
  }, [open]);
  useEffect(() => {
    if (wasOpen.current && !open) queueMicrotask(() => triggerRef?.current?.focus());
    wasOpen.current = open;
  }, [open, triggerRef]);

  function handleCancel() {
    onCancel();
    queueMicrotask(() => triggerRef?.current?.focus());
  }

  const resolvedFooter = footer === undefined
    ? (
        <>
          <Button aria-label={cancelText} onClick={handleCancel}>{cancelText}</Button>
          {onConfirm !== undefined && (
            <Button aria-label={confirmText} danger={danger} disabled={confirmDisabled} loading={confirmLoading} type="primary" onClick={onConfirm}>{confirmText}</Button>
          )}
        </>
      )
    : footer;
  const widthProps = width === undefined ? {} : { width };

  if (mode === 'drawer') {
    return (
      <Drawer
        className="dashboard-dialog dashboard-dialog--drawer"
        open={open}
        title={title}
        {...widthProps}
        onClose={handleCancel}
        extra={resolvedFooter}
      >
        {children}
      </Drawer>
    );
  }

  return (
    <Modal
      className="dashboard-dialog dashboard-dialog--modal"
      footer={resolvedFooter}
      open={open}
      title={title}
      {...widthProps}
      onCancel={handleCancel}
    >
      {children}
    </Modal>
  );
}
