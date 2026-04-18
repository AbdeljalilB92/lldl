import { useEffect, useState } from "react";
import { useWailsEvents } from "../hooks/useWailsEvents";
import type { DownloadProgressEvent, DownloadCompleteResponse, DownloadStartResponse } from "../hooks/useWailsBinding";

interface Props {
  onCancel: () => void;
  onComplete: (summary: DownloadCompleteResponse) => void;
}

interface FileStatus {
  jobId: string;
  fileName: string;
  status: string;
  error?: string;
}

export default function DownloadProgress({ onCancel, onComplete }: Props): JSX.Element {
  const [files, setFiles] = useState<FileStatus[]>([]);
  const [totalJobs, setTotalJobs] = useState<number>(0);
  const [completed, setCompleted] = useState(false);

  const events = useWailsEvents<Record<string, DownloadProgressEvent | DownloadCompleteResponse | DownloadStartResponse>>(
    ["download:start", "download:progress", "download:complete"],
  );

  const startEvt = events["download:start"] as DownloadStartResponse | undefined;
  const progressEvt = events["download:progress"] as DownloadProgressEvent | undefined;
  const completeEvt = events["download:complete"] as DownloadCompleteResponse | undefined;

  // Initialize total job count from the start event.
  useEffect(() => {
    if (startEvt && startEvt.totalJobs > 0) {
      setTotalJobs(startEvt.totalJobs);
    }
  }, [startEvt]);

  // Track individual file statuses as events come in.
  useEffect(() => {
    if (progressEvt) {
      setFiles((prev) => {
        const idx = prev.findIndex((f) => f.jobId === progressEvt.jobId);
        if (idx >= 0) {
          const updated = [...prev];
          updated[idx] = {
            jobId: progressEvt.jobId,
            fileName: progressEvt.fileName,
            status: progressEvt.status,
            error: progressEvt.error,
          };
          return updated;
        }
        return [
          ...prev,
          {
            jobId: progressEvt.jobId,
            fileName: progressEvt.fileName,
            status: progressEvt.status,
            error: progressEvt.error,
          },
        ];
      });
    }
  }, [progressEvt]);

  useEffect(() => {
    if (completeEvt && !completed) {
      setCompleted(true);
      onComplete(completeEvt);
    }
  }, [completeEvt, completed, onComplete]);

  const succeeded = files.filter((f) => f.status === "succeeded").length;
  const failed = files.filter((f) => f.status === "failed").length;
  const skipped = files.filter((f) => f.status === "skipped").length;
  const total = totalJobs > 0 ? totalJobs : files.length;
  const pct = total > 0 ? Math.round(((succeeded + failed + skipped) / total) * 100) : 0;

  function statusIcon(status: string): string {
    switch (status) {
      case "succeeded":
        return "\u2713";
      case "failed":
        return "\u2717";
      case "skipped":
        return "\u2014";
      default:
        return "\u25B6";
    }
  }

  function statusClass(status: string): string {
    switch (status) {
      case "succeeded":
        return "download-progress__file--success";
      case "failed":
        return "download-progress__file--failed";
      case "skipped":
        return "download-progress__file--skipped";
      default:
        return "download-progress__file--active";
    }
  }

  return (
    <div className="download-progress">
      <h2 className="download-progress__title">Downloading Files</h2>

      <div className="download-progress__bar-container">
        <div
          className="download-progress__bar-fill"
          style={{ width: `${pct}%` }}
          role="progressbar"
          aria-valuenow={pct}
          aria-valuemin={0}
          aria-valuemax={100}
          aria-label="Download progress"
        />
      </div>

      <div className="download-progress__stats">
        <span>{succeeded} done</span>
        {failed > 0 && <span className="download-progress__stat-error">{failed} failed</span>}
        {skipped > 0 && <span>{skipped} skipped</span>}
        <span>{pct}%</span>
      </div>

      <div className="download-progress__file-list">
        {files.map((f) => (
          <div key={f.jobId} className={`download-progress__file ${statusClass(f.status)}`}>
            <span className="download-progress__file-icon">{statusIcon(f.status)}</span>
            <span className="download-progress__file-name">{f.fileName}</span>
            {f.error && <span className="download-progress__file-error">{f.error}</span>}
          </div>
        ))}
      </div>

      {!completed && (
        <button className="download-progress__cancel-btn" onClick={onCancel}>
          Cancel
        </button>
      )}
    </div>
  );
}
