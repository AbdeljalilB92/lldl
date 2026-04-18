interface Props {
  loading: boolean;
  success: boolean;
  error: string | null;
  onRetry: () => void;
}

export default function AuthStatus({ loading, success, error, onRetry }: Props): JSX.Element {
  if (loading) {
    return (
      <div className="auth-status auth-status--loading">
        <div className="auth-status__spinner" />
        <p>Authenticating with LinkedIn...</p>
      </div>
    );
  }

  if (success) {
    return (
      <div className="auth-status auth-status--success">
        <span className="auth-status__icon">{"\u2713"}</span>
        <p>Authentication successful</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="auth-status auth-status--error">
        <span className="auth-status__icon">{"\u2717"}</span>
        <p className="auth-status__error-msg">{error}</p>
        <button className="auth-status__retry-btn" onClick={onRetry}>
          Back to Settings
        </button>
      </div>
    );
  }

  return <div />;
}
