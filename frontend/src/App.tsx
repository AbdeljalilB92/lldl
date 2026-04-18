import { useState, useCallback, useRef, useEffect } from "react";
import StepIndicator, { type StepKey } from "./components/StepIndicator";
import ConfigForm from "./components/ConfigForm";
import AuthStatus from "./components/AuthStatus";
import CourseSummary from "./components/CourseSummary";
import ResolveProgress from "./components/ResolveProgress";
import DownloadProgress from "./components/DownloadProgress";
import CompletionSummary from "./components/CompletionSummary";
import ErrorBoundary from "./components/ErrorBoundary";
import SettingsPanel from "./components/SettingsPanel";
import { wailsBinding, type CourseResponse, type DownloadCompleteResponse } from "./hooks/useWailsBinding";
import "./App.css";

interface AppConfig {
  token: string;
  quality: string;
  outputDir: string;
  courseURL: string;
}

function App(): JSX.Element {
  const [step, setStep] = useState<StepKey>("config");
  const [showSettings, setShowSettings] = useState(false);
  const mountedRef = useRef(true);

  useEffect(() => {
    return () => {
      mountedRef.current = false;
    };
  }, []);

  // Auth state
  const [authLoading, setAuthLoading] = useState(false);
  const [authSuccess, setAuthSuccess] = useState(false);
  const [authError, setAuthError] = useState<string | null>(null);

  // Course state
  const [course, setCourse] = useState<CourseResponse | null>(null);
  const [courseError, setCourseError] = useState<string | null>(null);

  // Resolve state
  const [resolveError, setResolveError] = useState<string | null>(null);

  // Download complete state
  const [downloadSummary, setDownloadSummary] = useState<DownloadCompleteResponse | null>(null);

  // --- Step transitions ---

  async function handleStart(formData: AppConfig) {
    setAuthLoading(true);
    setAuthError(null);
    setStep("auth");

    try {
      // Persist quality and output dir on the backend before authenticate
      // because Authenticate constructs resolvers that depend on quality.
      await wailsBinding.setQuality(formData.quality);
      await wailsBinding.setOutputDir(formData.outputDir);
      await wailsBinding.saveConfig({
        token: formData.token,
        quality: formData.quality,
        outputDir: formData.outputDir,
        courseUrl: formData.courseURL,
      });

      const authResp = await wailsBinding.authenticate(formData.token);
      if (!authResp.success) {
        setAuthError(authResp.error || "Authentication failed");
        setAuthLoading(false);
        return;
      }

      setAuthSuccess(true);
      setAuthLoading(false);

      // Auto-advance to course fetch after brief delay so user sees success.
      setTimeout(async () => {
        if (!mountedRef.current) return;
        setStep("course");
        try {
          const courseResp = await wailsBinding.fetchCourse(formData.courseURL);
          if (!mountedRef.current) return;
          if (courseResp.error) {
            setCourseError(courseResp.error);
            return;
          }
          setCourse(courseResp);
        } catch (err) {
          if (!mountedRef.current) return;
          setCourseError(err instanceof Error ? err.message : String(err));
        }
      }, 800);
    } catch (err) {
      setAuthError(err instanceof Error ? err.message : String(err));
      setAuthLoading(false);
    }
  }

  function handleAuthRetry() {
    setAuthError(null);
    setAuthSuccess(false);
    setStep("config");
  }

  async function handleConfirmCourse() {
    setStep("resolve");
    setResolveError(null);

    try {
      // Resolve exercises first (quick), then videos (slower, emits progress).
      if (course?.hasExercises) {
        await wailsBinding.resolveExercises();
      }
      await wailsBinding.resolveVideos();
    } catch (err) {
      setResolveError(err instanceof Error ? err.message : String(err));
    }
  }

  function handleResolveComplete() {
    setStep("download");
    // Fire-and-forget — progress events drive the UI.
    wailsBinding.startDownload().catch(() => {
      // Error is communicated via download:complete event.
    });
  }

  const handleDownloadComplete = useCallback((summary: DownloadCompleteResponse) => {
    setDownloadSummary(summary);
    setStep("complete");
  }, []);

  async function handleCancel() {
    await wailsBinding.cancel();
  }

  function handleReset() {
    setAuthLoading(false);
    setAuthSuccess(false);
    setAuthError(null);
    setCourse(null);
    setCourseError(null);
    setResolveError(null);
    setDownloadSummary(null);
    setStep("config");
  }

  // --- Render ---

  function renderStep() {
    switch (step) {
      case "config":
        return <ConfigForm onStart={handleStart} />;

      case "auth":
        return (
          <AuthStatus
            loading={authLoading}
            success={authSuccess}
            error={authError}
            onRetry={handleAuthRetry}
          />
        );

      case "course":
        if (courseError) {
          return (
            <div className="app__error-panel">
              <p>{"\u2717"} Failed to load course</p>
              <p className="app__error-detail">{courseError}</p>
              <button className="app__back-btn" onClick={handleReset}>
                Back to Settings
              </button>
            </div>
          );
        }
        if (!course) {
          return (
            <div className="app__loading-panel">
              <div className="app__spinner" />
              <p>Fetching course information...</p>
            </div>
          );
        }
        return <CourseSummary course={course} onConfirm={handleConfirmCourse} />;

      case "resolve":
        return (
          <ResolveProgress error={resolveError} onComplete={handleResolveComplete} />
        );

      case "download":
        return (
          <DownloadProgress onCancel={handleCancel} onComplete={handleDownloadComplete} />
        );

      case "complete":
        if (!downloadSummary) {
          return <div />;
        }
        return <CompletionSummary summary={downloadSummary} onReset={handleReset} />;

      default:
        return <ConfigForm onStart={handleStart} />;
    }
  }

  return (
    <div className="app">
      <header className="app__header">
        <h1 className="app__logo">lldl</h1>
        <StepIndicator currentStep={step} />
        <button
          className="app__settings-btn"
          onClick={() => setShowSettings((v) => !v)}
          title="Settings"
        >
          {"\u2699"}
        </button>
      </header>
      {showSettings && <SettingsPanel onClose={() => setShowSettings(false)} />}
      <main className="app__content">
        <ErrorBoundary>
          {renderStep()}
        </ErrorBoundary>
      </main>
    </div>
  );
}

export default App;
