/**
 * frontend/src/components/TransitionOverlay.tsx
 */
interface Props {
  message: string;
}

export function TransitionOverlay({ message }: Props) {
  return (
    <div class="transition-overlay" role="status" aria-live="polite" aria-label={message}>
      <div class="transition-card">
        <div class="spinner" />
        <div class="transition-text">{message}</div>
      </div>
    </div>
  );
}
