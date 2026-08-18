package environment

import (
	"log"
	"os"
	"strings"
)

// Required lists the variables each entrypoint needs to do its job. Unset ones are
// reported at boot rather than surfacing later as a 401 or an empty path, often deep
// inside an activity. Missing values do not stop the process: a worker that can do
// most of its work is worth more than one that refuses to start.
var (
	RequiredByWorker = []string{
		"TEMPORAL_HOST_PORT",
		"VIDISPINE_BASE_URL", "VIDISPINE_USERNAME", "VIDISPINE_PASSWORD",
		"CANTEMO_URL", "CANTEMO_TOKEN",
		"SUBTRANS_BASE_URL", "SUBTRANS_API_KEY",
		"DIRECTUS_BASE_URL", "DIRECTUS_API_KEY",
		"RCLONE_USERNAME", "RCLONE_PASSWORD",
		"FILECATALYST_URL", "FILECATALYST_USERNAME", "FILECATALYST_PASSWORD",
		"PLAYOUT_FTP_ADDRESS", "PLAYOUT_FTP_USERNAME", "PLAYOUT_FTP_PASSWORD",
		"TELEGRAM_BOT_TOKEN",
		"SENDGRID_API_KEY",
	}

	RequiredByTriggerUI = []string{
		"TEMPORAL_HOST_PORT",
		"VIDISPINE_BASE_URL", "VIDISPINE_USERNAME", "VIDISPINE_PASSWORD",
		"MASSIVE_WEBHOOK_API_KEY",
		"SUBTITLE_STYLES_DIR", "OVERLAYS_DIR", "MASTER_TRIGGER_DIR",
	}

	RequiredByHTTPIn = []string{
		"TEMPORAL_HOST_PORT",
		"TRANSCODE_ROOT_PATH",
	}

	RequiredByBMMTrigger = []string{
		"TEMPORAL_HOST_PORT",
		"RAVENDB_URL", "RAVENDB_DATABASE",
	}
)

// WarnMissing reports every unset variable at once, so a misconfigured deployment is
// one log line rather than a series of discoveries.
func WarnMissing(required []string) []string {
	var missing []string
	for _, name := range required {
		if os.Getenv(name) == "" {
			missing = append(missing, name)
		}
	}

	if len(missing) > 0 {
		log.Printf("WARNING: %d unset environment variables: %s", len(missing), strings.Join(missing, ", "))
	}

	return missing
}
