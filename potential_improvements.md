# Potential improvements

Findings are grouped by severity so the list stays usable as it grows:

- **Tier 1** — a failure looks like a success, or a workflow wedges forever.
- **Tier 2** — determinism and operational hazards (replay safety, worker health, history growth).
- **Tier 3** — simplifications: duplication that a single abstraction would collapse.
- **Tier 4** — security hardening. The HTTP services sit behind an identity-aware proxy, so
  these are the in-process defenses that should back that proxy up, not emergencies.
- **Tier 5** — conventions, cleanup, dead code.

Entries marked _(previously logged)_ predate the 2026-08-11 full-tree audit and are kept
verbatim; two of them carry an **Update** with what the audit added.

---

## Process — why the rest of this list accumulated

Neither CI workflow runs `go test`, `go vet`, `workflowcheck`, or a linter, and neither has a
`pull_request` trigger, so a PR gets **zero** automated feedback — not even a compile check.
`Makefile:3-7` defines exactly the right target and nothing invokes it. Meanwhile
`.github/workflows/publish.yml:11-31` auto-bumps `VERSION`, tags, and releases on every push
to `master`, and `cmd/worker/main.go:80-88` self-updates from GitHub releases every 5 minutes
with no `selfupdate.Validator` (no checksum, no signature). So every merge to master reaches
the worker fleet by two independent paths within minutes, ungated and unverified.

Also missing from CI: `go mod tidy -diff`, `govulncheck`, image scanning, and a
`.dockerignore` (all four Dockerfiles `COPY . .`, so a local `docker build` bakes in any
`cmd/*/.env` present on the builder).

Tooling baseline as of 2026-08-11 — `go vet ./...` clean, `go test ./...` all pass, but:

| Check | Result |
| --- | --- |
| `workflowcheck ./...` | **exit 3**, 10 non-determinism sites — so `make test` fails today |
| `golangci-lint run --no-config --default=standard ./...` | **89 issues**: errcheck 45, staticcheck 27, ineffassign 6, unused 8, govet 3 |
| Coverage | `workflows/export` 9.9%, `activities` 13.3%, `services/ffmpeg` 4.1%, `services/cantemo` 0%; **no tests at all** for `directus`, `transcribe`, `baton`, `ftp`, `subtrans`, `filecatalyst`, `common`, `cache` |
| Error handling | 258 `fmt.Errorf` vs 18 `merry.*` — the `ansel1/merry/v2` dependency is effectively unused |
| Config | 73 non-test `os.Getenv` sites over 55 variables; exactly **one** validated at startup |

Churn hotspots over the last 500 commits, i.e. where bugs cluster:
`workflows/export/generate_short.go` (28), `workflows/ingest/incremental_ingest.go` (27),
`workflows/export/vx_export_bmm.go` (25), `workflows/workflows.go` (23),
`cmd/worker/main.go` (20), `services/vidispine/export.go` (19).

---

## Tier 1 — Silent success and permanent hangs

- **`VXExportToVOD` hangs forever if any transcode fails.** `workflows/export/vx_export_vod.go:185-199`
  computes the number of `filesSelector.Select(ctx)` calls from the *input* lists, but futures
  are only added inside `onVideoCreated`, which returns early at `:146` and `:156` **without
  adding one**. A single failed `TranscodeToVideoH264` means more `Select` calls than futures,
  so the coroutine blocks until `WorkflowExecutionTimeout` and the `service.errs` check at
  `:206` is never reached. Duplicate entries in `params.ParentParams.Resolutions` cause the
  same mismatch, because `videosByQuality` is a map and dedupes while `qualitiesWithLanguages`
  does not. Fix: increment a counter at every `AddFuture` and drain with
  `for pending > 0 { selector.Select(ctx); pending-- }`, or drop the selector and `Get` a
  `[]workflow.Future` in order.

- **`IngestSyncFix` has the same deadlock, and additionally reports every failure as success.**
  `workflows/ingest/sync_fix.go:109-159` always performs `2×len(languages)` `Select` calls
  regardless of how many futures were added, and `errs` accumulated at `:115,124,128,150` is
  discarded by the `return nil` at `:163`.

- **Child-workflow failure swallowed by `err` shadowing.** `workflows/ingest/asset_ingest.go:186-197`,
  checked at `:209`. The `OrderFormUpload` case uses `outputDir, err := …`, which declares a
  **new** case-scoped `err`; the subsequent `ExecuteChildWorkflow(...).Get()` assigns that
  inner variable, so the function-scope check at `:209` sees `nil` and a failed
  `MoveUploadedFiles` returns success. The `OrderFormSeriesMaster` branch at `:160-162` does
  it correctly with `var outputDir paths.Path` + `=`. `ineffassign` flags this at `:192`.

- **Audio normalization is inverted — it never boosts quiet audio.** `workflows/misc/normalize_audio.go:50`:
  ```go
  // Don't adjust if the suggested adjustment is less than 0.01 Db
  if r128Result.SuggestedAdjustment <= 0.01 {
  ```
  The code does the opposite of its comment and there is no `math.Abs`, so adjustments
  *greater* than 0.01 dB — exactly the ones that matter — are skipped, while negligible and
  negative ones are applied. Fix: `if math.Abs(r128Result.SuggestedAdjustment) > 0.01`.

- **`ExecuteIndependently` is a no-op, so sidecar subtitle imports are lost.**
  `utils/workflows/execute.go:96-103` sets `ParentClosePolicy` via `workflow.WithChildOptions`
  and then calls `Execute`, which schedules an **activity** — `ParentClosePolicy` is a
  child-workflow option and is ignored by `workflow.ExecuteActivity`. The only caller,
  `workflows/misc/transcribe-vx.go:111`, discards the returned task and the workflow completes
  immediately after, so `ImportFileAsSidecarActivity` is cancelled rather than allowed to
  finish. Fix: either `.Wait(ctx)` at the call site and delete the helper, or implement it as
  a genuinely detached child workflow.

- **`SendTelegramErorr` ignores its `channel` argument.** `utils/workflows/notify.go:13-15`
  accepts `channel telegram.Chat` and then hardcodes `telegram.ChatOther`, so callers passing
  `telegram.ChatBMM` (e.g. `workflows/ingest/bmm_simple_upload.go:52`) send error
  notifications to the wrong chat. The name is also misspelled.

- **`paths.Files.Less` is not a valid ordering, and it runs inside workflow code.**
  `paths/paths.go:276-278`:
  ```go
  return f[i].Drive.Value < f[j].Drive.Value || f[i].Path < f[j].Path
  ```
  This is not antisymmetric — for `{drive:b,path:a}` vs `{drive:a,path:b}` both `Less(i,j)`
  and `Less(j,i)` are true — so `sort.Sort` produces arbitrary output. It is called at
  `workflows/ingest/multitrack.go:53` and `:69`, i.e. **inside a workflow**, which makes
  multitrack channel ordering non-deterministic across replay rather than merely wrong. Fix:
  compare drive first and fall through to path (`slices.SortFunc` + `cmp.Or`).

- **`ExecuteAnalysisCmd` deadlocks on chatty stderr.** `utils/execute_command.go:66-91` drains
  stdout to completion *before* it starts reading stderr. ffmpeg writes the loudnorm/astats
  JSON that this function exists to scrape to **stderr**, so once the 64 KiB stderr pipe
  buffer fills, the child blocks on write, never closes stdout, and `scannerOut.Scan()` blocks
  forever — the activity hangs until its timeout. Used by EBU R128 analysis
  (`services/ffmpeg/ebur128.go:41`). The correct concurrent version is sitting commented out
  at `:26-32`. Fix: drain both pipes concurrently, or assign `bytes.Buffer`s and use
  `cmd.Run()`. (`ExecuteCmd` immediately above is fine — it already uses
  `cmd.Stderr = &errorBytes`.)

- **HTTP 4xx/5xx treated as success across ~50 resty call sites.** Only three places in
  `services/**` call `IsError()`, and resty returns `err == nil` for an HTTP 500.
  `services/vidispine/vsapi/items.go:30` therefore reports a **failed bulk item delete as
  success**; `vsapi/metadata.go:51-99` yields a zero-valued `*MetadataResult` so downstream
  `Get()` silently returns fallback titles, languages, and `0` durations. Same pattern in
  `services/subtrans/client.go:31-87`, `services/baton/tasks.go`, and
  `vsapi/{collections,relations,jobs,search,placeholder,files_paths}.go`. Fix: one
  `OnAfterResponse` hook per client that turns `resp.IsError()` into an error —
  `services/cantemo/client.go:30-36` already does exactly this, so copy it. This single change
  converts ~50 silent failures into errors.

- **Nil-deref panics.** A panic inside `workflow.Go` fails the workflow task, which then
  retries forever.
  - `workflows/ingest/incremental_ingest.go:161-176` — the import error is discarded with `_`,
    the `if err != nil` at `:169` inspects an unrelated captured variable, and then
    `lowresImportJob.FileID` is dereferenced at `:175`.
  - `workflows/export/vx_export_playout.go:30`, `workflows/export/isilon_export.go:119`,
    `workflows/export/generate_short.go:127,199` — `*params.MergeResult.VideoFile`, but
    `VXExport` legitimately sets `MakeVideo: !bmmOnly && fileInfo.HasVideo`
    (`vx_export.go:177`), so `VideoFile` is nil for audio-only items.
  - `services/vidispine/export.go:233-237` — `shapes.GetShape("original")` used without the
    nil check that every sibling call site has (`export.go:191`, `clips.go:36,97,129`).
  - `services/transcode/subtitles.go:17-22` — `CreateBurninASSFile`'s error is **never
    checked** and `assFile.Local()` is dereferenced; the function returns `(nil, err)` on four
    paths. Line 16 (`assFile := &subtitleFile`) is a dead assignment.

- **`MoveMBFile` always reports success.** `workflows/misc/slow_move_files.go:47-69` does
  `_, _ = c.SignalWithStartWorkflow(...)` then `return nil`, so the worker flow never starting
  is invisible. The `ctx.Value(ClientContextKey).(client.Client)` assertion is also unchecked
  and panics if the background activity context wasn't wired (`cmd/worker/main.go:148`).

- **Two `cmd/httpin` handlers return `200 OK` on failure.** `cmd/httpin/main.go:218-229` — the
  `NormalizeAudio` case's `target, err := strconv.ParseFloat(...)` shadows the function-scope
  `err` declared at `:58`, so the check at `:241` sees `nil` and `:248` returns `200 OK` with a
  nil body even when `ExecuteWorkflow` failed. Separately the `switch job` has **no `default`
  case**, so a typo'd or removed job name also falls through to `:248` and returns `200 OK`
  with body `null` — the FileCatalyst and watcher integrations read that as success for work
  that was never started. Worth enabling the `shadow` analyzer; this class is invisible to the
  current toolchain.

- **`services/transcode/preview.go::GrowingPreview` calls `os.Exit(1)`** if creating the
  tail→ffmpeg pipe fails. Library/activity code should return the error instead of killing the
  whole worker process. _(previously logged)_

- **Smaller error-loss sites.**
  - `services/transcribe/transcribe.go:287-333` — `MergeTranscripts` appends to `errs` at
    `:293` and never returns them (the signature has no `error`), so unreadable transcript
    files silently produce a short merged transcript. `SA4010` confirms.
  - `workflows/misc/transcribe-vx.go:106-109` — formats `errs` (empty at that point) instead of
    `err`, so the real Vidispine failure reason is dropped.
  - `workflows/ingest/masters.go:88-104` — runs the expensive external Baton QC into `report`
    and never uses it; `MasterResult.Report` is always nil. Either wire it up or delete the
    block.
  - `workflows/ingest/multitrack.go:109` — `return nil, nil` despite having computed
    `result.AssetID` and `muxResult.OutputPath`.

---

## Tier 2 — Determinism and operational hazards

- **`workflowcheck` reports 10 sites; `make test` fails because of them.** Triaged:
  - Real: `workflows/export/generate_short.go:101` uses `time.Now()` in a workflow, so the
    generated filename differs on replay → `workflow.Now(ctx)`.
  - Real: `workflows/export/shorts.go:432` calls `os.Stat` inside
    `generateThumbnailForShort(ctx workflow.Context, …)`. That runs on the *workflow* worker,
    which may not even mount the storage, is not recorded in history, and can take the
    opposite branch on replay. Use the existing `wfutils.RcloneCheckFileExists`
    (`utils/workflows/files.go:196`).
  - Real: `workflows/export/shorts.go:254,353,361` and `workflows/misc/cleanup_production.go:83-90`
    use `fmt.Printf` instead of `workflow.GetLogger`.
  - Conservative false positives: the raw `json.MarshalIndent`/`Unmarshal` in
    `workflows/ingest/import_subtitles.go:113` and `workflows/misc/masv_import.go` — route
    them through the existing `wfutils.MarshalJson`/`UnmarshalJson`
    (`utils/workflows/encoding.go:22,34`) and annotate what remains, so the checker can
    eventually become a hard gate.
  - `workflows/misc/merge_import_subs.go:88` ranges over an activity-returned map. Benign
    today because it only populates a map whose keys are later ordered via
    `GetMapKeysSafely`, but it should use that helper for consistency.

- **`ActivityWG` is a global `sync.WaitGroup` mutated from workflow code.**
  `utils/workflows/execute.go:29,87-91`, waited at `cmd/worker/main.go:269`. `Add(1)` runs on
  every replay, but `Done()` only runs if that `workflow.Go` coroutine is scheduled to
  completion — so fire-and-forget `Execute` calls and workflows evicted from the worker cache
  leak counts permanently, `ActivityWG.Wait()` blocks forever, and **that worker never
  self-updates again**. It is also accounting in the wrong process: the counter lives where the
  *workflow* runs, but transcode/audio/live workers register activities only
  (`cmd/worker/main.go:247-252`), so their counter is always zero — the update gate is a no-op
  on precisely the workers running multi-hour ffmpeg jobs. Each `Execute` additionally spawns a
  coroutine that double-consumes the future, which is plausibly why
  `DeadlockDetectionTimeout` is set to 3 h at `cmd/worker/main.go:139`. Fix: do this
  bookkeeping in the **activity** interceptor
  (`AnalyticsActivityInboundInterceptor.ExecuteActivity`, `utils/workflows/interceptors.go:83`)
  — activity-side, replay-free, and it already wraps every activity.

- **A 10-minute heartbeat timeout on an activity that never heartbeats.**
  `activities/vizualizer.go:58-92` is a pure ticker/poll loop with zero
  `activity.RecordHeartbeat`, and it is invoked from `workflows/export/vx_export_vod.go:119`
  with `Timeout: 2 * time.Hour` under `GetDefaultActivityOptions()`, which sets
  `HeartbeatTimeout: time.Minute * 10` (`utils/workflows/common.go:20`). Any visualization job
  longer than 10 minutes therefore hits heartbeat timeout, retries 10×, and fails identically
  every time. The correct pattern is at `activities/vidispine/files.go:224`.

- **Unbounded workflow histories.**
  - `workflows/export/generate_short.go:155-176` — `for { CheckJobStatus; Sleep(5s) }` with no
    attempt cap: two history events every five seconds until the server terminates the
    workflow.
  - `workflows/ingest/incremental_ingest.go:214-242` — `maxCopyAttempts = 1000`, one activity
    plus one timer per iteration (~6000 events, ~16 h), and `signalReceived` is only polled
    *after* each copy returns, so a signal can be delayed a full cycle. Fix: a
    `workflow.Selector` with `AddReceive` + `AddFuture` (or `AwaitWithTimeout`), plus
    `ContinueAsNew` past a threshold.
  - `workflows/misc/slow_move_files.go:150-224` — an eternal fixed-ID signal workflow whose
    history is never reset. Add a `GetCurrentHistoryLength()` threshold →
    `workflow.NewContinueAsNewError`.
  - Oversized payloads: `workflows/ingest/import_subtitles.go:40-44` passes a full word-level
    `Transcription` (with `Tokens []int`) as a workflow argument;
    `workflows/export/vx_export.go:200` re-serializes the whole `ExportData` into every
    destination child; `workflows/export/merge_export_data.go:40-42` persists the entire
    `MergeInput` through `workflow.SideEffect` for what is a pure function;
    `workflows/scheduled/files_cleanup.go:137-140` returns every deleted path across ~60
    folders as the workflow result.

- **Workflows that never call `WithActivityOptions` get no `StartToCloseTimeout` or
  `HeartbeatTimeout`.** `wfutils.Execute` backfills only `ScheduleToCloseTimeout = 3h` and a
  retry policy (`utils/workflows/execute.go:79-81`); everything else stays zero. Affected:
  `workflows/export/shorts.go:57` (`BulkExportShorts`), `:108` (`ExportShort`), and
  `workflows/ingest/import_subtitles.go:88`. Also `workflows/export/generate_short.go:68` runs
  `GetExportDataActivity` **before** options are applied at `:86-87` — move those two lines
  above it.

- **`ExecuteWithLowPrioQueue` has drifted from `Execute`.** `utils/workflows/execute.go:106-130`
  copy-pastes the retry-policy switch from `:67-77` verbatim but omits the
  `ScheduleToCloseTimeout` default and the `ActivityWG` bookkeeping. Extract the shared body
  as `executeOn(ctx, queue, activity, params)`.

- **Activity→queue routing is stringly-typed with a silent fallback.** `activities/queues.go:62-74`
  routes on the reflected **short method name** against three slices; anything unmatched
  silently lands on the worker queue, so an ffmpeg activity attached to the wrong struct fails
  at runtime as an opaque timeout instead of at build time. All 112 activity method names are
  currently unique (verified) but nothing enforces that, and `DisableRegistrationAliasing: true`
  (`cmd/worker/main.go:137`) would panic on a collision on the debug queue where both `Video`
  and `Audio` are registered. Fix: build a `map[string]string` once (which also removes a
  linear `lo.Contains` scan on every `Execute` call) and add a test asserting the name sets are
  disjoint.

- **Registration drift between two hand-maintained lists.** `workflows/workflows.go` keeps
  `TriggerableWorkflows` and `WorkerWorkflows` with 21 overlapping entries and no shared source
  of truth; `TriggerableWorkflows` is missing `export.BulkExportShorts`, `export.ExportShort`,
  `export.GenerateShort`, `export.IsilonExport`, `ingestworkflows.AssetJSON`, and
  `ingestworkflows.ImportSubtitles`. Fix: derive `TriggerableWorkflows` as a filtered view of
  one registry, plus a test asserting `TriggerableWorkflows ⊆ WorkerWorkflows`. Separately,
  `workflows/misc/cleanup_production.go:61` executes `MoveFileByImportDate` as an activity that
  is registered nowhere, so that flow cannot run at all even after the workflow is registered.

- **`paths.Drive.UnmarshalJSON` rejects the empty string**, so any *unset non-pointer*
  `paths.Path` field in a workflow/activity payload fails to decode with `drive not found`
  (this is what broke `Test_VBExportToXDCAM` until `OriginalFile` was set in the fixture).
  Either make `Drive.UnmarshalJSON` tolerate `""` (zero-value round-trip) or use `*paths.Path`
  for optional fields. _(previously logged)_

- **ffmpeg-based transcode functions in `services/transcode/`** (e.g. `playout_mux.go`,
  `merge.go`, `h264.go`, `prores.go`, `hap.go`, `avc_intra.go`, `mux_mxf_simple.go`,
  audio/video variants) write an output path directly to `ffmpeg.Do` without first running
  `os.MkdirAll(filepath.Dir(out), ...)`. `Mux` in `mux.go` just hit this in production (missing
  `output/vod/` directory) and was fixed locally. The same latent bug likely affects the other
  call sites — worth a consistent sweep to mirror the convention used in `activities/files.go`
  (`MoveFile`, `CopyFile`, `WriteFile`). _(previously logged)_

  **Update (2026-08-11 audit):** enumerated — it is **29 of 30** output-writing functions.
  Only `Mux` (`services/transcode/mux.go:46`) creates its parent directory. The rest:
  `AudioAac`, `PrepareForTranscription`, `AudioWav`, `AudioMP3`, `SplitAudioChannels`,
  `ExtractAudioChannels`, `GenerateToneFile`, `TrimFile`, `Convert51to4Mono`, `AvcIntra`,
  `H264`, `AdjustAudioLevel`, `MultitrackMux`, `MergeVideo`, `MergeAudio`,
  `MergeSubtitlesByOffset`, `MergeSubtitles`, `HAP`, `MuxToSimpleMXF`, `PrependSilence`,
  `PlayoutMux`, `ProRes`, `VideoH264`, `Preview`, `AudioPreview`, `GrowingPreview`,
  `SubtitleBurnIn`, `CreateBurninASSFile`, `XDCAM`. Rather than 29 separate patches, the
  shared `ffmpeg.Run(Job)` helper proposed in Tier 3 closes the whole class in one place —
  along with the 34 sites that chmod output to `os.ModePerm` (0777).

- **`services/vidispine/export.go::enrichClipWithEmbeddedAudio` mishandles the `zxx`** (no
  linguistic content / music) language on the **16-channel** path (uses `MU1ChannelStart` = -1)
  and the **softron 64-channel** path (uses `SoftronStartCh` = -1). For `zxx`,
  `uint(l.MU1ChannelStart + i)` / `uint(langInfo.SoftronStartCh)` underflow to a garbage stream
  ID — no error, just wrong/garbage channels. The simpler embedded branches (0/1/2/8
  components) handle `zxx` correctly. This is the *embedded* audio-source path; the
  related-audio path was just fixed to fall back to the clip's own original audio for `zxx`.
  Worth handling `zxx` (and any language with -1 channel offsets) explicitly on these two
  embedded branches too. _(previously logged)_

  **Update (2026-08-11 audit):** traced end to end. `AudioStream.ChannelID`/`StreamID` are
  `uint` (`services/vidispine/export.go:44`) and `SoftronStartCh: -1` for both `zxx`
  (`language_config.go:602`) and `und` (`:617`), so `uint(-1)` = `18446744073709551615` and
  `+1` wraps to `0`. That value reaches an ffmpeg filter verbatim at
  `services/transcode/merge.go:161` as `pan=stereo|c0=c18446744073709551615|c1=c0`. And
  `export.go:143-148` explicitly routes `zxx` into this branch, so it is reachable, not
  theoretical. The 16-channel path at `export.go:382` is the same class but is currently
  masked: `MU1ChannelCount` happens to be `0` for those languages, so the loop body never
  runs and the result is a **silently empty** `Streams` (→ `anullsrc` silence) instead of an
  error. Cleanest fix: change `StreamID`/`ChannelID` to `int` — they are never used as
  unsigned — and reject negative offsets explicitly rather than relying on a count of zero.

---

## Tier 3 — Simplifications

- **One `ffmpeg.Run(Job)` collapses ~23 wrappers and fixes the `MkdirAll` class at once.**
  18+ functions in `services/transcode/` are the same seven-step pipeline: probe → build
  output name → join output path → build args → `ffmpeg.Do` → `os.Chmod` → wrap in a result
  struct. Steps 1, 3, 5, 6 and 7 are byte-identical modulo variable names. Introduce in
  `services/ffmpeg`:
  ```go
  type Job struct {
      Input, Output string
      Args    []string    // codec/filter args only — no -i/-y/-progress/output
      Info    *StreamInfo // nil ⇒ probe Input
      OutMode os.FileMode // default 0664
  }
  func Run(j Job, cb ProgressCallback) (StreamInfo, error)
  ```
  `Run` probes if `Info` is nil, runs `os.MkdirAll(filepath.Dir(j.Output), 0775)`, prepends
  `-progress pipe:1 -hide_banner -i <input>`, appends `-y <output>`, calls `Do`, then chmods.
  Folds in `AudioAac`, `PrepareForTranscription`, `AudioWav`, `AudioMP3`, `AdjustAudioLevel`,
  `TrimFile`, `Convert51to4Mono`, `GenerateToneFile`, `PrependSilence`, `AvcIntra`, `H264`,
  `XDCAM`, `ProRes`, `VideoH264`, `MuxToSimpleMXF`, `Mux`, `PlayoutMux`, `MergeVideo`,
  `MergeAudio`, `SubtitleBurnIn`, `Preview`, `MultitrackMux`, `SplitAudioChannels`; `HAP`
  becomes three `Run` calls.

  While there: collapse the **six** competing "base name without extension" idioms onto the
  existing `paths.Path.BaseNoExt()` (`paths/paths.go:177`), and replace the 34 `os.ModePerm`
  (0777, world-writable masters on a shared Isilon mount) with 0664/0775.

  Remaining duplication in the same package: `MergeSubtitles` (`merge.go:328-435`) and
  `MergeSubtitlesByOffset` (`:250-325`) share ~55 identical lines — extract
  `concatSrtFiles(...)`; `Mux` and `MuxToSimpleMXF` are the same function with and without
  language metadata; the subtitle burn-in prep is copy-pasted in `h264.go:106-113`,
  `prores.go:63-69`, `avc_intra.go:78-84`; and `EncodeResult` (`h264.go:27`), `ProResResult`
  (`prores.go:27`) and `HAPResult` (`hap.go:28`) are three identical types.

- **One shared HTTP client removes ~50 missing status checks and ~8 missing timeouts.** There
  are four competing styles across eight clients:
  1. resty, no base URL, no timeout, the `StatusCode() != 200` check copy-pasted per call —
     `services/directus/client.go` (10×), `services/vizualizer/client.go` (4×),
     `services/clickup/client.go` (2×).
  2. resty with `SetBaseURL` and **no status check at all** — `subtrans`, `baton`, `vsapi`.
  3. resty with `SetError` + an `OnAfterResponse` hook — `services/cantemo/client.go:29-36`.
     This is the right shape and exactly one client uses it.
  4. raw `net/http` with a generic `doRequest[T]` — `services/rclone/requests.go:20-47` (no
     timeout) and `services/bmm/raven.go`'s `queryRaven[T]` (`:288-314`), which is the best
     code in the layer: ctx, timeout, `io.LimitReader` on error bodies, wrapped errors,
     generic decode. `filecatalyst/filecatalyst.go` is a fifth per-function variant.

  Proposal: a `services/internal/httpx` with
  `New(Config{BaseURL, Timeout, Retry, Auth, DecodeError}) *resty.Client` that installs the
  `OnAfterResponse` error hook. Then `directus`, `vizualizer`, `clickup`, `subtrans`, `baton`,
  `cantemo`, `vsapi` and `transcribe` all shrink to a `Config` literal and each gains the
  missing status check and timeout at once. Keep `rclone`/`raven` on `net/http` but share
  `queryRaven`'s shape.

  Note one hazard to fix in passing: `vsapi/http.go:21-22` combines a 10 s timeout with
  `SetRetryCount(5)` on **non-idempotent** POST/DELETE (`AddShapeToItem`, `DeleteItems`,
  `CreatePlaceholder`), so a slow-but-successful Vidispine call currently creates duplicate
  shapes and jobs.

- **~400 lines removable from `workflows/vb_export/`.** `vb_export_bstage.go:25-88` and
  `vb_export_gfx.go:25-88` are byte-identical except five tokens (`"B-Stage"`/`"GFX"`, output
  folder name, `Interlace`, `Alpha`, and the `notifyExportDone` label). More broadly, the same
  destination-child preamble/postamble — logger, default activity options, `IsImage`, dest
  extension, `_SUB_NOR` suffix, rclone destination, `RcloneWaitForFileGone`, output dir,
  transcode, `RcloneCopyFileWithNotifications`, `notifyExportDone`, `return &VBExportResult{}`
  — appears at **ten** sites, and the literal
  `wfutils.RcloneWaitForFileGone(ctx, rcloneDestination, telegram.ChatOslofjord, 10)` occurs
  eleven times. Fix: one
  `vbExportChild(ctx, params, cfg, transcode func(...) (paths.Path, error))` wrapper — the
  pattern `vb_export_hippo_v2.go:37` already uses via `exportToHippoHAP`.

  Related inconsistency worth fixing at the same time: `VBExportResult.Title` is set in
  `raw_abekas.go:33`, `caspar_cg.go:33` and `abekas.go:81` but omitted in `bstage`, `gfx`,
  `hippo`, `hippo_v2`, `xdcam` and `hyperdeck`, so `VBExport`'s aggregated results have blank
  titles half the time.

- **One `internal/bootstrap` package for the four `cmd/` entrypoints.** Currently duplicated:
  Temporal `client.Dial` with `TEMPORAL_HOST_PORT`/`TEMPORAL_NAMESPACE` **4×**
  (`cmd/worker/main.go:90-93`, `cmd/httpin/main.go:256-259`, `cmd/trigger_ui/main.go:35-38`,
  `services/bmm/trigger.go:19-23`); `getQueue()` **2×**, both re-implementing
  `environment.GetQueue()`; `getTriggeredBy(ctx)` **2×** with the *same name and divergent
  semantics* (query param vs proxy header); `PORT` default + `Run` + error handling **3×** with
  three different defaults and two different error behaviours; `getFunctionName` **2×**
  (`cmd/httpin/main.go:290-309` and `activities/queues.go:43`, both lifted from Temporal SDK
  internals); the `IDENTITY` env lookup **2×**. A single package exposing `LoadEnv()`,
  `TemporalClient(cfg)`, `HTTPServer(r, port)` and `RunUntilSignal(srv)` removes ~120
  duplicated lines and simultaneously fixes the missing server timeouts, graceful shutdown,
  `gin.ReleaseMode`, and the dead `.env` files listed in Tier 5.

- **Inline code that an existing helper already covers.**

  | Inline code | Existing helper |
  | --- | --- |
  | `misc/handle_multitrack.go:65-79`, `export/vx_export_vod.go:440-449`, `ingest/sync_fix.go:109-119` — hand-rolled copy + wait-for-job | `wfutils.RcloneCopyFile` / `RcloneCopyFileWithNotifications` |
  | `export/isilon_export.go:72-77` and `export/vx_export.go:218-223` — hand-built `Export/<YYYY-MM>/<title>-<runID>` paths that **disagree** (one uses the full run ID, the other `id[0:8]`) | `wfutils.GetWorkflowIsilonOutputFolder` (`utils/workflows/files.go:151`) |
  | `misc/masv_import.go:141,173`, `ingest/import_subtitles.go:113` — raw `json.Unmarshal`/`MarshalIndent` | `wfutils.UnmarshalJson` / `MarshalJson` (`utils/workflows/encoding.go:22,34`) |
  | `transcode/preview.go:99-104,227-234,343-347`, `vidispine/export.go:442-455` — four ad-hoc "filter streams by CodecType" loops | `FFProbeResult.AudioStreams()` (`services/ffmpeg/probe.go:303`) |
  | `export/vx_export.go:245-263` and `vb_export/vb_export.go:267-286` — identical result-collection loops | a generic `CollectChildResults[T]` over the existing `wfutils.ResultOrError[T]` (`utils/workflows/common.go:11`) |
  | abandon-child-options boilerplate at 6 sites (`ingest/masters.go:106,204`, `ingest/common.go:114,138`, `ingest/import_audio_from_reaper.go:36`, `utils/workflows/execute.go:98`) | a new `wfutils.WithAbandonChildOptions(ctx)` |
  | `ingest/common.go:112-134` and `:136-156` — `createPreviewsAsync` and `transcribe` have the same structure | one generic `startChildPerAsset[T]` |
  | `services/directus/client.go:206-213` — 8-line linear search for a valid style | `slices.Contains` |
  | `misc/watch_folder_transcode.go:66-131` — eight copy-pasted transcode dispatches differing only in activity and encode params | a `map[string]struct{act; params}` table |
  | `misc/watch_folder_transcode.go:36-64` — five identical folder-create blocks | a loop over `[]string{"processing","out","tmp","error","processed"}` |
  | `scheduled/files_cleanup.go:23-101` — `workflow.Now(ctx).Add(-14*24*time.Hour)` written **60 times** as map values | hoist to one variable + a `[]string` of folders, which also removes the pointless `GetMapKeysSafely` `SideEffect` at `:105` |

- **Layering inversion.** `common/merge.go:5` imports `services/vidispine` for
  `vidispine.AudioStream`, so the dependency chain is
  `services/transcode → common → services/vidispine`: every ffmpeg wrapper transitively
  depends on the Vidispine client. Move `AudioStream`/`ChannelID` into `common` (or a `media`
  package) and have `vidispine` depend on that instead. Separately,
  `common/transcode.go:36` declares an **unexported** field (`offset time.Duration`) in a
  Temporal activity input struct — it can never round-trip the data converter, so it is either
  dead or broken; delete it.

- **`language_config.go` should be data, not Go.** 478 of its 620 lines are a hand-written
  literal whose authoritative source is a cited Notion page (`:141`), hand-transcribed with no
  diff, no schema check and no CI validation — which is exactly what produces the misspellings
  and key-set gaps listed in Tier 5. Move the table to an embedded `languages.json`/`.csv`
  (`gocarina/gocsv` is already a direct dependency) loaded via `//go:embed`, and add a
  `TestLanguageConfigConsistency` asserting the invariants. That leaves ~140 lines of logic and
  makes the Notion→repo sync reviewable as a data diff.

  While moving it, move both root files into a `languages/` package. `package bccmflows` at the
  module root forces a `bccmflows "github.com/bcc-code/bcc-media-flows"` alias in 15 files
  (because the package name differs from the last path element) and is one added import away
  from a cycle: the root package is imported by `utils`, `services/*`, `workflows/*` and
  `activities/*`, and it stays acyclic today only because it imports nothing but `merry`.
  Since `paths → environment` already exists, a root → `paths` import would be the trap.

- **Functions over 100 lines that decompose naturally.**
  `ingest/incremental_ingest.go:71` `doIncremental` (276 — splits cleanly into
  setup/placeholder, preview coroutine, copy+signal loop, post-processing);
  `export/generate_short.go:43` (188); `export/vx_export.go:74` (192);
  `export/vx_export_vod.go:30` (187); `misc/masv_import.go:65` (151);
  `export/vx_export_bmm.go:66` (150); `ingest/sync_fix.go:21` (144);
  `export/merge_export_data.go:34` (131 — `:101-133` is the same collect-results block twice);
  `misc/watch_folder_transcode.go:23` (125); `export/shorts.go:232` (99);
  `ingest/masters.go:281` `addMetaTags` (98 — a `[]struct{field, value string}` table + loop).

---

## Tier 4 — Security hardening (defense-in-depth)

The HTTP services sit behind an identity-aware proxy, which is the control today. Everything
below is the in-process defense that should back it up, roughly in order of what would hurt
most if the proxy were ever misconfigured, bypassed, or reached from an already-trusted host.

- **`POST /trigger/ExecuteFFmpeg` grants arbitrary `ffmpeg` argv execution.**
  `cmd/httpin/main.go:173-183` → `workflows/misc/ffmpeg.go:24` → `activities/transcode.go:322`,
  where `input.Arguments` reaches `ffmpeg.Do(...)` unvalidated. ffmpeg is a file-read,
  file-write and network primitive (`-i /mnt/isilon/anything -f data -y /mnt/isilon/anywhere`,
  `-f rtp rtp://…` for exfiltration, `-protocol_whitelist file,http,tcp`), so anyone past the
  proxy has a general-purpose read/write/exfil primitive on the transcode fleet. Recommend
  deleting the case, or gating it on an admin-only credential plus an argv allowlist. This is
  the single highest-value change in this tier.

- **No in-process authn/authz anywhere.** The only `r.Use()` in all of `cmd/` is
  `cors.Default()`; `grep -riE "BasicAuth|Authorization|jwt|authenticat" cmd/` returns nothing
  else. That leaves 23 of 24 routes with no check, including state-changing and administrative
  ones: `POST /move-files` (`cmd/trigger_ui/main.go:556`), `POST /upload-master/admin` (`:543`,
  which mutates the SQLite program-ID catalog including deletes), the export triggers, and
  `POST /filecatalyst` (`:520`) + `POST /webhook/filecatalyst` (`:524`) which signal the
  fixed-ID `LIVE-INGEST` workflow. `cmd/trigger_ui/main.go:52-54` shows the proxy was
  anticipated, but `TRIGGERED_BY_HEADER` feeds only the audit label, never access control.

- **The audit trail is attacker-controlled in `httpin`.** `cmd/httpin/main.go:32-38` reads
  `triggeredBy` from a POST form or query parameter, so the attribution stamped on every
  workflow can be set to any value. `trigger_ui` uses a proxy header instead, which is the
  right idea — but `TRIGGERED_BY_HEADER` is unset in practice (see the dead `.env` finding in
  Tier 5), so everything is attributed to the constant `"trigger-ui"`. Derive `triggeredBy`
  from the authenticated identity only.

- **`cors.Default()`** (`cmd/httpin/main.go:266`) sets `AllowAllOrigins = true`, i.e.
  `Access-Control-Allow-Origin: *`, over `GET POST PUT PATCH DELETE HEAD OPTIONS`. Because
  there is no in-process auth, this is not merely "credentials aren't sent" — any page an
  operator visits can drive `GET /schemas` and `GET/POST /trigger/:job` cross-origin **and read
  the responses**. Nothing in-repo needs CORS; either drop the middleware or use an explicit
  `AllowOrigins` allowlist. (`cmd/trigger_ui` correctly has no CORS middleware.)

- **`GET /schemas` + `POST /trigger-dynamic` are a self-documenting reflection RPC surface.**
  `cmd/httpin/jsonschema.go:22` publishes the full JSON Schema of every param struct and
  `:51-114` executes any workflow found in `TriggerableWorkflows` by name — including the
  destructive `scheduled.CleanupTemp` and `scheduled.MediabankenPurgeTrash`. At minimum, drop
  the scheduled entries from the triggerable set.

- **No CSRF protection, and state-changing operations available over `GET`.**
  `grep -i "csrf\|_token" cmd/trigger_ui/templates/*.gohtml` returns nothing across all 10
  forms. And `r.GET("/trigger/:job", triggerHandler)` (`cmd/httpin/main.go:269`) makes every
  workflow trigger reachable from an `<img src="…/trigger/ExportAssetVX?vxID=…">`;
  `cmd/httpin/trigger.html:7` uses `method="get"`, so this is deliberate rather than
  accidental. Once auth exists, add `SameSite=Strict` plus a per-session token, and remove the
  `GET` registration.

- **Path traversal from form input.** `cmd/trigger_ui/masters.go:156-157` joins a form field
  into `masterTriggerDir`; `filepath.Join` cleans `..`, but `paths.Parse`
  (`paths/paths.go:235-251`) only checks that the result carries a known drive prefix — it does
  not confine to `masterTriggerDir`. And `MASTER_TRIGGER_DIR` has no default
  (`cmd/trigger_ui/main.go:50`), so unset means `filepath.Join("", p)` and the caller controls
  the path outright; the result becomes the `Masters` ingest `SourceFile`. Worse,
  `getOverlayFilePath` (`cmd/trigger_ui/main.go:78-80`, used at `:284-287`) and
  `cmd/httpin/main.go:168` apply no validation at all, feeding an arbitrary `WatermarkPath`
  into `export.VXExportParams` and thence to ffmpeg. Fix: require
  `strings.HasPrefix(filepath.Clean(joined), filepath.Clean(root)+string(os.PathSeparator))`,
  and validate the basename against the file list already rendered by `getFilenames(root)`.

- **Self-update executes unverified release binaries.** `cmd/worker/main.go:263-296` calls
  `selfupdate.DetectLatest`/`UpdateTo` with **no `selfupdate.Validator`**, so there is no
  checksum or signature verification — the asset is trusted on TLS and GitHub alone — and
  `:80-88` applies it every 5 minutes with no rollout gate. Combined with `publish.yml`
  auto-tagging on every master push, anyone able to publish a release reaches the whole fleet
  within minutes. Add a `ChecksumValidator` over a signed `checksums.txt`, and prefer
  operator-triggered updates; running both this *and* `deploy-images.yml` is itself the
  problem.

- **XML injection into Vidispine metadata documents.**
  `services/vidispine/vsapi/xml_templates.go:5` imports `text/template` — not `html/template`
  and not `encoding/xml` — and interpolates user-controlled values raw: `{{ .Title }}` at
  `:22` and `:63` (asset titles from the ingest order form) and `{{.Key}}`/`{{.Value}}` at
  `:76,78,80` (arbitrary metadata via `SetItemMetadataField`/`AddToItemMetadataField`). A title
  containing `&`, `<`, or `</value><field><name>portal_mf…</name><value>x` produces malformed
  XML or writes fields the caller never intended. The correct pattern already exists in the
  same package — `vsapi/search.go:40-58` marshals with `encoding/xml`.

- **Auth tokens and payloads written to logs.** `services/cantemo/client.go:65` leaves
  `req.SetDebug(true)` on in `GetMetadata`, so resty dumps all request headers — including the
  `Auth-Token` set at `:26` — plus the full response body to stdout on every metadata fetch.
  Same class: `services/transcribe/transcribe.go:208` (`Debug = true`, full transcript
  payloads), `services/vidispine/vsapi/shapes.go:84` (`spew.Dump` on every `AddShapeToItem`),
  `services/baton/tasks.go:22` (`print(res.Body())`), and `utils/execute_command.go:19` — a
  builtin `println("FFMPEG Command:", cmd.String())` that logs the **full argv of every
  command** through a generic runner, which for FTP/rclone-adjacent invocations risks printing
  credentials.

- **Hardcoded credentials and endpoints.** `services/clickup/client.go:20-24` embeds
  `defaultToken = "ddef98b0bd69a81"` plus workspace/view IDs (the comment argues it is a public
  share token; it is still an unrotatable credential in git history, and it is duplicated into
  `cmd/worker/.env.example`). Internal infrastructure hardcoded with no env override:
  `services/rclone/upload.go:12` (`http://rclone.lan.bcc.media`, and `requests.go:26-27` sends
  **HTTP Basic auth over plaintext HTTP**), `services/transcribe/transcribe.go:16`,
  `activities/reaper.go:13`, `activities/baton/client.go:6`, `cmd/worker/main.go:194`,
  `paths/paths.go:167`, `services/emails/send.go:19`. And `cmd/httpin/trigger.html:7`
  hardcodes `action="https://temporal-trigger.lan.bcc.media/trigger/ExportAssetVX"`, so a
  staging or dev `httpin` serves a form that fires at **production**.

- **Fail-open webhook key.** `cmd/trigger_ui/workflows.go:53-59` only checks the `api-key`
  header when `MASSIVE_WEBHOOK_API_KEY` is non-empty, so an unset variable means no
  authentication at all — and given nothing loads `cmd/trigger_ui/.env` (Tier 5), unset is the
  default posture. The comparison is also not constant-time. Fix: refuse to start when the key
  is empty; use `subtle.ConstantTimeCompare`.

- **Full workflow history served on an unauthenticated page.**
  `cmd/trigger_ui/workflows.go:387-397` marshals `resp.History` (discarding the marshal error)
  and `templates/workflow-details.gohtml:65` renders it in a `<pre>`. That history contains
  every workflow and activity input/output payload: filesystem layouts, `SenderEmail` from
  master uploads (`masters.go:169`), Vidispine IDs, destination configs. The structured
  `Activities` timeline above it already covers the operator need — drop the raw dump, or gate
  it behind an admin role.

- **Query injection into the Temporal visibility query.** `cmd/trigger_ui/workflows.go:371-373`
  builds `fmt.Sprintf("ParentWorkflowId='%s'", workflowID)` from `ctx.Param("id")`. Impact is
  limited, but reject IDs not matching `^[A-Za-z0-9._:-]+$` before use.

- **Third-party CDN script in all 13 admin templates, no SRI, no CSP.** Every
  `cmd/trigger_ui/templates/*.gohtml` loads `https://cdn.tailwindcss.com` at `:4-6`. A CDN
  compromise yields JS execution in the admin UI that drives the ingest pipeline (and
  Tailwind's CDN build is explicitly not for production). Vendor a built CSS file into the
  image — the Dockerfile already copies the templates directory — and add a
  `Content-Security-Policy` header.

- **No server timeouts, size limits, graceful shutdown, or release mode.**
  `cmd/httpin/main.go:287`, `cmd/trigger_ui/main.go:571` and `cmd/fakerclone/main.go:132` all
  call `gin.Engine.Run`, which uses a zero-value `http.Server`: no `ReadTimeout`,
  `WriteTimeout`, `ReadHeaderTimeout` or `IdleTimeout`, so Slowloris trivially exhausts
  connections. No `MaxBytesReader` anywhere, while `ctx.BindJSON` reads unbounded bodies at
  `watchers.go:32,211`, `httpin/main.go:177` and `jsonschema.go:71`. No
  `signal.NotifyContext` + `srv.Shutdown`, so in-flight requests are cut on every rolling
  deploy — and `cmd/httpin/main.go:263`'s `defer temporalClient.Close()` never executes
  because `r.Run` blocks until the process is killed. `grep -rn "SetMode\|GIN_MODE"` returns
  nothing repo-wide, so all three services run in **gin debug mode**, logging every request
  line including query strings carrying `vxID`, `file`, `watermarkPath` and `destinationPath`.
  Finally, `cmd/httpin/main.go:287` discards `r.Run`'s error, so a bind failure makes the
  process **exit 0** and the orchestrator may never restart or alert.

- **ffmpeg filter injection from filenames.** `services/transcode/multitrack.go:12-21,47`
  joins `f.Base()` values with newlines and interpolates them unescaped into a `drawtext`
  filter, where `:`, `'`, `\`, `%`, `,` and the newlines themselves all terminate or alter the
  filter graph. Same class, lower risk: `merge.go:294,404` build `concat:a|b|c` from paths (a
  `|` in a filename breaks it) and `merge.go:277` writes `file '%s'` concat lists without
  escaping `'`. Note there is **no shell involved** — `exec.Command` is used correctly
  throughout — the injection is into ffmpeg's own filter grammar. Fix: escape per ffmpeg rules
  or use `textfile=` with a temp file.

- **`cmd/httpin/watchers.go`.** Worth correcting a common assumption: this is a **webhook
  receiver**, not a filesystem watcher, so there is no polling race or goroutine leak. The real
  issues are: seven production mount paths hardcoded in the dispatch chain at `:41-54`, none of
  which honour `environment.GetIsilonPrefix()`/`GetFileCatalystMountPrefix()`, silently
  bypassing the local-testing override the rest of the codebase respects; the `else` branch at
  `:72-74` routes **any unrecognised path** to transcode; `:141` interpolates
  `TRANSCODE_ROOT_PATH` **unescaped** into a package-init `regexp.MustCompile`, so an unset
  variable degrades the pattern to `(?:/)([\w-]*)(?:/in/)` and any regex metacharacter silently
  breaks it; only the `doRawImport` branch calls `paths.Parse`, while the request-supplied path
  flows unvalidated into `IncrementalParams.Path`, `CopyFileInput.Source` and
  `AssetJSONParams.JSONPath`; and `:132` uses the
  fixed workflow ID `"LIVE-INGEST"`, so two concurrent growing-file events collide and the
  resulting `WorkflowExecutionAlreadyStarted` is returned as a 500 with no dedup logic.

---

## Tier 5 — Conventions, cleanup, dead code

### Configuration

- **73 non-test `os.Getenv` sites across 55 variables, and exactly one validated at startup**
  (`cmd/trigger_ui/main.go:487-490`, for `TRIGGER_DB`). Every other required secret fails
  **late** — at first use, deep inside a Temporal activity, as an opaque 401 or connection
  error, potentially hours into a pipeline. Examples: `VIDISPINE_USERNAME`/`PASSWORD`
  (`cmd/trigger_ui/main.go:478`) let the service start and then 500 on every export page;
  `SUBTRANS_API_KEY`, `SENDGRID_API_KEY`, `TELEGRAM_BOT_TOKEN`, `DIRECTUS_API_KEY`,
  `FILECATALYST_PASSWORD`, `PLAYOUT_FTP_PASSWORD` are all silently empty until reached; and an
  unset `TEMPORAL_HOST_PORT` makes `client.Dial` fall back to the SDK default rather than
  failing loudly. Fix: one `environment.Config` struct with `required` tags, loaded once per
  `main`, reporting **all** missing variables at once — and keep `environment/` as the only
  package that touches `os.Getenv`.

- **`godotenv` is loaded in only 2 of 4 entrypoints, so half the `.env` files are dead.**
  `cmd/worker/main.go:70` and `cmd/bmm-trigger/main.go:18` load it;
  `cmd/httpin/.env`, `cmd/httpin/.env.example`, `cmd/trigger_ui/.env`,
  `cmd/trigger_ui/.env.example` and `cmd/trigger_ui/example.env` all exist on disk and
  `cmd/trigger_ui/readme.md` implies env-file config, but **nothing loads them**. This is
  precisely why `TRIGGERED_BY_HEADER` and `MASSIVE_WEBHOOK_API_KEY` end up unset in practice.
  `cmd/worker/main.go:70-73` also inverts the useful convention — it logs on success and stays
  silent on failure.

- **Env read at package-var/`init()` time** in `cmd/trigger_ui/main.go:49,50,54`,
  `cmd/trigger_ui/vb.go:23`, `cmd/httpin/watchers.go:21`, `environment/environment.go:8,45,55,65`,
  `activities/shorts.go:16`, `services/ftp/playout.go:8-10`, `services/rclone/requests.go:12-13`,
  `services/telegram/chats.go:27`, and `workflows/vb_export/vb_export.go:107`. Two consequences:
  `godotenv.Load` can never affect these, and `t.Setenv` in tests has no effect because the
  value is already frozen. The `vb_export.go:107` case is also a replay hazard — a redeploy or
  a fleet with differing values changes the computed `SubtitleStyle` path.

- **`QUEUE` is read in four places with three different fallbacks:**
  `environment/environment.go:8-15`, plus byte-identical local `getQueue()` reimplementations
  in `cmd/httpin/main.go:47-53` and `cmd/trigger_ui/main.go:41-47`, plus a different fallback
  in `services/bmm/trigger.go:26`. And `environment/environment.go:17-43` repeats the same
  5-line `queue == QueueDebug` check across five getters.

- **`.env.example` drift.** `cmd/trigger_ui/` has both `.env.example` *and* `example.env` with
  contradictory contents (`localhost:7233` vs `127.0.0.1:7233`; each documents variables the
  other omits). `cmd/worker/.env.example:2` documents `TEMPORAL_ADDRESS` while the code reads
  `TEMPORAL_HOST_PORT` (`cmd/worker/main.go:91`), and gives it as a URL when `client.Dial`
  wants `host:port` — an operator following the docs gets a worker silently dialling
  `localhost:7233`. `DEBUG=true` is documented in three files and read by **no code**.
  `PORT=8081` is documented while the code defaults to `8083`. No `.env.example` exists for
  `cmd/bmm-trigger` despite its eight required variables. `cmd/worker/readme.md` documents four
  queues while `environment/queues.go:3-10` defines six (`debug` and `live` undocumented).

- **Templates loaded from a CWD-relative path.** `cmd/trigger_ui/main.go:485` calls
  `LoadHTMLGlob("./templates/*")`, which works only because the Dockerfile copies templates to
  `/templates` with `WORKDIR /` — so `go run ./cmd/trigger_ui` from the repo root panics at
  startup. `cmd/httpin/main.go:251-252` already does the right thing with `//go:embed`.

### The language table

- **`""` collides across three languages, and `ParseLanguageCode("")` returns Swahili.**
  `language_config.go:110-116` (`ByISO6392TwoLetter`) has **no empty-string guard**, while
  `yue` (`:489`), `kha` (`:551`) and `swa` (`:566`) all have `ISO6392TwoLetter: ""`. All three
  write key `""` and slice order makes Swahili the winner. So `language_parser.go:13` misses
  `LanguagesByISO[""]` but then *hits* `LanguagesByISOTwoLetter[""]` and returns Swahili, and
  `workflows/export/vx_export_bmm.go`'s `if lang, ok := bccmflows.LanguagesByISOTwoLetter[lang]; ok`
  succeeds for empty input. Every sibling builder guards its sentinel (`BySoftron:48`,
  `ByMU1:75`, `ByMU2:89`, `ByReaperChan:102`, `ByBMMCode:121`) — `ByISO6392TwoLetter` and
  `ByISO6391` are the two that don't.

- **An unknown language key becomes a zero-value `Language` that impersonates Norwegian.**
  `utils/languages.go:9-17` maps keys through `bccmflows.LanguagesByISO[key]`, and the comment
  "Will panic" is simply wrong — a Go map miss returns `Language{}`, whose
  `LanguageNumber: 0` **is Norwegian's** (`language_config.go:144`), with empty `ISO6391`,
  `MBPreviewTag` and `RelatedMBFieldID`. `Less` compares only `LanguageNumber`, so unknown
  languages sort first, tied with real Norwegian, then flow into
  `workflows/export/mux_files.go:23,53` and `services/transcode/mux.go:26` as a silently empty
  audio/subtitle track. Fix: return `([]Language, error)` or `lo.FilterMap` with the `ok` form;
  delete the misleading comment either way.

- **`ISO6391` and `ISO6392TwoLetter` hold each other's formats.** `ISO6391` contains
  *three*-letter codes (`"nor"`, `"deu"`, `"nld"` — ISO 639-2/T) and `ISO6392TwoLetter`
  contains *two*-letter codes (`"no"`, `"de"`, `"nl"` — ISO 639-1). Both field names assert the
  opposite of their contents, and `LanguagesByISO` inherits the confusion. This is the most
  likely source of future integration bugs in the file. `ISO6392TwoLetter` is also overloaded
  with a third format: `"no-x-tolk"` (`:472`) and `"und-x-ai-generated"` (`:611`) are BCP-47
  private-use subtags, not two-letter codes.

- **Two different "not applicable" sentinels.** 27 of 29 entries use `-1`; the AI-generated
  entry (`LanguageNumber: 100`) uses `-2` at `:613-615` while its `SoftronStartCh` at `:617` is
  `-1`. All consumers test `< 0`, so behaviour is identical today, but any future `== -1` check
  silently mishandles that entry. Relatedly, `SoftronStartCh` runs `0,2,4,…,44,48,50,52,54` —
  **46 is skipped**, undocumented, between Tamil (`:526`) and Estonian (`:541`).

- **Six languages can never be produced by MU1/MU2 extraction.** `mal`, `tam`, `est`, `kha`,
  `swa` and `afr` (numbers 21-26) have `MU1ChannelStart: -1` *and* `MU2ChannelStart: -1`
  (`:509-510,524-525,539-540,554-555,569-570,584-585`) while carrying valid `ReaperChannel`
  22-27 and `SoftronStartCh` 42-54, so `workflows/ingest/mu1_mu2_extract.go` silently skips
  them. Either intentional — in which case document it — or a transcription gap. (The MU1 and
  MU2 channel maps themselves are clean: no collisions in either, and Norwegian is deliberately
  in both.)

- **`MU1ChannelCount` is inconsistent with Softron spacing.** `nor`/`deu`/`nld`/`eng` use
  `MU1ChannelCount: 2` while `fra` through `pol` use `1` (`:222,239,256,273,290,307,324,341`),
  yet all of them get two Softron channels, leaving MU1 channels 17+ unallocated. Possibly
  correct (mono vs stereo feeds), but nothing states it.

- **`LanguageByBMM["no"]` resolves to the interpreter track, not Norwegian.** `:150` gives
  Norwegian `BMMLangauageCode: "nb"`, while `:473` ("Norsk tolk") gets `"no"`. Any BMM
  integration passing the obvious `"no"` gets the interpreter feed. Deserves a comment at
  minimum. The field name is also misspelled ("Langauage") at `:10` and referenced by that
  misspelling at `:121`, `:124` and in all 29 literals, and its values are stylistically mixed
  (2-letter for 24 entries, but `"hun"`, `"yue"`, `"kha"`, `"zxx"`).

- **Placeholder and misspelled strings reach the production UI.** These render into
  `vx-export.gohtml`/`isilon-export.gohtml` via `TriggerGETParams.Languages`
  (`cmd/trigger_ui/main.go:123,235`): `:469`
  `LanguageNameNative: "Forstår du ikke hva jeg sier?"` is a joke placeholder in the language
  picker; `:163` `"Deutch"` → `Deutsch`; `:180` `"Nederland"` → `Nederlands`; `:231`
  `"Española"` → `Español`; `:502` `"Maylaisisk"` + `:504` `"Malayallam"` → Malayalam (and
  "Maylaisisk" reads as *Malay*, a different language, though `ISO6391: "mal"` is correctly
  Malayalam). `LanguageNameNative` is empty at `:563`, `:578` and `:593`, which renders as
  blank options.

### Project conventions (`CLAUDE.md`)

- **`.Result(ctx)` over `.Get(ctx, &x)`:** 93 `.Get(ctx` call sites remain, e.g.
  `workflows/ingest/multitrack.go:60,75`.

- **`github.com/orsinium-labs/enum` over string constants.** The codebase already does this
  well in 11 places (`paths/paths.go:32`, `workflows/export/vx_export.go:21`,
  `workflows/vb_export/vb_export.go:24`, `workflows/ingest/asset_ingest.go:19`,
  `services/vidispine/export.go:74`, `services/telegram/chats.go:11`,
  `services/transcode/hap.go:13`, `services/rclone/upload.go:18`, `services/baton/plans.go:5`,
  `utils/tc_samples.go:11`, `workflows/export/isilon_export.go:25`); these are the stragglers:

  | Site | Strings |
  | --- | --- |
  | `common/codecs.go:4-11` | `FolderProRes422HQHD`… untyped consts, switched on in `misc/watch_folder_transcode.go:67` |
  | `export/generate_short.go:162,168` | `"completed"` / `"in_progress"` |
  | `activities/vidispine/client.go:50,54,78,81` | `"FINISHED"`, `"STARTED"`, `"READY"`, `"WAITING"` |
  | `export/shorts.go:265,318` | `Type: "short"`, `Status: "draft"` |
  | `ingest/bmm_simple_upload.go:110` | `"bmm-int"` — should be `export.AssetExportDestinationBMMIntegration` |
  | `export/vx_export_bmm.go:364-367` | `"aac"` / `"mp3"` |
  | ~25 sites | bare shape tags: `"original"`, `"lowres"`, `"lowres_watermarked"`, `"lowaudio"`, `"transcription_json"`, `"Transcribed_Subtitle_SRT"`, `fmt.Sprintf("sub_%s_srt")` |
  | `misc/transcribe-vx.go:16` | `transcriptionMetadataFieldName = "portal_mf624761"` while neighbours use `vscommon.Field*` |
  | `misc/slow_move_files.go:79-130` | `Storages []MBStorage` with string VXIDs + linear scans |

  Related: `string` used where an enum type already exists —
  `export/vx_export.go:42` `Destinations []string` (with a `jsonschema:"enum=…"` tag
  duplicating `AssetExportDestinations` declared 15 lines above), `:43` `AudioSource string`
  → `vidispine.ExportAudioSource`, `vb_export/vb_export.go:83` `Destinations []string`, and
  `export/isilon_export.go:20,22`.

- **Error handling: four competing conventions coexist.** (1) merry sentinels + `merry.Wrap` +
  `WithHTTPCode` in `paths`, `rclone`, `cantemo`; (2) `errors.New` sentinel + `%w`
  (`vsapi.ErrShapeTagNotFound`, `transcode.ErrUnknownAudioChannelFormat`); (3) bare
  `fmt.Errorf` with **no `%w`** — the majority, and some drop the cause entirely
  (`mux.go:107`, `mux_mxf_simple.go:61`, `playout_mux.go:240`, `directus/client.go` ×10,
  `ffmpeg/probe.go:326` interpolates `err.Error()` with `%s`); (4)
  `temporal.NewNonRetryableApplicationError` (`paths/paths.go:247-252`, sole site). With only
  18 merry call sites, dropping merry for stdlib `errors`/`%w` plus
  `NewNonRetryableApplicationError` at the activity boundary is the smaller migration; if merry
  stays, every `fmt.Errorf` still needs `%w`.

  Good news worth protecting: **zero** string-matched error comparisons
  (`strings.Contains(err…)` and `err.Error() ==` both return nothing).

  Roughly 10 sites stringify a slice of errors instead of joining it — `export/shorts.go:101`,
  `ingest/file_move.go:48`, `ingest/incremental_ingest.go:340`, `ingest/masters.go:196-201`
  (`lo.Reduce` string concat), `ingest/mu1_mu2_extract.go:156-165`, `misc/transcribe-vx.go:103`,
  `ingest/import_subtitles.go:154`, `export/vx_export.go:260`, `vb_export/vb_export.go:282`.
  Prefer `errors.Join`, which preserves `errors.As` — something
  `workflows/ingest/common.go:73-74` actually depends on for its `JOB_FAILED` detection.

  Also, `logger.Error("%w", err)` appears at `ingest/incremental_ingest.go:103,134,139,170,179`:
  `workflow.GetLogger` takes a message plus key/value pairs, so `"%w"` becomes the message and
  the error text is **never logged**. Same class with `logger.Info` at `export/shorts.go:63`
  and `:110`.

- **JSON tags on workflow payloads are inconsistent — and the choice is permanent.** These
  structs are serialized into workflow history, so adding or renaming a tag later breaks replay
  of in-flight workflows. Today: `ingest/bmm_simple_upload.go:19-28` and
  `bmm_track_metadata.go:33-51` tag every field in camelCase; `export/vx_export.go:38-50`
  (`VXExportParams`) tags none; `:60-70` (`VXExportChildWorkflowParams`) tags 2 of 9 in
  *snake_case*; `MergeExportDataParams`/`Result`, `CleanupResult` and `MasterParams` tag none;
  and `paths.Path` (`paths/paths.go:104-107`) has **no tags at all** despite crossing every
  activity boundary and relying on a custom `MarshalJSON` at `:35`. Settle on one convention
  before more workflows ship.

  Related naming inconsistencies: params suffixes are variously `…Params`, `…Input`, and bare
  (`ShortsData`); `export/shorts.go:108` `ExportShort(ctx, short *ShortsData)` is the only
  registered workflow taking a pointer and the only one without a params struct;
  `export/generate_short.go:29-30` has field `OutputDirPath` with tag `json:"OutputDir"`;
  `ingest/bmm_simple_upload.go:25` has field `BmmTargetEnvionment` (typo) with the correctly
  spelled tag; `misc/normalize_audio.go:12-22` shadows the identically-named
  `activities.NormalizeAudioParams`/`Result` and uses `FilePath string` instead of
  `paths.Path`; `ingest/bmm_track_metadata.go:33-65` duplicates 11 fields between
  `BmmTrackMetadataParams` and `bmmTrackMetadataPayload` (embed instead);
  `ingest/asset_ingest.go:52` and `ingest/bmm_simple_upload.go:30` are empty result structs.

- **`interface{}`/`any` where a type would do.** `export/vx_export.go:210` `var w interface{}`
  and `vb_export/vb_export.go:228` `var w any` for a workflow handle (both would be better as a
  package-level `map[Destination]any` lookup table — a keyed lookup, so still deterministic);
  `utils/workflows/execute.go:41` mixes `interface{}` and `any` in one file; and ~12 activities
  return `(any, error)` purely to satisfy `Execute`'s `func(ctx, T) (TR, error)` shape
  (`activities/files.go:137,154,196`, `activities/bmm.go:17`, `activities/pubsub.go:10`,
  `activities/transcode.go:314`, `activities/preview.go:76`, `activities/vidispine/items.go:15`,
  `activities/vidispine/files.go:185,205`, `activities/cantemo/{files,relations}.go`) — define a
  `type Void struct{}` or let `Execute` also accept `func(ctx, T) error`.

### Cleanup

- **`workflows/ingest/incremental_ingest.go` (`doIncremental`): `previewTempPath.Append("preview")`
  discards its return value** — `Append` returns a new path, so this line is a no-op and the
  preview temp files land directly in the workflow temp folder instead of a `preview`
  subfolder. _(previously logged)_

- **`workflows/export/shorts.go::createShortInPlatform` still iterates `tagCodes`** after the
  Notion→ClickUp migration, but `tagCodes` is now always empty (Source/Purpose/Type were
  dropped because ClickUp has no equivalents). Either re-introduce ClickUp custom fields for
  those tags (and wire them into `shortsDataFromClickUpTask`) or delete the loop entirely once
  it's clear we will not bring them back. _(previously logged)_

- **`workflows/export/shorts.go::triggerShortExport` hardcodes `Languages: []string{"nor"}`**
  after the Notion field was dropped. If shorts in other languages ever need to be exported,
  either add a `Language` custom field on the ClickUp "Shorts Export" list and reintroduce it
  on `ShortsData`, or derive it from the task name suffix (e.g. `..._NOR` → `nor`).
  _(previously logged)_

- **`workflows/export/shorts.go::ShortsData` fields `InHm`/`OutHm` are named after `HH:MM:SS`**,
  but ClickUp stores `MM:SS` (`convertToSeconds` already accepts both formats by prepending
  `"0"`). Consider renaming or commenting the expected formats. _(previously logged)_

- **Dead code** (`unused` linter plus manual cross-checking):
  - `language_parser.go` is **entirely dead** (31 lines: `ParseLanguageCode`,
    `MustParseLanguageCode`, `ErrLanguageParsingFailed`) — and it is the only thing that would
    surface the `""`→Swahili collision above as a silent wrong answer. Delete the file, or fix
    the collision first.
  - `LanguagesByNumber` (`language_config.go:24,35`) and `LanguageList.ByNumber()` (`:56-62`) —
    zero reads repo-wide.
  - `GetAnalyticsService` + `analyticsSvc` (`cmd/worker/main.go:61-65`) — `analyticsSvc` is
    never assigned, so this always returns nil, and it has no callers. The real accessor is
    `analytics.GetService()`.
  - `cmd/fakerclone/` (136 lines) — a stub rclone API, one commit from 2024-08-27, untouched
    since; in no image matrix, no release build, no test, no readme, and nothing imports it. If
    the local-dev capability is wanted it belongs as an `httptest.Server` fixture in
    `services/rclone/*_test.go`, where it would actually be exercised.
  - `workflows/ingest/masters.go:217` `analyzeAudioAndSetMetadata`;
    `workflows/ingest/multitrack.go:13-31` `channelSource`/`channelSources` and its whole
    `sort.Interface`; `services/transcribe/transcribe.go:21` `errNoLanguage` and the exported
    debug-only `DebugResponse` (`:48`); `common/transcode.go:36`;
    `workflows/export/prepare_files.go:14-24`; `fixedFiles` in
    `workflows/ingest/asset_ingest.go:79-86` (`SA4010` — the real mutation happens on the next
    line, so this is harmless but pointless); `fileByAssetID` in
    `workflows/ingest/raw_material.go:89,110`; `workflows/export/shorts.go:156,180`
    `watermarkPath := ""` then passed as `""`; `workflows/export/vx_export_vod.go:369`
    commented-out `DeletePath`.

- **Debug leftovers and commented-out code.** `utils/execute_command.go:14,26-32` — the
  abandoned concurrent-stderr goroutine, i.e. the fix for the `ExecuteAnalysisCmd` deadlock,
  commented out; `:19` the builtin `println` of every command's argv.
  `cmd/trigger_ui/isilon_export.go:76` — `spew.Dump(ctx.PostForm("exportFormat"))` on a live
  handler. `cmd/httpin/jsonschema.go:30-40` — three `fmt.Printf` debug loops per request.
  `utils/files.go:58` — commented-out `olderThan`. `cmd/trigger_ui/main.go:519` — a
  `// MD: This is a legacy route` comment on `POST /filecatalyst`, which is registered a second
  time at `:524` as `/webhook/filecatalyst`; both point at the same handler, so the legacy one
  can go now.

- **Awkward constructs.** `workflows/export/vx_export_vod.go:206-208`
  `for _, err = range service.errs { return nil, err }` → `errors.Join`.
  `workflows/misc/merge_import_subs.go:145` `langs = append(langs, lang)` appends to the slice
  currently being ranged at `:125` — a copy-paste from `import_subs.go:93` where `langs` is a
  separate accumulator; here it just corrupts the `GetMapKeysSafely` result for no reason (and
  the `if err != nil` at `:104-106` is placed *after* the loop that consumes `res`).
  `workflows/vb_export/vb_export_dubbing.go:18` shadows the package-level `deliveryFolder`, and
  `:21` logs `"Starting ExportToAbekas"` in the dubbing workflow.
  `workflows/vb_export/vb_export_hippo.go:79` accepts 25/50/**60** fps while its error at `:80`
  says "Expected 25 or 50", diverging from `hippo_v2.go:76`.
  `workflows/export/vx_export.go:240` and `vb_export/vb_export.go:262` re-apply
  `WithChildOptions` inside the destination loop when it is already applied at
  `vx_export.go:168`. `workflows/misc/create_thumbnails-vx.go:43` uses
  `vsactivity.Activities{}.CreateThumbnailsActivity` while every other site uses the
  `activities.Vidispine` singleton. `workflows/export/timedmetadata_vod.go:38` creates an output
  dir without `CreateFolder`, working only because `activities.Util.WriteFile` does
  `os.MkdirAll` — pick one convention.

- **Redundant work.** `VBExport` already computes `AnalyzeResult` and passes it in
  `VBExportChildWorkflowParams` (`vb_export.go:169,225`), but
  `vb_export_abekas.go:41-43` and `vb_export_hyperdeck.go:39-41` re-run the activity anyway
  (`bstage`, `gfx`, `hippo`, `hippo_v2` correctly use the passed value). And
  `workflows/misc/handle_multitrack.go:23,40` calls the `StandardizeFileName` activity twice
  for the same path.

- **Mutating deserialised inputs inside workflows.** `workflows/export/vx_export_vod.go:251,253`
  deletes from `mergeResult.AudioFiles`, which shares its map with `params.MergeResult`;
  `workflows/export/vx_export_bmm.go:110` writes into `params.MergeResult.AudioFiles`. Build a
  fresh output map instead.

- **Errors dropped with `_ =` or an unassigned call** — 45 `errcheck` findings, concentrated in:
  `workflows/ingest/common.go:190-216` (Telegram/email sends and their constructors),
  `ingest/masters.go:122`, `ingest/incremental_ingest.go:304,343`,
  `ingest/bmm_simple_upload.go:121`, `ingest/bmm_track_metadata.go:179`,
  `ingest/import_audio_from_reaper.go:158`, `ingest/raw_material.go:37,47`,
  `misc/merge_import_subs.go:65,66,147`, `misc/import_subs.go:54,95,126`,
  `misc/watch_folder_transcode.go:136,139,142`, `misc/handle_multitrack.go:23,40`,
  `vb_export/vb_export_dubbing.go:119,120`, `vb_export/vb_export_hippo.go:126`,
  `vb_export/vb_export_hippo_v2.go:108`, `export/vx_export_vod.go:315`,
  `export/vx_export_bmm.go:239,333-336` (`:335` returns a *partially built* `BMMData` on error),
  `misc/masv_import.go:117`, `misc/transcode_preview-vx.go:38`, `misc/create_thumbnails-vx.go:29`.

- **Panic-prone indexing and unvalidated input.**
  - `cmd/trigger_ui/main.go:289-315` and `cmd/trigger_ui/isilon_export.go:66-72` index
    `vsresolutions` with a form value parsed by `bccmUtils.AsInt`, which **discards the parse
    error**, so garbage becomes `0` and `"-1"` becomes `-1` → index-out-of-range (a 500 via
    gin's Recovery). Use `strconv.Atoi` with the error checked plus a bounds check.
  - `cmd/httpin/jsonschema.go:33` calls `reflect.Type.NumField()` with no
    `Kind() == reflect.Struct` guard, even though `triggerDynamicHandler:88-94` explicitly
    handles pointer params — so adding one pointer-param workflow to `TriggerableWorkflows`
    breaks `GET /schemas`.
  - `workflows/ingest/asset_ingest.go:228` `dirs[0]`, `:72` `s[:mid]` on an empty string;
    `workflows/export/vx_export.go:222` `id[0:8]`; `workflows/ingest/multitrack.go:80`
    `files[0]` after `ListFiles` with no emptiness check;
    `services/transcode/playout_mux.go:142-152` `leftStreams[lang][0]` with no length check
    (reachable because `playoutLanguages` uses ISO 639-2 while `AudioFilePaths` is keyed by
    639-1); `services/transcode/multitrack.go:23` `files[0]`;
    `services/ffmpeg/probe.go:92` `info.Streams[0]` — and note `if info != nil` at `:98` is
    checked *after* `info` was dereferenced at `:74`;
    `services/vidispine/vsapi/metadata.go:183` `val[0]` with no length check, even though the
    `Get` helper 150 lines up (`:26-34`) handles exactly that case.
  - `utils/tc_samples.go:19-44` divides by a caller-supplied `fps` with no zero check
    (`fps == 0` → integer divide-by-zero panic), and `:29` maps `NTSC` to `30` rather than
    30000/1001, so real 29.97 drop-frame material drifts ~0.1% — about 3.6 seconds over a
    one-hour programme.
  - `utils/files.go:114-119` `IsDirEmpty` returns `(true, nil)` on **any** I/O error including
    permission denied, and it feeds `GetEmptyDirs` (`:84-105`) which drives directory deletion.
    Should be `errors.Is(err, io.EOF)`.
  - `utils/files.go:36-47` — `ValidRawFilename` lowercases the extension at `:40` but `IsMedia`
    does not at `:45`, so camera-standard `.MOV`/`.MXF` files pass raw-material validation and
    are then not treated as media at `ingest/raw_material.go:113` and `ingest/masters.go:89`,
    silently skipping transcode/analysis.
  - `cache/store.go:56` `return i.Value.(*T)` — unchecked assertion on a process-wide flat key
    namespace; two callers using the same key with different `T` panic. Safe today only because
    `services/ffmpeg/probe.go` is the sole user.

- **`panic` used for control flow inside workflow code.** `workflows/export/vx_export_bmm.go:61`
  (`getBMMDestinationConfig`) and `:370` (`prepareBMMData`, unsupported audio format), both
  reachable from `VXExportToBMM`. A panic in workflow code fails the workflow *task*, which is
  retried indefinitely, so the workflow never terminates with a useful error. Return an error
  instead.

- **Resource leaks, races, and missing timeouts in the service layer.**
  - `services/rclone/jobs.go:59-72` builds one `*http.Request` outside the retry loop and
    reuses it, so retry #2+ POSTs an **empty body** and rclone cannot resolve the job. Rebuild
    inside the loop or set `GetBody`.
  - `services/rclone/requests.go:31` uses `http.DefaultClient` — **zero timeout**, no ctx — so
    an unresponsive rclone host hangs the activity indefinitely.
  - `services/transcribe/transcribe.go:210-211` sets `RetryWaitTime = 10` and
    `RetryMaxWaitTime = 30`, which as `time.Duration` are **10 and 30 nanoseconds**, not
    seconds. Its poll loop (`:232-250`) never checks `ctx.Done()`, never sets `SetContext(ctx)`,
    and never checks the POST's status code — so a 500 leaves `job.ID == ""` and it polls
    forever, pinning a worker slot until restart.
  - `services/transcode/preview.go:353-421` (`GrowingPreview`) never calls `tailCmd.Wait()` and
    never closes `pipe`, so each run leaks a zombie `tail` plus an fd; `ffmpegCmd` at `:355` is
    plain `exec.Command` rather than `CommandContext`, so it survives ctx cancellation. Also
    `:400` is an `SA4011` ineffective `break` inside a `select` — harmless today, but the loop
    never notices ffmpeg exiting, so it spins until ctx cancel.
  - `services/ftp/client.go:14-26` returns nil after a failed `Login` without `Quit()`ing the
    dialled connection.
  - `services/telegram/messages.go:40-56` lazily initialises a package-level `*telebot.Bot`
    with no synchronisation — a read/write data race plus duplicate bot construction across
    concurrent activities. Use `sync.OnceValues`.
  - `services/rclone/queue.go:22-35` leaves `ch` in `transferQueue[priority]` forever on its
    1-hour timeout (unbounded map growth, and dead channels block the scheduler's
    non-blocking send), and `:76` does `transferQueue[p] = transferQueue[p][started:]`, where
    `started` counts only *successful* sends — so live waiters get dropped and abandoned ones
    retained. `waitForTransferSlot` also takes no ctx, so a cancelled activity still blocks up
    to an hour.
  - `cache/store.go:44-49` starts an unstoppable janitor goroutine from `init()` — as a side
    effect of merely importing the package, including in tests and in `cmd/bmm-trigger` where
    it is useless — and `janitor.start()` (`:20-28`) is a `select` with a single case
    (`S1000`); `:65` hardcodes the 5-minute TTL so no caller can choose one, which also means a
    growing file re-probed within 5 minutes gets stale stream info.
  - `analytics/service.go:52-54` prints `"FATAL: …"` on `NewWithConfig` failure and then
    **returns the broken service anyway**, so every later call logs
    `"DEBUG: rudderstack client is nil"` once per activity; `newService` cannot report failure
    to its caller. `:39` uses `fmt.Printf` with no trailing newline, and `:67` re-reads
    `os.Getenv("IDENTITY")` on every `ActivityStarted`, duplicating
    `cmd/worker/main.go:112-115`.
  - `cmd/trigger_ui/main.go:252-281` fires a goroutine that writes
    `FieldExportAudioSource`/`FieldLangsToExport` while `:354` reads the same item and the
    workflow started at `:336`/`:344` reads the same fields — a genuine race, with all errors
    only `log.Println`'d so a failed metadata write is invisible to the user who just saw a
    success page. The goroutine also outlives the request with no context. Do the writes
    synchronously before starting the workflow, or move them into an activity.

- **`services/telegram/chats.go:19-51` builds an enum from zero-valued members.**
  `Chats = enum.New(ChatOslofjord, ChatVOD, ChatBMM, ChatOther)` runs at package-var init when
  all four are `{Value: 0}`, and `init()` at `:27-51` then mutates the originals. Since
  `enum.New` copies members into its internal map, `Chats.Members()/Contains()/Parse()` are
  permanently all-zero and mutually indistinguishable. It works today only because the single
  call site (`messages.go:67`) uses `Chats.Value(m)`, which never consults the enum — any
  future `Parse`/`Contains` will be silently wrong.

- **Other unhandled-error spots in the service layer.** `services/transcode/audio.go:390-400`
  `ExtractAudioChannels` builds `out := make(map[int]paths.Path)` and never writes to it, so it
  always returns an empty map (currently harmless — `activities/audio.go:189` discards the
  return and rebuilds the map itself — but it is a trap).
  `services/vidispine/export.go:611-626` `convertFromClipTCTimeToSequenceRelativeTime` aliases
  `out.Terse[name]` to the caller's `[]*MetadataField` and writes through it, mutating the
  original chapter timecodes (two calls compound the offset) — deep-copy instead.
  `services/vidispine/export.go:486` `if str := meta.Get(FieldBmmTitle, ""); strings.TrimSpace(title) != ""`
  tests the outer `title`, not `str`, so an asset with a title but no BMM-title field gets
  `BmmTitle = ptr("")` instead of nil. `services/vidispine/export.go:575-582` calls
  `allSubLanguages.Add("und")` outside its nil guard, so **every** clip gets an `und` entry
  pointing at `EmtpySRTFile` (`:595`) and a bogus empty subtitle track in the mux.
  `services/vidispine/vsapi/placeholder.go:38` discards `tpl.Execute`'s error while
  `xml_templates.go:50` references a `{{ .Email }}` field that `PlacholderTplData` (`:23-25`)
  does not have — currently latent because only `PlaceholderTypeRaw` is called, but it would
  PUT truncated XML. `services/transcode/subtitles.go:98-151` `specialASSConverter` ignores
  every `WriteString` error and never checks `scanner.Err()`, and its `return err` at `:150`
  returns the stale nil from `os.Create` — a truncated `.ass` (disk full, or a line >64 KiB)
  surfaces as success and gets burned into the video.
  `services/subtrans/client.go:57-60` `stripBOM` uses `bytes.Trim(b, "﻿")`, which takes a
  *cutset of runes* and so strips U+FEFF from both ends repeatedly; it should be
  `bytes.TrimPrefix`. `services/subtrans/client.go:31` also concatenates a caller-supplied name
  into the URL path with no `url.PathEscape`.
  `paths/paths.go:192-200` `Prepend` writes into the variadic backing array, so a caller doing
  `p.Prepend(mySlice...)` has `mySlice` silently rewritten.

- **Testing.** Coverage numbers are in the Process section above. Structural gaps and smells:
  - **Only one interface exists in the whole service layer** — `vidispine.Client`
    (`services/vidispine/service.go:9-46`), and it is correctly placed on the consumer side and
    properly generated (`//go:generate mockgen` at `:1`, `go.uber.org/mock`). Every other client
    is a concrete `*Client`, so `directus`, `cantemo`, `clickup`, `baton`, `subtrans`, `rclone`,
    `transcribe` and `filecatalyst` are untestable without network — which is exactly the set
    with no tests. Replicating the `vidispine.Client` pattern is what unblocks coverage there.
    (`vidispine.Client` itself is inconsistent: only `DeleteItems` takes a `context.Context`;
    the other 20 methods don't. Adding ctx uniformly would also fix the missing timeouts.)
  - `services/transcode/testdata/generated/` is written by tests and **partly tracked in git** —
    `.gitignore` only excludes `results`, so `1s_video_then_stereo.mov`, `avci_prog.mov`,
    `hap_test_*.mp4` and `h264_weird_resolutions*` are committed build artefacts. Use
    `t.TempDir()`.
  - `utils/testutils/{video,audio}.go` shell out to real ffmpeg and `panic(err)` on failure
    (`video.go:35,60,103,147`; `audio.go:30,56,90`) instead of `t.Fatal`/`t.Skip`, and take no
    `*testing.T`. `hap_test.go:28-31` skips properly; most others don't.
  - `t.Parallel` appears in exactly one file (`services/transcode/preview_test.go`), and can't
    safely spread while tests share fixed paths under `testdata/generated/`.
  - `preview_test.go:22` embeds a ~20 KB ffprobe JSON blob as a const in the test file; it
    belongs in `testdata/`. And `preview_test.go` is in package `transcode` while
    `hap_test.go` is `transcode_test` — mixed convention within one package.

- **`go.mod`.** All 31 direct requires are imported somewhere — nothing is strictly unused — but:
  - **Two caching implementations:** the homegrown `cache/` and
    `github.com/Code-Hex/go-generics-cache`, the latter used once (`services/rclone/stats.go:5`).
  - **A set library used once:** `github.com/deckarep/golang-set/v2`
    (`services/vidispine/export.go:16`), alongside `samber/lo` in 32 files.
  - **Two mock frameworks:** `go.uber.org/mock` (direct, correct) and the deprecated/archived
    `github.com/golang/mock` (indirect) — check whether anything still pulls it.
  - **`teris-io/shortid`** (`cmd/trigger_ui/isilon_export.go:15`) overlaps `google/uuid`, used 6×.
  - **`davecgh/go-spew`** is pinned to a pre-release pseudo-version
    (`v1.1.2-0.20180830191138-…`) rather than the released `v1.1.1`, and is imported by live
    handler code.
  - **Five direct dependencies are stranded inside `// indirect` blocks**
    (`bcc-code/mediabank-bridge`, `aws-sdk-go-v2`, `invopop/jsonschema`, `rs/zerolog`,
    `rudderlabs/analytics-go/v4`), which means `go mod tidy` is not part of anyone's workflow.
  - **`aws-sdk-go-v2`** is used only by `workflows/export/vx_export_bmm_test.go:8` and is pinned
    at `v1.21.2` (Oct 2023).

- **Typos in exported names:** `SendTelegramErorr` (`utils/workflows/notify.go:13`),
  `EmtpySRTFile` (`services/vidispine/export.go:85`), `PlacholderTplData`
  (`vsapi/placeholder.go:23`), `SubSteams` (`services/ffmpeg/progress.go:51`),
  `sanitizeDuplicatdPath` (`workflows/ingest/asset_ingest.go:59`), `BmmTargetEnvionment`
  (`workflows/ingest/bmm_simple_upload.go:25`), `BMMLangauageCode` (`language_config.go:10`).
  Plus capitalised error strings (`ST1005`) at `services/vidispine/export.go:153,393`,
  `vsapi/metadata.go:181`, `vsapi/clips.go:135`, `vsapi/files_paths.go:29`,
  `workflows/ingest/multitrack.go:45`, `workflows/ingest/common.go:168`, and the ~10
  `"Directus API error"` sites.

- **Dockerfiles.** `worker.Dockerfile:8-15` runs as **root** (unlike `httpin.Dockerfile:12` and
  `trigger_ui.Dockerfile:12`, which both set `USER nonroot:nonroot`), uses unpinned
  `alpine:latest` with three separate `RUN apk` layers and no `--no-cache`, and installs no
  `ffmpeg`/`ffprobe` — so running that image with `QUEUE=audio` panics at
  `cmd/worker/main.go:138`. `deploy-images.yml` also has no build caching, so
  `transcode-worker.Dockerfile` recompiles ffmpeg from source on every master push, and it
  pushes a mutable `:latest` with no scanning, SBOM, or provenance. `cmd/bmm-trigger` and
  `cmd/fakerclone` are never built or verified anywhere.

- **Two stray SQLite databases** (`cmd/trigger_ui/trigger.db`, `ui.db`) sit untracked in the
  tree. Correctly `.gitignore`d and `git ls-files` confirms no secrets are committed — but they
  invite pointing `TRIGGER_DB` at a repo path during development.

- **TODO/FIXME/HACK inventory: 3 matches repo-wide**, none in `cmd/`, `environment/`, `utils/`,
  `analytics/`, `cache/` or the root files. Given the volume above, the debt in this codebase is
  *undocumented* rather than tracked — which is the main argument for keeping this file current.

## Found while fixing the workflowcheck sites (2026-08-12)

- **`analytics.GetService()` returns nil until `Init` runs, and every method dereferences the
  receiver.** `analytics/service.go:23` returns the package var `Instance`, which is only set by
  `Init`; each method then starts with `if s.rudderClient == nil`, which panics on a nil `s`
  rather than returning. Latent today — `cmd/worker/main.go:104` is the only `Init` caller and
  also the only place that registers `AnalyticsWorkerInterceptor` — but the interceptor wraps
  **every** workflow and activity, so a second entrypoint registering it without `Init`, or a
  test that does, panics on the first workflow task. A one-line `if s == nil { return }` at the
  top of each method removes the whole class.

- **`GetMapKeysSafely` is more expensive and less useful than sorting.**
  `utils/workflows/maps.go` records `lo.Keys` through a `workflow.SideEffect`: one history event
  per call, and the keys come back in whatever order the first run happened to produce. Its
  doc comment also overstates the guarantee — the recorded order is stable across *replays of
  one execution*, not "identical to other workflow executions". `wfutils.SortedKeys` gives a
  stronger guarantee for free wherever the key type is ordered. The ten existing callers were
  left alone deliberately: dropping a `SideEffect` changes the recorded history and breaks
  replay for in-flight executions, so any migration needs to be sequenced against a drain.

- **Dead `append` inside a `range` in `MergeAndImportSubtitlesFromCSV`.**
  `workflows/misc/merge_import_subs.go:146` does `langs = append(langs, lang)` while ranging
  over `langs`. `range` evaluates the slice header once, so the appended lowercased duplicates
  are never visited — the statement only grows a slice that is discarded. Either the loop was
  meant to re-process the lowercased language or the line is leftover; as written it reads like
  an intentional feed-forward and is not one.

- **`workflowcheck` is not pinned.** `make test` invokes whatever version is on the developer's
  `PATH` (v0.4.0 locally). Now that the check exits 0 and can gate merges, it should be a
  `tool` directive in `go.mod` and invoked as `go tool workflowcheck`, so everyone runs the same
  analyzer with the same built-in override list.

- **`SortFilesByImportedDate` carries a function-wide `//workflowcheck:ignore`.**
  `workflows/misc/cleanup_production.go:34` suppresses everything in a workflow that reads
  `time.Since`, iterates two maps and writes five `fmt.Printf` lines. That is fine while the
  comment above it holds — it is deliberately registered nowhere — but the suppression is
  unconditional, so registering it later silently ships all of that. Worth converting to the
  narrow line-level ignores the rest of the tree now uses.
