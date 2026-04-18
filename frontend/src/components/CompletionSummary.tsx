import type { DownloadCompleteResponse } from "../hooks/useWailsBinding";

interface Props {
  summary: DownloadCompleteResponse;
  onReset: () => void;
}

export default function CompletionSummary({ summary, onReset }: Props): JSX.Element {
  const hasFailures = summary.failed > 0;

  return (
    <div className="completion-summary">
      <h2 className="completion-summary__title">
        {hasFailures ? "Download Completed with Errors" : "Download Complete"}
      </h2>

      <div className="completion-summary__stats">
        <div className="completion-summary__stat completion-summary__stat--success">
          <span className="completion-summary__stat-value">{summary.succeeded}</span>
          <span className="completion-summary__stat-label">Succeeded</span>
        </div>

        <div className="completion-summary__stat completion-summary__stat--failed">
          <span className="completion-summary__stat-value">{summary.failed}</span>
          <span className="completion-summary__stat-label">Failed</span>
        </div>

        <div className="completion-summary__stat completion-summary__stat--skipped">
          <span className="completion-summary__stat-value">{summary.skipped}</span>
          <span className="completion-summary__stat-label">Skipped</span>
        </div>
      </div>

      {summary.error && (
        <div className="completion-summary__error">{summary.error}</div>
      )}

      <button className="completion-summary__reset-btn" onClick={onReset}>
        Download Another Course
      </button>
    </div>
  );
}
