const STEPS = [
  { key: "config", label: "Settings" },
  { key: "auth", label: "Auth" },
  { key: "course", label: "Course" },
  { key: "resolve", label: "Resolve" },
  { key: "download", label: "Download" },
  { key: "complete", label: "Complete" },
] as const;

export type StepKey = (typeof STEPS)[number]["key"];

interface Props {
  currentStep: StepKey;
}

export default function StepIndicator({ currentStep }: Props): JSX.Element {
  const currentIndex = STEPS.findIndex((s) => s.key === currentStep);

  return (
    <div className="step-indicator">
      {STEPS.map((step, i) => {
        const isCompleted = i < currentIndex;
        const isCurrent = i === currentIndex;
        let cls = "step-indicator__item";
        if (isCompleted) cls += " step-indicator__item--completed";
        if (isCurrent) cls += " step-indicator__item--current";

        return (
          <div key={step.key} className={cls}>
            <span className="step-indicator__dot">
              {isCompleted ? "\u2713" : i + 1}
            </span>
            <span className="step-indicator__label">{step.label}</span>
            {i < STEPS.length - 1 && <span className="step-indicator__line" />}
          </div>
        );
      })}
    </div>
  );
}
