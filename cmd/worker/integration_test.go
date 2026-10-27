//go:build integration

// Local integration tests for the transcribe queue. Each test starts a Temporal dev
// server, runs workers as separate processes built by registerWorker, and serves a stub
// transcriber from this process, so it outlives a killed worker. Nothing here touches
// production configuration.
//
//	go test -tags integration -run TestIntegration -v ./cmd/worker/
//
// Needs the temporal CLI on PATH. Takes a few minutes: the transcribe client polls
// every 10 seconds.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	enumspb "go.temporal.io/api/enums/v1"
	historypb "go.temporal.io/api/history/v1"
	taskqueuepb "go.temporal.io/api/taskqueue/v1"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/environment"
	"github.com/bcc-code/bcc-media-flows/internal/bootstrap"
	"github.com/bcc-code/bcc-media-flows/paths"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
)

const (
	helperEnv          = "FLOWS_IT_HELPER_WORKER"
	legacyHeartbeatEnv = "FLOWS_IT_LEGACY_HEARTBEAT"
	workflowQueue      = "it-workflows"
)

// TestIntegrationHelperWorker is not a test of its own. The tests re-run this binary
// with FLOWS_IT_HELPER_WORKER set to get a worker in a separate process, configured
// from the environment exactly as cmd/worker's main configures one.
func TestIntegrationHelperWorker(t *testing.T) {
	if os.Getenv(helperEnv) == "" {
		t.Skip("runs only as a helper process")
	}

	environment.Load()
	c, err := bootstrap.TemporalClient()
	if err != nil {
		log.Fatal(err)
	}

	options := newWorkerOptions(c, bootstrap.Identity(), environment.Get().ActivityCount)

	if os.Getenv(legacyHeartbeatEnv) != "" {
		w := worker.New(c, environment.GetQueue(), options)
		w.RegisterActivityWithOptions(legacyTranscribe, activity.RegisterOptions{Name: "Transcribe"})
		_ = w.Run(worker.InterruptCh())
		return
	}

	buildClients(environment.Get())
	registerWorker(c, environment.GetQueue(), options)
}

// legacyTranscribe heartbeats what Transcribe did before its details were typed, the
// string "Transcribe", and then waits to be killed
func legacyTranscribe(ctx context.Context, _ activities.TranscribeParams) (*activities.TranscribeResponse, error) {
	activity.RecordHeartbeat(ctx, "Transcribe")
	<-ctx.Done()
	return nil, ctx.Err()
}

// transcribeOnce routes Transcribe the way the transcribe workflows do, through
// wfutils.Execute, so TRANSCRIBE_QUEUE_ENABLED decides the queue.
type transcribeOnceInput struct {
	Params           activities.TranscribeParams
	HeartbeatTimeout time.Duration
}

func transcribeOnce(ctx workflow.Context, input transcribeOnceInput) (*activities.TranscribeResponse, error) {
	options := wfutils.GetDefaultActivityOptions()
	if input.HeartbeatTimeout > 0 {
		options.HeartbeatTimeout = input.HeartbeatTimeout
	}
	ctx = workflow.WithActivityOptions(ctx, options)

	return wfutils.Execute(ctx, activities.Transcriber.Transcribe, input.Params).Result(ctx)
}

// --- harness ---

type harness struct {
	t      *testing.T
	start  time.Time
	server *testsuite.DevServer
	client client.Client
	stub   *stubTranscriber
}

func newHarness(t *testing.T, transcribeQueueEnabled bool, jobDuration func(n int) time.Duration) *harness {
	t.Helper()

	temporalCLI, err := exec.LookPath("temporal")
	if err != nil {
		t.Skip("the temporal CLI is not on PATH")
	}

	// Routing is decided here, where the workflow code runs.
	t.Setenv("QUEUE", "")
	t.Setenv("TRANSCRIBE_QUEUE_ENABLED", strconv.FormatBool(transcribeQueueEnabled))
	environment.Load()
	t.Cleanup(func() { environment.Load() })

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	server, err := testsuite.StartDevServer(ctx, testsuite.DevServerOptions{
		ExistingPath: temporalCLI,
		LogLevel:     "error",
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = server.Stop() })

	h := &harness{t: t, start: time.Now(), server: server, client: server.Client()}
	h.stub = newStubTranscriber(t, "gpu-a", h.start, jobDuration)

	w := worker.New(h.client, workflowQueue, worker.Options{})
	w.RegisterWorkflow(transcribeOnce)
	require.NoError(t, w.Start())
	t.Cleanup(w.Stop)

	return h
}

func (h *harness) since(at time.Time) string {
	if at.IsZero() {
		return "-"
	}
	return fmt.Sprintf("%5.1fs", at.Sub(h.start).Seconds())
}

type helperWorker struct {
	identity string
	cmd      *exec.Cmd
	stopped  bool
}

// startWorker runs a worker process for queue, against the default stub transcriber, and waits until it polls
func (h *harness) startWorker(queue, identity string, activityCount int) *helperWorker {
	h.t.Helper()
	return h.startWorkerSpec(workerSpec{queue: queue, identity: identity, activityCount: activityCount, stub: h.stub})
}

type workerSpec struct {
	queue         string
	identity      string
	activityCount int
	stub          *stubTranscriber
	// legacyHeartbeat runs legacyTranscribe as Transcribe instead of the real one
	legacyHeartbeat bool
}

func (h *harness) startWorkerSpec(spec workerSpec) *helperWorker {
	h.t.Helper()
	queue, identity := spec.queue, spec.identity

	logFile, err := os.Create(filepath.Join(h.t.TempDir(), identity+".log"))
	require.NoError(h.t, err)

	cmd := exec.Command(os.Args[0], "-test.run=^TestIntegrationHelperWorker$")
	cmd.Env = append(os.Environ(),
		helperEnv+"=1",
		"QUEUE="+queue,
		"IDENTITY="+identity,
		"ACTIVITY_COUNT="+strconv.Itoa(spec.activityCount),
		"TEMPORAL_HOST_PORT="+h.server.FrontendHostPort(),
		"TEMPORAL_NAMESPACE=default",
		"TRANSCRIBER_BASE_URL="+spec.stub.server.URL,
	)
	if spec.legacyHeartbeat {
		cmd.Env = append(cmd.Env, legacyHeartbeatEnv+"=1")
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	require.NoError(h.t, cmd.Start())

	hw := &helperWorker{identity: identity, cmd: cmd}
	h.t.Cleanup(func() {
		hw.stop()
		_ = logFile.Close()
		if h.t.Failed() {
			out, _ := os.ReadFile(logFile.Name())
			h.t.Logf("%s log:\n%s", identity, out)
		}
	})

	require.Eventually(h.t, func() bool {
		resp, err := h.client.DescribeTaskQueue(context.Background(), queue, enumspb.TASK_QUEUE_TYPE_ACTIVITY)
		if err != nil {
			return false
		}
		return slices.ContainsFunc(resp.GetPollers(), func(p *taskqueuepb.PollerInfo) bool { return p.GetIdentity() == identity })
	}, 30*time.Second, 200*time.Millisecond, "%s never polled %s", identity, queue)

	return hw
}

// crash kills the worker the way a crash would: no shutdown, no deferred cancel.
func (hw *helperWorker) crash() {
	_ = hw.cmd.Process.Signal(syscall.SIGKILL)
	_ = hw.cmd.Wait()
	hw.stopped = true
}

func (hw *helperWorker) stop() {
	if hw.stopped {
		return
	}
	hw.stopped = true
	_ = hw.cmd.Process.Signal(os.Interrupt)
	done := make(chan struct{})
	go func() { _ = hw.cmd.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		_ = hw.cmd.Process.Kill()
		<-done
	}
}

func (h *harness) startTranscription(id string, n int, heartbeatTimeout time.Duration) client.WorkflowRun {
	h.t.Helper()

	run, err := h.client.ExecuteWorkflow(context.Background(),
		client.StartWorkflowOptions{ID: id, TaskQueue: workflowQueue},
		transcribeOnce,
		transcribeOnceInput{
			Params: activities.TranscribeParams{
				File:            paths.MustParse(fmt.Sprintf("/mnt/temp/it/in-%d.wav", n)),
				DestinationPath: paths.MustParse("/mnt/temp/it/out"),
				Language:        "no",
			},
			HeartbeatTimeout: heartbeatTimeout,
		})
	require.NoError(h.t, err)

	return run
}

// activityEvents are the Transcribe events of a workflow, from its history.
type activityEvents struct {
	scheduledQueue string
	maxAttempts    int32
	started        []*historypb.ActivityTaskStartedEventAttributes
	startedAt      []time.Time
}

func (h *harness) activityEvents(workflowID string) activityEvents {
	h.t.Helper()

	var events activityEvents
	iter := h.client.GetWorkflowHistory(context.Background(), workflowID, "", false, enumspb.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT)
	for iter.HasNext() {
		event, err := iter.Next()
		require.NoError(h.t, err)
		if scheduled := event.GetActivityTaskScheduledEventAttributes(); scheduled != nil {
			events.scheduledQueue = scheduled.GetTaskQueue().GetName()
			events.maxAttempts = scheduled.GetRetryPolicy().GetMaximumAttempts()
		}
		if started := event.GetActivityTaskStartedEventAttributes(); started != nil {
			events.started = append(events.started, started)
			events.startedAt = append(events.startedAt, event.GetEventTime().AsTime())
		}
	}

	return events
}

// startedActivities counts, as Temporal sees them, the activities of these workflows
// that a worker is executing right now.
func (h *harness) startedActivities(workflowIDs []string) int {
	started := 0
	for _, id := range workflowIDs {
		resp, err := h.client.DescribeWorkflowExecution(context.Background(), id, "")
		if err != nil {
			continue
		}
		for _, activity := range resp.GetPendingActivities() {
			if activity.GetState() == enumspb.PENDING_ACTIVITY_STATE_STARTED {
				started++
			}
		}
	}
	return started
}

// pendingHeartbeat is what Temporal holds as the pending Transcribe's last heartbeat
// details, as raw JSON, and its attempt. Empty when nothing has reached the server.
func (h *harness) pendingHeartbeat(workflowID string) (string, int32) {
	resp, err := h.client.DescribeWorkflowExecution(context.Background(), workflowID, "")
	if err != nil {
		return "", 0
	}
	pending := resp.GetPendingActivities()
	if len(pending) == 0 {
		return "", 0
	}
	var data []string
	for _, payload := range pending[0].GetHeartbeatDetails().GetPayloads() {
		data = append(data, string(payload.GetData()))
	}
	return strings.Join(data, ","), pending[0].GetAttempt()
}

// awaitHeartbeatJob waits until Temporal holds heartbeat details naming a job, so a
// crash after it leaves the job findable, and returns them
func (h *harness) awaitHeartbeatJob(workflowID string) activities.TranscribeHeartbeat {
	h.t.Helper()

	var heartbeat activities.TranscribeHeartbeat
	require.Eventually(h.t, func() bool {
		raw, _ := h.pendingHeartbeat(workflowID)
		return raw != "" && json.Unmarshal([]byte(raw), &heartbeat) == nil && heartbeat.JobID != ""
	}, 30*time.Second, 50*time.Millisecond, "no heartbeat naming a job reached Temporal")

	return heartbeat
}

// --- stub transcriber ---

// stubCanceled as a job's failure ends it CANCELED rather than FAILED
const stubCanceled = "<canceled>"

type stubJob struct {
	ID        string
	Key       string
	Path      string
	Duration  time.Duration
	Submitted time.Time
	Failure   string    // the error the job FAILED with; empty means it COMPLETED
	Completed time.Time // when the job's final status was first handed to a poller
	LastPoll  time.Time
	Polls     int
	Deleted   bool
}

func (j *stubJob) attempt() string {
	return j.Key[strings.LastIndex(j.Key, "/")+1:]
}

// stubTranscriber speaks the transcriber API. A job counts as active from its submit
// until a poller is told its final status, which is when Transcribe returns.
type stubTranscriber struct {
	mu        sync.Mutex
	name      string
	start     time.Time
	duration  func(n int) time.Duration
	failure   func(n int) string
	hold      chan struct{} // while set, submit responses wait for it to close
	jobs      []*stubJob
	byID      map[string]*stubJob
	unknown   []string // IDs asked about that this transcriber never issued
	lastSeen  time.Time
	active    int
	maxActive int
	server    *httptest.Server
}

// newStubTranscriber names its job IDs after itself, as the real transcriber's random
// IDs never match another's
func newStubTranscriber(t *testing.T, name string, start time.Time, duration func(n int) time.Duration) *stubTranscriber {
	s := &stubTranscriber{name: name, start: start, duration: duration, byID: map[string]*stubJob{}}
	s.server = httptest.NewServer(http.HandlerFunc(s.handle))
	t.Cleanup(s.server.Close)
	t.Cleanup(s.releaseSubmits)
	return s
}

// holdSubmits makes the transcriber accept submits but not answer them, until
// releaseSubmits: the job exists, but the worker never learns its ID
func (s *stubTranscriber) holdSubmits() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hold = make(chan struct{})
}

func (s *stubTranscriber) releaseSubmits() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.hold != nil {
		close(s.hold)
		s.hold = nil
	}
}

func (s *stubTranscriber) handle(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	now := time.Now()
	s.lastSeen = now

	if r.Method == http.MethodPost {
		var in struct {
			Path       string `json:"path"`
			OutputPath string `json:"output_path"`
		}
		_ = json.NewDecoder(r.Body).Decode(&in)
		job := &stubJob{
			ID:        fmt.Sprintf("%s-job-%d", s.name, len(s.jobs)+1),
			Key:       r.Header.Get("Idempotency-Key"),
			Path:      in.Path,
			Duration:  s.duration(len(s.jobs) + 1),
			Submitted: now,
		}
		if s.failure != nil {
			job.Failure = s.failure(len(s.jobs) + 1)
		}
		s.jobs = append(s.jobs, job)
		s.byID[job.ID] = job
		s.active++
		s.maxActive = max(s.maxActive, s.active)
		if hold := s.hold; hold != nil {
			s.mu.Unlock()
			<-hold
			s.mu.Lock()
		}
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": job.ID, "status": "QUEUED", "output_path": in.OutputPath})
		return
	}

	job, ok := s.byID[path.Base(r.URL.Path)]
	if !ok {
		s.unknown = append(s.unknown, path.Base(r.URL.Path))
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if r.Method == http.MethodDelete {
		job.Deleted = true
		_ = json.NewEncoder(w).Encode(map[string]any{"id": job.ID, "status": "CANCELED"})
		return
	}

	job.Polls++
	job.LastPoll = now
	if now.Sub(job.Submitted) < job.Duration {
		_ = json.NewEncoder(w).Encode(map[string]any{"id": job.ID, "status": "RUNNING"})
		return
	}

	if job.Completed.IsZero() {
		job.Completed = now
		s.active--
	}
	if job.Failure == stubCanceled {
		_ = json.NewEncoder(w).Encode(map[string]any{"id": job.ID, "status": "CANCELED"})
		return
	}
	if job.Failure != "" {
		_ = json.NewEncoder(w).Encode(map[string]any{"id": job.ID, "status": "FAILED", "error": job.Failure})
		return
	}
	out := "/mnt/temp/it/out/" + filepath.Base(job.Path)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id": job.ID, "status": "COMPLETED", "output_path": "/mnt/temp/it/out",
		"results": []string{out + ".json", out + ".srt", out + ".txt", out + ".words.srt"},
	})
}

// failWith decides, per job number, the error a job FAILS with; "" lets it complete.
func (s *stubTranscriber) failWith(failure func(n int) string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failure = failure
}

func (s *stubTranscriber) requests() (unknown []string, lastSeen time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.unknown), s.lastSeen
}

func (s *stubTranscriber) snapshot() ([]stubJob, int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	jobs := make([]stubJob, len(s.jobs))
	for i, job := range s.jobs {
		jobs[i] = *job
	}
	return jobs, s.maxActive
}

func (h *harness) logJobs(jobs []stubJob) {
	h.t.Logf("%-6s %-8s %-9s %-9s %-9s %-6s %-8s %s", "job", "attempt", "submitted", "ended", "last poll", "polls", "deleted", "result")
	for _, job := range jobs {
		result := "COMPLETED"
		switch {
		case job.Completed.IsZero():
			result = "-"
		case job.Failure != "":
			result = fmt.Sprintf("FAILED %q", job.Failure)
		}
		h.t.Logf("%-6s %-8s %-9s %-9s %-9s %-6d %-8t %s", job.ID, job.attempt(), h.since(job.Submitted), h.since(job.Completed), h.since(job.LastPoll), job.Polls, job.Deleted, result)
	}
}

// --- tests ---

// ACTIVITY_COUNT must hold for the whole transcription, not only the submit: each job
// runs for several poll intervals, and with four jobs and two slots the second pair may
// only start once the first pair has finished. The run with four slots shows the
// measurement can see more than two.
func TestIntegrationActivityCountLimitsConcurrentTranscriptions(t *testing.T) {
	for _, activityCount := range []int{2, 4} {
		t.Run(fmt.Sprintf("ACTIVITY_COUNT=%d", activityCount), func(t *testing.T) {
			h := newHarness(t, true, func(int) time.Duration { return 12 * time.Second })
			h.startWorker(environment.QueueTranscribe, "it-transcribe", activityCount)

			var ids []string
			var runs []client.WorkflowRun
			for n := 1; n <= 4; n++ {
				id := fmt.Sprintf("concurrency-%d", n)
				ids = append(ids, id)
				runs = append(runs, h.startTranscription(id, n, 0))
			}

			// Sample what Temporal reports as executing until every run is done.
			done := make(chan struct{})
			maxStarted := 0
			var sampled sync.WaitGroup
			sampled.Add(1)
			go func() {
				defer sampled.Done()
				for {
					select {
					case <-done:
						return
					case <-time.After(250 * time.Millisecond):
						maxStarted = max(maxStarted, h.startedActivities(ids))
					}
				}
			}()

			for _, run := range runs {
				require.NoError(t, run.Get(context.Background(), nil))
			}
			close(done)
			sampled.Wait()

			jobs, maxActive := h.stub.snapshot()
			h.logJobs(jobs)
			t.Logf("max executing Transcribe activities (Temporal, sampled every 250ms): %d", maxStarted)
			t.Logf("max jobs between submit and completion (stub transcriber): %d", maxActive)

			require.Len(t, jobs, 4)
			assert.Equal(t, activityCount, maxStarted)
			assert.Equal(t, activityCount, maxActive)

			if activityCount == 2 {
				// The second pair waited for a slot from the first.
				firstDone := jobs[0].Completed
				if jobs[1].Completed.Before(firstDone) {
					firstDone = jobs[1].Completed
				}
				for _, job := range jobs[2:] {
					assert.False(t, job.Submitted.Before(firstDone), "%s was submitted before a slot freed", job.ID)
				}
				for _, job := range jobs {
					assert.GreaterOrEqual(t, job.Polls, 2, "%s should span several polls", job.ID)
				}
			}
		})
	}
}

// recoveryHeartbeatTimeout stands in for the production 10 minutes, so a killed
// worker's attempt is retried within the test
const recoveryHeartbeatTimeout = 20 * time.Second

// crashAfterHeartbeat starts a transcription on worker a against stub, waits until
// Temporal holds heartbeat details naming its job, and kills a. It returns the details
// and when the kill happened
func (h *harness) crashAfterHeartbeat(workflowID string, stub *stubTranscriber) (client.WorkflowRun, activities.TranscribeHeartbeat, time.Time) {
	h.t.Helper()

	a := h.startWorkerSpec(workerSpec{queue: environment.QueueTranscribe, identity: "it-worker-a", activityCount: 2, stub: stub})
	run := h.startTranscription(workflowID, 1, recoveryHeartbeatTimeout)
	heartbeat := h.awaitHeartbeatJob(workflowID)
	h.t.Logf("Temporal holds heartbeat %+v at %s", heartbeat, h.since(time.Now()))

	a.crash()
	crashedAt := time.Now()
	h.t.Logf("worker it-worker-a killed (SIGKILL) at %s", h.since(crashedAt))

	return run, heartbeat, crashedAt
}

func (h *harness) logAttempts(workflowID string) activityEvents {
	h.t.Helper()

	events := h.activityEvents(workflowID)
	for i, started := range events.started {
		h.t.Logf("history: ActivityTaskStarted at %s attempt=%d identity=%s last failure=%q",
			h.since(events.startedAt[i]), started.GetAttempt(), started.GetIdentity(), started.GetLastFailure().GetMessage())
	}
	return events
}

// A: a worker killed mid-transcription. Temporal retries after the heartbeat timeout,
// and the new attempt resumes the job its heartbeat names instead of submitting one.
func TestIntegrationResumeAfterWorkerCrash(t *testing.T) {
	h := newHarness(t, true, func(int) time.Duration { return 45 * time.Second })

	run, heartbeat, crashedAt := h.crashAfterHeartbeat("resume", h.stub)
	h.startWorkerSpec(workerSpec{queue: environment.QueueTranscribe, identity: "it-worker-b", activityCount: 2, stub: h.stub})

	var response activities.TranscribeResponse
	require.NoError(t, run.Get(context.Background(), &response))

	jobs, _ := h.stub.snapshot()
	h.logJobs(jobs)
	events := h.logAttempts("resume")

	require.Len(t, jobs, 1, "the retry submits nothing")
	job := jobs[0]
	assert.Equal(t, heartbeat.JobID, job.ID)
	assert.Equal(t, h.stub.server.URL, heartbeat.Transcriber)
	assert.Equal(t, "1", job.attempt(), "the job is still attempt 1's")
	assert.False(t, job.Deleted)
	assert.True(t, job.LastPoll.After(crashedAt.Add(recoveryHeartbeatTimeout-5*time.Second)), "attempt 2 polled it")
	assert.False(t, job.Completed.IsZero())

	require.Len(t, events.started, 1)
	assert.Equal(t, int32(2), events.started[0].GetAttempt())
	assert.Equal(t, "it-worker-b", events.started[0].GetIdentity())
	assert.Equal(t, enumspb.TIMEOUT_TYPE_HEARTBEAT, events.started[0].GetLastFailure().GetTimeoutFailureInfo().GetTimeoutType())

	assert.Equal(t, paths.MustParse("/mnt/temp/it/out/in-1.wav.json"), response.JSONPath)
}

// B: the job finished while no worker was running. The retry returns its result.
func TestIntegrationRecoverACompletedJob(t *testing.T) {
	h := newHarness(t, true, func(int) time.Duration { return 5 * time.Second })

	run, heartbeat, crashedAt := h.crashAfterHeartbeat("completed", h.stub)

	// The job is done 5s after its submit; bring a worker back only after that.
	time.Sleep(8 * time.Second)
	h.startWorkerSpec(workerSpec{queue: environment.QueueTranscribe, identity: "it-worker-b", activityCount: 2, stub: h.stub})

	var response activities.TranscribeResponse
	require.NoError(t, run.Get(context.Background(), &response))

	jobs, _ := h.stub.snapshot()
	h.logJobs(jobs)
	events := h.logAttempts("completed")

	require.Len(t, jobs, 1, "the retry submits nothing")
	assert.Equal(t, heartbeat.JobID, jobs[0].ID)
	assert.True(t, jobs[0].Completed.After(crashedAt), "first seen complete by attempt 2")
	require.Len(t, events.started, 1)
	assert.Equal(t, int32(2), events.started[0].GetAttempt())
	assert.Equal(t, paths.MustParse("/mnt/temp/it/out/in-1.wav.json"), response.JSONPath)
	assert.Equal(t, paths.MustParse("/mnt/temp/it/out/in-1.wav.txt"), response.TXTPath)
}

// C: the retry runs on another GPU host. Its worker only knows its own transcriber, so it
// leaves the job on the first one alone (no request there, no question about it here)
// and submits a new job on its own
func TestIntegrationRecoveryAcrossHosts(t *testing.T) {
	h := newHarness(t, true, func(int) time.Duration { return 10 * time.Minute })
	gpuB := newStubTranscriber(t, "gpu-b", h.start, func(int) time.Duration { return 5 * time.Second })

	run, heartbeat, crashedAt := h.crashAfterHeartbeat("cross-host", h.stub)
	h.startWorkerSpec(workerSpec{queue: environment.QueueTranscribe, identity: "it-worker-b", activityCount: 2, stub: gpuB})

	var response activities.TranscribeResponse
	require.NoError(t, run.Get(context.Background(), &response))

	jobsA, _ := h.stub.snapshot()
	jobsB, _ := gpuB.snapshot()
	t.Log("gpu-a:")
	h.logJobs(jobsA)
	t.Log("gpu-b:")
	h.logJobs(jobsB)
	h.logAttempts("cross-host")

	_, lastSeenA := h.stub.requests()
	unknownB, _ := gpuB.requests()
	t.Logf("gpu-a last request at %s; IDs gpu-b was asked about but never issued: %v", h.since(lastSeenA), unknownB)

	require.Len(t, jobsA, 1)
	assert.Equal(t, heartbeat.JobID, jobsA[0].ID)
	assert.True(t, jobsA[0].Completed.IsZero(), "gpu-a's job is still running")
	assert.False(t, lastSeenA.After(crashedAt), "nothing asks gpu-a after the crash")

	require.Len(t, jobsB, 1, "gpu-b's worker submits its own job")
	assert.Equal(t, "2", jobsB[0].attempt())
	assert.Empty(t, unknownB, "gpu-a's job ID is never asked about on gpu-b")
	assert.Equal(t, paths.MustParse("/mnt/temp/it/out/in-1.wav.json"), response.JSONPath)
}

// D: an attempt that heartbeated the old string "Transcribe" is retried by the new code.
func TestIntegrationLegacyHeartbeatDetails(t *testing.T) {
	h := newHarness(t, true, func(int) time.Duration { return 5 * time.Second })

	legacy := h.startWorkerSpec(workerSpec{queue: environment.QueueTranscribe, identity: "it-legacy", activityCount: 2, stub: h.stub, legacyHeartbeat: true})
	run := h.startTranscription("legacy", 1, recoveryHeartbeatTimeout)

	var details string
	require.Eventually(t, func() bool {
		details, _ = h.pendingHeartbeat("legacy")
		return details != ""
	}, 30*time.Second, 50*time.Millisecond)
	t.Logf("Temporal holds legacy heartbeat %s at %s", details, h.since(time.Now()))
	assert.Equal(t, `"Transcribe"`, details)

	legacy.crash()
	h.startWorkerSpec(workerSpec{queue: environment.QueueTranscribe, identity: "it-worker-b", activityCount: 2, stub: h.stub})

	require.NoError(t, run.Get(context.Background(), nil), "the old details don't break the retry")

	jobs, _ := h.stub.snapshot()
	h.logJobs(jobs)
	events := h.logAttempts("legacy")

	require.Len(t, jobs, 1, "with no job to resume, the retry submits one")
	assert.Equal(t, "2", jobs[0].attempt())
	require.Len(t, events.started, 1)
	assert.Equal(t, int32(2), events.started[0].GetAttempt())
}

// F: the worker dies after the transcriber accepted its job but before the ID reached
// Temporal. Nothing tells the retry about that job, so it submits another one: the
// duplicate this recovery can't prevent.
func TestIntegrationCrashBeforeTheHeartbeat(t *testing.T) {
	h := newHarness(t, true, func(int) time.Duration { return 5 * time.Second })
	h.stub.holdSubmits()

	a := h.startWorkerSpec(workerSpec{queue: environment.QueueTranscribe, identity: "it-worker-a", activityCount: 2, stub: h.stub})
	run := h.startTranscription("unpersisted", 1, recoveryHeartbeatTimeout)

	require.Eventually(t, func() bool {
		jobs, _ := h.stub.snapshot()
		return len(jobs) == 1
	}, 30*time.Second, 50*time.Millisecond, "attempt 1 never submitted")
	details, attempt := h.pendingHeartbeat("unpersisted")
	t.Logf("job accepted, response held; Temporal holds heartbeat %q for attempt %d", details, attempt)
	assert.Empty(t, details, "the job's ID never reached Temporal")

	a.crash()
	crashedAt := time.Now()
	t.Logf("worker it-worker-a killed (SIGKILL) at %s", h.since(crashedAt))
	h.stub.releaseSubmits()
	h.startWorkerSpec(workerSpec{queue: environment.QueueTranscribe, identity: "it-worker-b", activityCount: 2, stub: h.stub})

	require.NoError(t, run.Get(context.Background(), nil))

	jobs, _ := h.stub.snapshot()
	h.logJobs(jobs)
	h.logAttempts("unpersisted")

	require.Len(t, jobs, 2, "the retry can't know about attempt 1's job")
	assert.Equal(t, "1", jobs[0].attempt())
	assert.Zero(t, jobs[0].Polls, "attempt 1's job is orphaned: nobody ever polls it")
	assert.Equal(t, "2", jobs[1].attempt())
}

// With both queues served, TRANSCRIBE_QUEUE_ENABLED decides which worker runs it.
func TestIntegrationRoutingFollowsTheFlag(t *testing.T) {
	tests := []struct {
		enabled   bool
		wantQueue string
		wantBy    string
	}{
		{enabled: false, wantQueue: environment.QueueWorker, wantBy: "it-worker-queue"},
		{enabled: true, wantQueue: environment.QueueTranscribe, wantBy: "it-transcribe-queue"},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("TRANSCRIBE_QUEUE_ENABLED=%t", test.enabled), func(t *testing.T) {
			h := newHarness(t, test.enabled, func(int) time.Duration { return time.Second })
			h.startWorker(environment.QueueWorker, "it-worker-queue", 2)
			h.startWorker(environment.QueueTranscribe, "it-transcribe-queue", 2)

			run := h.startTranscription("routing", 1, 0)
			require.NoError(t, run.Get(context.Background(), nil))

			events := h.activityEvents("routing")
			require.Len(t, events.started, 1)
			t.Logf("Transcribe scheduled on %q, run by %q", events.scheduledQueue, events.started[0].GetIdentity())

			assert.Equal(t, test.wantQueue, events.scheduledQueue)
			assert.Equal(t, test.wantBy, events.started[0].GetIdentity())
		})
	}
}

// The transcriber stopping a job at its JOB_TIMEOUT has to end Transcribe after one
// attempt, although the activity allows ten, while any other failure is still retried.
// The stub fails jobs after a second instead of 30 minutes.
func TestIntegrationTranscriberJobTimeoutIsNotRetried(t *testing.T) {
	tests := []struct {
		name         string
		failure      func(n int) string
		wantAttempt  int32
		wantJobs     int
		wantFailType string // empty: the workflow succeeds
	}{
		{
			name:         "JOB_TIMEOUT",
			failure:      func(int) string { return "timeout" },
			wantAttempt:  1,
			wantJobs:     1,
			wantFailType: activities.TranscriberJobTimeoutErrorType,
		},
		{
			name: "transient failure",
			failure: func(n int) string {
				if n == 1 {
					return "write srt: no space left on device"
				}
				return ""
			},
			wantAttempt: 2,
			wantJobs:    2,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h := newHarness(t, true, func(int) time.Duration { return time.Second })
			h.stub.failWith(test.failure)
			h.startWorker(environment.QueueTranscribe, "it-transcribe", 2)

			run := h.startTranscription("job-timeout", 1, 0)
			err := run.Get(context.Background(), nil)
			endedAt := time.Now()

			// The workflow is closed, so nothing can be scheduled any more; waiting past
			// the retry interval (1s) makes sure no late submit is missed either.
			time.Sleep(3 * time.Second)

			jobs, _ := h.stub.snapshot()
			events := h.activityEvents("job-timeout")

			h.logJobs(jobs)
			t.Logf("workflow ended at %s, error: %v", h.since(endedAt), err)
			t.Logf("history: Transcribe scheduled with MaximumAttempts=%d", events.maxAttempts)
			for i, started := range events.started {
				t.Logf("history: ActivityTaskStarted at %s attempt=%d identity=%s last failure=%q",
					h.since(events.startedAt[i]), started.GetAttempt(), started.GetIdentity(), started.GetLastFailure().GetMessage())
			}

			assert.Equal(t, int32(10), events.maxAttempts, "the activity's retry policy is unchanged")
			require.Len(t, events.started, 1)
			assert.Equal(t, test.wantAttempt, events.started[0].GetAttempt())
			require.Len(t, jobs, test.wantJobs, "one transcription job per attempt")

			if test.wantFailType == "" {
				require.NoError(t, err)
				assert.Equal(t, "2", jobs[1].attempt())
				return
			}

			require.Error(t, err)
			var appErr *temporal.ApplicationError
			require.True(t, errors.As(err, &appErr), "the workflow fails with the activity's error")
			assert.Equal(t, test.wantFailType, appErr.Type())
			assert.True(t, appErr.NonRetryable())
			assert.Contains(t, appErr.Error(), jobs[0].ID)
		})
	}
}
