import { useEffect, useState } from "react";
import { wailsBinding, type ConfigResponse } from "../hooks/useWailsBinding";

const QUALITY_OPTIONS = [
  { value: "720p", label: "720p High" },
  { value: "540p", label: "540p Medium" },
  { value: "360p", label: "360p Low" },
];

interface Props {
  onStart: (config: {
    token: string;
    quality: string;
    outputDir: string;
    courseURL: string;
  }) => void;
}

export default function ConfigForm({ onStart }: Props): JSX.Element {
  const [token, setToken] = useState("");
  const [quality, setQuality] = useState("720p");
  const [outputDir, setOutputDir] = useState("");
  const [courseURL, setCourseURL] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    wailsBinding
      .loadConfig()
      .then((cfg: ConfigResponse) => {
        if (cfg.found) {
          setToken(cfg.token || "");
          setQuality(cfg.quality || "720p");
          setOutputDir(cfg.outputDir || "");
          setCourseURL(cfg.courseUrl || "");
        }
      })
      .catch(() => {
        // Config not found is fine — form stays empty.
      })
      .finally(() => setLoading(false));
  }, []);

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);

    if (!token.trim()) {
      setError("LinkedIn token is required.");
      return;
    }
    if (!courseURL.trim()) {
      setError("Course URL is required.");
      return;
    }
    if (!outputDir.trim()) {
      setError("Output directory is required.");
      return;
    }

    onStart({ token: token.trim(), quality, outputDir: outputDir.trim(), courseURL: courseURL.trim() });
  }

  if (loading) {
    return <div className="config-form__loading">Loading settings...</div>;
  }

  return (
    <form className="config-form" onSubmit={handleSubmit}>
      <h2 className="config-form__title">Download Settings</h2>

      <label className="config-form__field">
        <span className="config-form__label">LinkedIn li_at Token</span>
        <input
          type="password"
          className="config-form__input"
          value={token}
          onChange={(e) => setToken(e.target.value)}
          placeholder="Paste your li_at cookie value"
          autoComplete="off"
        />
      </label>

      <label className="config-form__field">
        <span className="config-form__label">Video Quality</span>
        <select
          className="config-form__select"
          value={quality}
          onChange={(e) => setQuality(e.target.value)}
        >
          {QUALITY_OPTIONS.map((opt) => (
            <option key={opt.value} value={opt.value}>
              {opt.label}
            </option>
          ))}
        </select>
      </label>

      <label className="config-form__field">
        <span className="config-form__label">Output Directory</span>
        <input
          type="text"
          className="config-form__input"
          value={outputDir}
          onChange={(e) => setOutputDir(e.target.value)}
          placeholder="/path/to/downloads"
        />
      </label>

      <label className="config-form__field">
        <span className="config-form__label">Course URL</span>
        <input
          type="text"
          className="config-form__input"
          value={courseURL}
          onChange={(e) => setCourseURL(e.target.value)}
          placeholder="https://www.linkedin.com/learning/..."
        />
      </label>

      {error && <div className="config-form__error">{error}</div>}

      <button type="submit" className="config-form__submit-btn">
        Start
      </button>
    </form>
  );
}
