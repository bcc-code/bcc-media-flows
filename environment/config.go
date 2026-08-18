package environment

import (
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
)

// Config is the environment as it was at boot. This file is the only place the
// process reads it; everything else takes what it needs from here.
type Config struct {
	Queue    string
	Identity string

	TemporalHostPort  string
	TemporalNamespace string
	Port              string

	IsilonPrefix            string
	TempMountPrefix         string
	FileCatalystMountPrefix string
	TranscodeRootPath       string
	SubtitleStylesDir       string
	OverlaysDir             string
	MasterTriggerDir        string

	VidispineBaseURL  string
	VidispineUsername string
	VidispinePassword string

	CantemoURL   string
	CantemoToken string

	SubtransBaseURL string
	SubtransAPIKey  string

	DirectusBaseURL      string
	DirectusAPIKey       string
	DirectusShortsFolder string

	ClickUpFrontdoorBaseURL string
	ClickUpWorkspaceID      string
	ClickUpShortsViewID     string
	ClickUpShortsViewToken  string

	RcloneUsername string
	RclonePassword string

	FileCatalystURL      string
	FileCatalystTaskID   string
	FileCatalystUsername string
	FileCatalystPassword string

	PlayoutFTPAddress  string
	PlayoutFTPUsername string
	PlayoutFTPPassword string

	RavenDBURL         string
	RavenDBDatabase    string
	RavenDBCertPath    string
	RavenDBCertKeyPath string

	TelegramBotToken      string
	TelegramChatVOD       int64
	TelegramChatOslofjord int64
	TelegramChatOther     int64
	TelegramChatBMM       int64

	SendgridAPIKey string

	ShortsServiceURL     string
	SyncServiceURL       string
	VizualizerBaseURL    string
	BMMFileBaseURL       string
	MassiveWebhookAPIKey string
	TriggeredByHeader    string

	RudderstackWriteKey     string
	RudderstackDataPlaneURL string

	ActivityCount int
	H264Encoder   string
}

var (
	mu      sync.RWMutex
	current *Config
)

func Load() *Config {
	cfg := &Config{
		Queue:    os.Getenv("QUEUE"),
		Identity: os.Getenv("IDENTITY"),

		TemporalHostPort:  os.Getenv("TEMPORAL_HOST_PORT"),
		TemporalNamespace: os.Getenv("TEMPORAL_NAMESPACE"),
		Port:              os.Getenv("PORT"),

		IsilonPrefix:            os.Getenv("ISILON_PREFIX"),
		TempMountPrefix:         os.Getenv("TEMP_MOUNT_PREFIX"),
		FileCatalystMountPrefix: os.Getenv("FILECATALYST_MOUNT_PREFIX"),
		TranscodeRootPath:       os.Getenv("TRANSCODE_ROOT_PATH"),
		SubtitleStylesDir:       os.Getenv("SUBTITLE_STYLES_DIR"),
		OverlaysDir:             os.Getenv("OVERLAYS_DIR"),
		MasterTriggerDir:        os.Getenv("MASTER_TRIGGER_DIR"),

		VidispineBaseURL:  os.Getenv("VIDISPINE_BASE_URL"),
		VidispineUsername: os.Getenv("VIDISPINE_USERNAME"),
		VidispinePassword: os.Getenv("VIDISPINE_PASSWORD"),

		CantemoURL:   os.Getenv("CANTEMO_URL"),
		CantemoToken: os.Getenv("CANTEMO_TOKEN"),

		SubtransBaseURL: os.Getenv("SUBTRANS_BASE_URL"),
		SubtransAPIKey:  os.Getenv("SUBTRANS_API_KEY"),

		DirectusBaseURL:      os.Getenv("DIRECTUS_BASE_URL"),
		DirectusAPIKey:       os.Getenv("DIRECTUS_API_KEY"),
		DirectusShortsFolder: os.Getenv("DIRECTUS_SHORTS_FOLDER_ID"),

		ClickUpFrontdoorBaseURL: os.Getenv("CLICKUP_FRONTDOOR_BASE_URL"),
		ClickUpWorkspaceID:      os.Getenv("CLICKUP_WORKSPACE_ID"),
		ClickUpShortsViewID:     os.Getenv("CLICKUP_SHORTS_VIEW_ID"),
		ClickUpShortsViewToken:  os.Getenv("CLICKUP_SHORTS_VIEW_TOKEN"),

		RcloneUsername: os.Getenv("RCLONE_USERNAME"),
		RclonePassword: os.Getenv("RCLONE_PASSWORD"),

		FileCatalystURL:      os.Getenv("FILECATALYST_URL"),
		FileCatalystTaskID:   os.Getenv("FILECATALYST_TASK_ID"),
		FileCatalystUsername: os.Getenv("FILECATALYST_USERNAME"),
		FileCatalystPassword: os.Getenv("FILECATALYST_PASSWORD"),

		PlayoutFTPAddress:  os.Getenv("PLAYOUT_FTP_ADDRESS"),
		PlayoutFTPUsername: os.Getenv("PLAYOUT_FTP_USERNAME"),
		PlayoutFTPPassword: os.Getenv("PLAYOUT_FTP_PASSWORD"),

		RavenDBURL:         strings.TrimRight(os.Getenv("RAVENDB_URL"), "/"),
		RavenDBDatabase:    os.Getenv("RAVENDB_DATABASE"),
		RavenDBCertPath:    os.Getenv("RAVENDB_CERT_PATH"),
		RavenDBCertKeyPath: os.Getenv("RAVENDB_CERT_KEY_PATH"),

		TelegramBotToken:      os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramChatVOD:       chatID("TELEGRAM_CHAT_ID_VOD"),
		TelegramChatOslofjord: chatID("TELEGRAM_CHAT_ID_OSLOFJORD"),
		TelegramChatOther:     chatID("TELEGRAM_CHAT_ID_OTHER"),
		TelegramChatBMM:       chatID("TELEGRAM_CHAT_ID_BMM"),

		SendgridAPIKey: os.Getenv("SENDGRID_API_KEY"),

		ShortsServiceURL:     os.Getenv("SHORTS_SERVICE_URL"),
		SyncServiceURL:       os.Getenv("SYNC_SERVICE_URL"),
		VizualizerBaseURL:    os.Getenv("VIZUALIZER_BASE_URL"),
		BMMFileBaseURL:       os.Getenv("BMM_FILE_BASE_URL"),
		MassiveWebhookAPIKey: os.Getenv("MASSIVE_WEBHOOK_API_KEY"),
		TriggeredByHeader:    os.Getenv("TRIGGERED_BY_HEADER"),

		RudderstackWriteKey:     os.Getenv("RUDDERSTACK_WRITE_KEY"),
		RudderstackDataPlaneURL: os.Getenv("RUDDERSTACK_DATA_PLANE_URL"),

		ActivityCount: intOr("ACTIVITY_COUNT", 5),
		H264Encoder:   os.Getenv("H264_ENCODER"),
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

// chatID reports a malformed value once, at boot, rather than on every send.
func chatID(name string) int64 {
	raw := os.Getenv(name)
	if raw == "" {
		return 0
	}

	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		log.Printf("WARNING: %s is not a number: %v", name, err)
		return 0
	}

	return id
}

func intOr(name string, fallback int) int {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		log.Printf("WARNING: %s is not a number, using %d: %v", name, fallback, err)
		return fallback
	}

	return value
}

// Getenv is for the handful of places that look up a name chosen at runtime.
func Getenv(name string) string {
	return os.Getenv(name)
}

// The service packages declare what they need; these hand it over.

func (c *Config) Vidispine() (string, string, string) {
	return c.VidispineBaseURL, c.VidispineUsername, c.VidispinePassword
}

func (c *Config) Cantemo() (string, string) {
	return c.CantemoURL, c.CantemoToken
}

func (c *Config) Subtrans() (string, string) {
	return c.SubtransBaseURL, c.SubtransAPIKey
}

func (c *Config) Directus() (string, string) {
	return c.DirectusBaseURL, c.DirectusAPIKey
}

func (c *Config) ClickUp() (string, string, string, string) {
	return c.ClickUpFrontdoorBaseURL, c.ClickUpWorkspaceID, c.ClickUpShortsViewID, c.ClickUpShortsViewToken
}

func (c *Config) Vizualizer() string {
	if c.VizualizerBaseURL != "" {
		return c.VizualizerBaseURL
	}
	return "http://vizualizer.lan.bcc.media"
}
