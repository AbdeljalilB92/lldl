package download

import "context"

// Engine defines the contract for executing a batch of download jobs.
// Implementations may use worker pools, sequential downloads, or any other strategy.
//
// The app layer translates domain objects into []Job before calling DownloadAll,
// so this feature never imports from other feature modules.
type Engine interface {
	// DownloadAll executes all jobs and returns one Result per job.
	// Results are guaranteed to be in the same order as the input jobs.
	// Context cancellation triggers graceful shutdown of in-flight downloads.
	DownloadAll(ctx context.Context, jobs []Job) []Result
}
