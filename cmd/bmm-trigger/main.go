package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/bcc-code/bcc-media-flows/services/bmm"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load(".env")

	trackID := flag.Int("track-id", 0, "BMM track ID to load from RavenDB (required)")
	language := flag.String("language", "", "Translation language to use; defaults to the track's OriginalLanguage")
	doTrigger := flag.Bool("trigger", false, "Fire the BmmTrackMetadata workflow (default: dry-run, only prints JSON)")
	doWait := flag.Bool("wait", false, "When --trigger is set, block until the workflow completes")
	vxSource := flag.String("vx-source", "", "Override vxSource (skip ingest, update metadata only)")
	fileURL := flag.String("file-url", "", "Override fileUrl (skip file URL construction from RavenDB)")
	fileBaseURL := flag.String("file-base-url", os.Getenv("BMM_FILE_BASE_URL"), "Base URL prefixed to the audio file Path (defaults to $BMM_FILE_BASE_URL)")
	noResolve := flag.Bool("no-resolve", false, "Skip following redirects on fileUrl; keep the constructed URL verbatim (with userinfo if present)")
	flag.Parse()

	if *trackID == 0 {
		fmt.Fprintln(os.Stderr, "error: --track-id is required")
		flag.Usage()
		os.Exit(2)
	}

	ctx := context.Background()

	cfg, err := bmm.LoadRavenConfigFromEnv()
	if err != nil {
		fatal("ravendb config: %v", err)
	}

	fmt.Fprintf(os.Stderr, "connecting to RavenDB url=%s database=%s cert=%s\n", cfg.URL, cfg.Database, certLabel(cfg))

	raven, err := bmm.NewRavenClient(cfg)
	if err != nil {
		fatal("ravendb client: %v", err)
	}

	track, err := raven.LoadTrackByBmmID(ctx, *trackID)
	if err != nil {
		fatal("load track: %v", err)
	}

	params, err := bmm.ToWorkflowParams(track, bmm.MappingOptions{
		Language:         *language,
		FileBaseURL:      *fileBaseURL,
		VXSourceOverride: *vxSource,
		FileURLOverride:  *fileURL,
	})
	if err != nil {
		fatal("map params: %v", err)
	}

	if params.FileURL != "" && !*noResolve {
		fmt.Fprintf(os.Stderr, "resolving fileUrl via HEAD (follow redirects)\n")
		resolved, err := bmm.ResolveFileURL(ctx, params.FileURL)
		if err != nil {
			fatal("resolve fileUrl: %v", err)
		}
		if resolved != params.FileURL {
			fmt.Fprintf(os.Stderr, "fileUrl resolved -> %s\n", resolved)
		}
		params.FileURL = resolved
	}

	pretty, err := json.MarshalIndent(params, "", "  ")
	if err != nil {
		fatal("marshal params: %v", err)
	}
	fmt.Println(string(pretty))

	if !*doTrigger {
		return
	}

	fmt.Fprintf(os.Stderr, "connecting to Temporal host=%s namespace=%s queue=%s\n",
		envOrDash("TEMPORAL_HOST_PORT"), envOrDash("TEMPORAL_NAMESPACE"), envOrDash("QUEUE"))

	tc, err := bmm.NewTemporalClient()
	if err != nil {
		fatal("temporal client: %v", err)
	}
	defer tc.Close()

	res, err := bmm.TriggerBmmTrackMetadata(ctx, tc, params, *doWait)
	if res != nil {
		fmt.Printf("workflow_id=%s run_id=%s\n", res.WorkflowID, res.RunID)
	}
	if err != nil {
		fatal("trigger: %v", err)
	}
	if res.Result != nil {
		fmt.Printf("asset_id=%s\n", res.Result.AssetID)
	}
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}

func certLabel(cfg bmm.RavenConfig) string {
	if cfg.CertPath == "" {
		return "(none)"
	}
	return cfg.CertPath
}

func envOrDash(key string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return "-"
}
