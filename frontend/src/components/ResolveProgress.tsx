import { useWailsEvent } from "../hooks/useWailsEvents";
import type { ResolveProgress } from "../hooks/useWailsBinding";

interface Props {
  error: string | null;
  onComplete: () => void;
}

export default function ResolveProgress({ error, onComplete }: Props): JSX.Element {
  const progress = useWailsEvent<ResolveProgress>("resolve:progress");

  const current = progress?.current ?? 0;
  const total = progress?.total ?? 0;
  const pct = total > 0 ? Math.round((current / total) * 100) : 0;

  // Count errors from the events we have seen.
  const hasError = progress?.error && progress.error.length > 0;

  return (
    <div className="resolve-progress">
      <h2 className="resolve-progress__title">Resolving Video URLs</h2>

      <div className="resolve-progress__bar-container">
        <div
          className="resolve-progress__bar-fill"
          style={{ width: `${pct}%` }}
        />
      </div>

      <div className="resolve-progress__stats">
        <span>{current} / {total} videos</span>
        <span>{pct}%</span>
      </div>

      {progress?.title && (
        <div className="resolve-progress__current">
          {hasError ? (
            <span className="resolve-progress__error">
              {"\u2717"} {progress.title} — {progress.error}
            </span>
          ) : (
            <span>{"\u25B6"} {progress.title}</span>
          )}
        </div>
      )}

      {error && (
        <div className="resolve-progress__fatal-error">
          Resolution failed: {error}
        </div>
      )}

      {!error && pct >= 100 && (
        <button className="resolve-progress__next-btn" onClick={onComplete}>
          Continue to Download
        </button>
      )}
    </div>
  );
}
