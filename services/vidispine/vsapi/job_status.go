package vsapi

import (
	"github.com/bcc-code/bcc-media-flows/internal/enumjson"
	"github.com/orsinium-labs/enum"
)

// JobStatus is the state Vidispine reports for a job. The members are the
// states Vidispine documents; a state it adds later still decodes and simply
// compares unequal to every named member, so it is treated as terminal.
type JobStatus enum.Member[string]

var (
	JobStatusNone            = JobStatus{Value: "NONE"}
	JobStatusReady           = JobStatus{Value: "READY"}
	JobStatusStarted         = JobStatus{Value: "STARTED"}
	JobStatusWaiting         = JobStatus{Value: "WAITING"}
	JobStatusFinished        = JobStatus{Value: "FINISHED"}
	JobStatusFinishedWarning = JobStatus{Value: "FINISHED_WARNING"}
	JobStatusFailedTotal     = JobStatus{Value: "FAILED_TOTAL"}
	JobStatusAbortedPending  = JobStatus{Value: "ABORTED_PENDING"}
	JobStatusAborted         = JobStatus{Value: "ABORTED"}
	JobStatusDisappeared     = JobStatus{Value: "DISAPPEARED"}
	JobStatusVidinetJob      = JobStatus{Value: "VIDINET_JOB"}
	JobStatuses              = enum.New(
		JobStatusNone,
		JobStatusReady,
		JobStatusStarted,
		JobStatusWaiting,
		JobStatusFinished,
		JobStatusFinishedWarning,
		JobStatusFailedTotal,
		JobStatusAbortedPending,
		JobStatusAborted,
		JobStatusDisappeared,
		JobStatusVidinetJob,
	)
)

// InProgress reports whether Vidispine is still working on the job, i.e. it
// is queued, waiting or running. Every other state is final.
func (s JobStatus) InProgress() bool {
	return s == JobStatusStarted || s == JobStatusReady || s == JobStatusWaiting
}

func (s JobStatus) String() string {
	return s.Value
}

//goland:noinspection GoMixedReceiverTypes
func (s JobStatus) MarshalJSON() ([]byte, error) {
	return enumjson.Marshal(s)
}

//goland:noinspection GoMixedReceiverTypes
func (s *JobStatus) UnmarshalJSON(data []byte) error {
	return enumjson.UnmarshalOpen(data, s)
}
