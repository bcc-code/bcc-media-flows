package environment

import (
	"os"
	"sync"
)

// Config is the environment as it was at boot. Load after bootstrap.LoadEnv.
type Config struct {
	Queue string

	IsilonPrefix            string
	TempMountPrefix         string
	FileCatalystMountPrefix string

	RcloneUsername string
	RclonePassword string

	PlayoutFTPAddress  string
	PlayoutFTPUsername string
	PlayoutFTPPassword string

	ShortsServiceURL  string
	TranscodeRootPath string
	SubtitleStylesDir string

	OverlaysDir          string
	MasterTriggerDir     string
	TriggeredByHeader    string
	MassiveWebhookAPIKey string
}

var (
	mu      sync.RWMutex
	current *Config
)

func Load() *Config {
	cfg := &Config{
		Queue: os.Getenv("QUEUE"),

		IsilonPrefix:            os.Getenv("ISILON_PREFIX"),
		TempMountPrefix:         os.Getenv("TEMP_MOUNT_PREFIX"),
		FileCatalystMountPrefix: os.Getenv("FILECATALYST_MOUNT_PREFIX"),

		RcloneUsername: os.Getenv("RCLONE_USERNAME"),
		RclonePassword: os.Getenv("RCLONE_PASSWORD"),

		PlayoutFTPAddress:  os.Getenv("PLAYOUT_FTP_ADDRESS"),
		PlayoutFTPUsername: os.Getenv("PLAYOUT_FTP_USERNAME"),
		PlayoutFTPPassword: os.Getenv("PLAYOUT_FTP_PASSWORD"),

		ShortsServiceURL:  os.Getenv("SHORTS_SERVICE_URL"),
		TranscodeRootPath: os.Getenv("TRANSCODE_ROOT_PATH"),
		SubtitleStylesDir: os.Getenv("SUBTITLE_STYLES_DIR"),

		OverlaysDir:          os.Getenv("OVERLAYS_DIR"),
		MasterTriggerDir:     os.Getenv("MASTER_TRIGGER_DIR"),
		TriggeredByHeader:    os.Getenv("TRIGGERED_BY_HEADER"),
		MassiveWebhookAPIKey: os.Getenv("MASSIVE_WEBHOOK_API_KEY"),
	}

	mu.Lock()
	current = cfg
	mu.Unlock()

	return cfg
}

// Get loads on first use if no main called Load.
func Get() *Config {
	mu.RLock()
	cfg := current
	mu.RUnlock()

	if cfg != nil {
		return cfg
	}

	return Load()
}
