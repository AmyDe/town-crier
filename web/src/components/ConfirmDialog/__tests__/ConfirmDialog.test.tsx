import { fireEvent, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, vi } from 'vitest';
import { ConfirmDialog } from '../ConfirmDialog';

describe('ConfirmDialog', () => {
  it('renders the title and message when open', () => {
    render(
      <ConfirmDialog
        open={true}
        title="Delete watch zone"
        message="Are you sure you want to delete this watch zone? This cannot be undone."
        confirmLabel="Delete"
        onConfirm={() => {}}
        onCancel={() => {}}
      />,
    );

    expect(screen.getByText('Delete watch zone')).toBeInTheDocument();
    expect(
      screen.getByText(
        'Are you sure you want to delete this watch zone? This cannot be undone.',
      ),
    ).toBeInTheDocument();
  });

  it('calls onConfirm when confirm button is clicked', async () => {
    const user = userEvent.setup();
    const onConfirm = vi.fn();

    render(
      <ConfirmDialog
        open={true}
        title="Delete"
        message="Are you sure?"
        confirmLabel="Delete"
        onConfirm={onConfirm}
        onCancel={() => {}}
      />,
    );

    await user.click(screen.getByRole('button', { name: 'Delete' }));
    expect(onConfirm).toHaveBeenCalledOnce();
  });

  it('calls onCancel when cancel button is clicked', async () => {
    const user = userEvent.setup();
    const onCancel = vi.fn();

    render(
      <ConfirmDialog
        open={true}
        title="Delete"
        message="Are you sure?"
        confirmLabel="Delete"
        onConfirm={() => {}}
        onCancel={onCancel}
      />,
    );

    await user.click(screen.getByRole('button', { name: /cancel/i }));
    expect(onCancel).toHaveBeenCalledOnce();
  });

  it('does not render content when closed', () => {
    render(
      <ConfirmDialog
        open={false}
        title="Delete"
        message="Are you sure?"
        confirmLabel="Delete"
        onConfirm={() => {}}
        onCancel={() => {}}
      />,
    );

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it('renders the dialog with an accessible role', () => {
    render(
      <ConfirmDialog
        open={true}
        title="Confirm"
        message="Proceed?"
        confirmLabel="Yes"
        onConfirm={() => {}}
        onCancel={() => {}}
      />,
    );

    expect(screen.getByRole('dialog')).toBeInTheDocument();
  });

  describe('modal behaviour', () => {
    function renderDialog(open: boolean, onCancel: () => void = () => {}) {
      const props = {
        title: 'Delete',
        message: 'Are you sure?',
        confirmLabel: 'Confirm',
        onConfirm: () => {},
        onCancel,
      };
      return {
        ui: (isOpen: boolean) => <ConfirmDialog open={isOpen} {...props} />,
        initial: <ConfirmDialog open={open} {...props} />,
      };
    }

    it('opens as a native modal dialog', () => {
      const { initial } = renderDialog(true);
      render(initial);

      const dialog = screen.getByRole('dialog');
      expect(dialog.tagName).toBe('DIALOG');
      expect(dialog).toHaveAttribute('open');
      expect(dialog).toHaveAttribute('aria-modal', 'true');
    });

    it('calls onCancel on the dialog cancel event (Escape) and stays open until the parent closes it', () => {
      const onCancel = vi.fn();
      const { initial } = renderDialog(true, onCancel);
      render(initial);

      const dialog = screen.getByRole('dialog');
      const event = new Event('cancel', { cancelable: true });
      dialog.dispatchEvent(event);

      expect(onCancel).toHaveBeenCalledOnce();
      expect(event.defaultPrevented).toBe(true);
      expect(screen.getByRole('dialog')).toBeInTheDocument();
    });

    it('moves focus to the Cancel button on open', () => {
      const { initial } = renderDialog(true);
      render(initial);

      expect(screen.getByRole('button', { name: /cancel/i })).toHaveFocus();
    });

    it('restores focus to the previously focused element on close', () => {
      const { ui } = renderDialog(false);
      render(<button>Trigger</button>);
      const trigger = screen.getByRole('button', { name: 'Trigger' });
      trigger.focus();

      const { rerender } = render(ui(false));
      rerender(ui(true));
      expect(screen.getByRole('button', { name: /cancel/i })).toHaveFocus();

      rerender(ui(false));
      expect(trigger).toHaveFocus();
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });

    it('does not call onCancel for other events', () => {
      const onCancel = vi.fn();
      const { initial } = renderDialog(true, onCancel);
      render(initial);

      fireEvent.keyDown(screen.getByRole('dialog'), { key: 'a' });
      expect(onCancel).not.toHaveBeenCalled();
    });
  });
});
