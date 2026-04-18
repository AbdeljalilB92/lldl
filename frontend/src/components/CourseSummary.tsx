import { useState } from "react";
import type { CourseResponse } from "../hooks/useWailsBinding";

interface Props {
  course: CourseResponse;
  onConfirm: () => void;
}

export default function CourseSummary({ course, onConfirm }: Props): JSX.Element {
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});

  function toggleChapter(slug: string) {
    setExpanded((prev) => ({ ...prev, [slug]: !prev[slug] }));
  }

  function formatDuration(seconds: number): string {
    const m = Math.floor(seconds / 60);
    const s = seconds % 60;
    return `${m}:${s.toString().padStart(2, "0")}`;
  }

  return (
    <div className="course-summary">
      <h2 className="course-summary__title">{course.title}</h2>

      <div className="course-summary__stats">
        <span>{course.chapterCount} chapters</span>
        <span>{course.videoCount} videos</span>
        {course.hasExercises && <span>Includes exercise files</span>}
      </div>

      <div className="course-summary__tree">
        {course.chapters.map((ch) => (
          <div key={ch.slug} className="course-summary__chapter">
            <button
              className="course-summary__chapter-toggle"
              onClick={() => toggleChapter(ch.slug)}
            >
              <span className="course-summary__chevron">
                {expanded[ch.slug] ? "\u25BC" : "\u25B6"}
              </span>
              <span>{ch.title}</span>
              <span className="course-summary__chapter-count">{ch.videos.length}</span>
            </button>

            {expanded[ch.slug] && (
              <ul className="course-summary__video-list">
                {ch.videos.map((vid) => (
                  <li key={vid.slug} className="course-summary__video">
                    <span className="course-summary__video-icon">{"\u25CB"}</span>
                    <span className="course-summary__video-title">{vid.title}</span>
                    {vid.duration > 0 && (
                      <span className="course-summary__video-duration">
                        {formatDuration(vid.duration)}
                      </span>
                    )}
                  </li>
                ))}
              </ul>
            )}
          </div>
        ))}
      </div>

      {course.hasExercises && (
        <div className="course-summary__exercises">
          {"\uD83D\uDCC2"} Exercise files will be downloaded
        </div>
      )}

      <button className="course-summary__confirm-btn" onClick={onConfirm}>
        Confirm &amp; Resolve URLs
      </button>
    </div>
  );
}
