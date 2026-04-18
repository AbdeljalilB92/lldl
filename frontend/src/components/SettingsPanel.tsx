import { useState } from "react";
import { wailsBinding } from "../hooks/useWailsBinding";

interface Props {
  onClose: () => void;
}

export default function SettingsPanel({ onClose }: Props): JSX.Element {
  const [resetting, setResetting] = useState(false);
  const [resetError, setResetError] = useState<string | null>(null);
  const [resetDone, setResetDone] = useState(false);

  async function handleReset(): Promise<void> {
    setResetting(true);
    setResetError(null);
    setResetDone(false);

    try {
      // Save empty config to effectively reset.
      await wailsBinding.saveConfig({
        token: "",
        quality: "720",
        outputDir: "",
        courseUrl: "",
      });
      setResetDone(true);
    } catch (err) {
      setResetError(err instanceof Error ? err.message : String(err));
    } finally {
      setResetting(false);
    }
  }

  return (
    <div className="settings-panel">
      <div className="settings-panel__header">
        <h2 className="settings-panel__title">Settings</h2>
        <button className="settings-panel__close-btn" onClick={onClose}>
          {"\u2715"}
        </button>
      </div>

      <div className="settings-panel__section">
        <h3 className="settings-panel__section-title">Configuration</h3>
        <p className="settings-panel__hint">
          Settings are saved per-session via the config form. Use the button below to clear saved values.
        </p>
      </div>

      <div className="settings-panel__section">
        <h3 className="settings-panel__section-title">Reset</h3>
        <p className="settings-panel__hint">
          Clear all saved configuration values. You will need to re-enter your token and preferences.
        </p>
        <button
          className="settings-panel__reset-btn"
          onClick={handleReset}
          disabled={resetting}
        >
          {resetting ? "Resetting..." : "Reset Config"}
        </button>
        {resetDone && (
          <p className="settings-panel__success">Configuration cleared.</p>
        )}
        {resetError && (
          <p className="settings-panel__error">{resetError}</p>
        )}
      </div>
    </div>
  );
}
