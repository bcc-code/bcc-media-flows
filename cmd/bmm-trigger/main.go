package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/bcc-code/bcc-media-flows/services/bmm"
	ingestworkflows "github.com/bcc-code/bcc-media-flows/workflows/ingest"
	"github.com/joho/godotenv"
	"go.temporal.io/sdk/client"
)

func main() {
	_ = godotenv.Load(".env")

	trackID := flag.Int("track-id", 0, "BMM track ID to load from RavenDB (mutually exclusive with --album-id and --playlist-id)")
	albumID := flag.Int("album-id", 0, "BMM album (ParentId) to load tracks for (mutually exclusive with --track-id and --playlist-id)")
	playlistID := flag.Int("playlist-id", 0, "BMM curated playlist ID to load tracks for (mutually exclusive with --track-id and --album-id)")
	doProcess := flag.Bool("process", false, "With --album-id or --playlist-id: run the full per-track pipeline (map → print JSON → optional trigger). Default lists track IDs only.")
	language := flag.String("language", "", "Translation language to use; defaults to the track's OriginalLanguage")
	doTrigger := flag.Bool("trigger", false, "Fire the BmmTrackMetadata workflow (default: dry-run, only prints JSON)")
	doWait := flag.Bool("wait", false, "When --trigger is set, block until the workflow completes")
	vxSource := flag.String("vx-source", "", "Override vxSource (skip ingest, update metadata only)")
	fileURL := flag.String("file-url", "", "Override fileUrl (skip file URL construction from RavenDB)")
	fileBaseURL := flag.String("file-base-url", os.Getenv("BMM_FILE_BASE_URL"), "Base URL prefixed to the audio file Path (defaults to $BMM_FILE_BASE_URL)")
	noResolve := flag.Bool("no-resolve", false, "Skip following redirects on fileUrl; keep the constructed URL verbatim (with userinfo if present)")
	flag.Parse()

	setCount := boolToInt(*trackID != 0) + boolToInt(*albumID != 0) + boolToInt(*playlistID != 0)
	if setCount == 0 {
		fmt.Fprintln(os.Stderr, "error: exactly one of --track-id, --album-id, or --playlist-id is required")
		flag.Usage()
		os.Exit(2)
	}
	if setCount > 1 {
		fmt.Fprintln(os.Stderr, "error: --track-id, --album-id, and --playlist-id are mutually exclusive")
		os.Exit(2)
	}
	multiTrack := *albumID != 0 || *playlistID != 0
	if *doProcess && !multiTrack {
		fmt.Fprintln(os.Stderr, "error: --process requires --album-id or --playlist-id")
		os.Exit(2)
	}
	if *vxSource != "" && multiTrack {
		fmt.Fprintln(os.Stderr, "error: --vx-source cannot be used with --album-id or --playlist-id (it would apply to every track)")
		os.Exit(2)
	}
	if *fileURL != "" && multiTrack {
		fmt.Fprintln(os.Stderr, "error: --file-url cannot be used with --album-id or --playlist-id (it would apply to every track)")
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

	mapOpts := bmm.MappingOptions{
		Language:         *language,
		FileBaseURL:      *fileBaseURL,
		VXSourceOverride: *vxSource,
		FileURLOverride:  *fileURL,
	}

	switch {
	case *trackID != 0:
		track, err := raven.LoadTrackByBmmID(ctx, *trackID)
		if err != nil {
			fatal("load track: %v", err)
		}

		tc, err := dialTemporalIfTriggering(*doTrigger)
		if err != nil {
			fatal("%v", err)
		}
		if tc != nil {
			defer tc.Close()
		}

		if err := processTrack(ctx, tc, track, mapOpts, !*noResolve, *doTrigger, *doWait); err != nil {
			fatal("%v", err)
		}

	case *albumID != 0:
		tracks, err := raven.LoadTracksByAlbumID(ctx, *albumID)
		if err != nil {
			fatal("load tracks: %v", err)
		}
		fmt.Fprintf(os.Stderr, "found %d tracks for album %d\n", len(tracks), *albumID)
		runMultiTrack(ctx, tracks, mapOpts, *doProcess, *doTrigger, *doWait, !*noResolve)

	case *playlistID != 0:
		pl, ids, err := raven.LoadTrackIDsForPlaylist(ctx, *playlistID)
		if err != nil {
			fatal("load playlist: %v", err)
		}

		mapOpts.CuratedPlaylist = &ingestworkflows.BmmCuratedPlaylist{
			ID:   strconv.Itoa(pl.ID),
			Name: pl.PlaylistName(*language),
		}
		fmt.Fprintf(os.Stderr, "found %d tracks for playlist %d (%q)\n", len(ids), *playlistID, mapOpts.CuratedPlaylist.Name)

		if !*doProcess {
			for _, id := range ids {
				fmt.Println(id)
			}
			return
		}

		tracks, err := raven.LoadTracksByIDs(ctx, ids)
		if err != nil {
			fatal("load tracks: %v", err)
		}
		if len(tracks) < len(ids) {
			fmt.Fprintf(os.Stderr, "warning: playlist references %d tracks but only %d were resolved from Tracks\n", len(ids), len(tracks))
		}
		runMultiTrack(ctx, tracks, mapOpts, true, *doTrigger, *doWait, !*noResolve)
	}
}

// runMultiTrack handles --album-id and --playlist-id --process flows: either lists IDs
// (when doProcess=false) or runs processTrack per track. Exits non-zero on per-track failures.
func runMultiTrack(ctx context.Context, tracks []bmm.RavenTrack, mapOpts bmm.MappingOptions, doProcess, doTrigger, doWait, doResolve bool) {
	if !doProcess {
		for _, t := range tracks {
			fmt.Println(t.ID)
		}
		return
	}

	tc, err := dialTemporalIfTriggering(doTrigger)
	if err != nil {
		fatal("%v", err)
	}
	if tc != nil {
		defer tc.Close()
	}

	failed := 0
	for i := range tracks {
		t := &tracks[i]
		fmt.Fprintf(os.Stderr, "--- track %d (%d/%d) ---\n", t.ID, i+1, len(tracks))
		if err := processTrack(ctx, tc, t, mapOpts, doResolve, doTrigger, doWait); err != nil {
			fmt.Fprintf(os.Stderr, "track %d: %v\n", t.ID, err)
			failed++
		}
	}

	if failed > 0 {
		fmt.Fprintf(os.Stderr, "completed with %d/%d tracks failed\n", failed, len(tracks))
		os.Exit(1)
	}
}

func processTrack(ctx context.Context, tc client.Client, track *bmm.RavenTrack, opts bmm.MappingOptions, doResolve, doTrigger, doWait bool) error {
	params, err := bmm.ToWorkflowParams(track, opts)
	if err != nil {
		return fmt.Errorf("map params: %w", err)
	}

	if params.FileURL != "" && doResolve {
		fmt.Fprintf(os.Stderr, "resolving fileUrl via HEAD (follow redirects)\n")
		resolved, err := bmm.ResolveFileURL(ctx, params.FileURL)
		if err != nil {
			return fmt.Errorf("resolve fileUrl: %w", err)
		}
		if resolved != params.FileURL {
			fmt.Fprintf(os.Stderr, "fileUrl resolved -> %s\n", resolved)
		}
		params.FileURL = resolved
	}

	pretty, err := json.MarshalIndent(params, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal params: %w", err)
	}
	fmt.Println(string(pretty))

	if !doTrigger {
		return nil
	}

	res, err := bmm.TriggerBmmTrackMetadata(ctx, tc, params, doWait)
	if res != nil {
		fmt.Printf("workflow_id=%s run_id=%s\n", res.WorkflowID, res.RunID)
	}
	if err != nil {
		return fmt.Errorf("trigger: %w", err)
	}
	if res.Result != nil {
		fmt.Printf("asset_id=%s\n", res.Result.AssetID)
	}
	return nil
}

func dialTemporalIfTriggering(doTrigger bool) (client.Client, error) {
	if !doTrigger {
		return nil, nil
	}
	fmt.Fprintf(os.Stderr, "connecting to Temporal host=%s namespace=%s queue=%s\n",
		envOrDash("TEMPORAL_HOST_PORT"), envOrDash("TEMPORAL_NAMESPACE"), envOrDash("QUEUE"))
	tc, err := bmm.NewTemporalClient()
	if err != nil {
		return nil, fmt.Errorf("temporal client: %w", err)
	}
	return tc, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
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
