package environment

import (
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
)

type Temporal struct {
	hostPort  string
	namespace string
}

func (t Temporal) HostPort() string  { return t.hostPort }
func (t Temporal) Namespace() string { return t.namespace }

type Paths struct {
	isilonPrefix      string
	tempMount         string
	fileCatalystMount string
	transcodeRoot     string
	subtitleStyles    string
	overlays          string
	masterTrigger     string
}

func (p Paths) IsilonPrefix() string      { return p.isilonPrefix }
func (p Paths) TempMount() string         { return p.tempMount }
func (p Paths) FileCatalystMount() string { return p.fileCatalystMount }
func (p Paths) TranscodeRoot() string     { return p.transcodeRoot }
func (p Paths) SubtitleStyles() string    { return p.subtitleStyles }
func (p Paths) Overlays() string          { return p.overlays }
func (p Paths) MasterTrigger() string     { return p.masterTrigger }

type Vidispine struct {
	baseURL  string
	username string
	password string
}

func (v Vidispine) BaseURL() string  { return v.baseURL }
func (v Vidispine) Username() string { return v.username }
func (v Vidispine) Password() string { return v.password }

type Cantemo struct {
	url   string
	token string
}

func (c Cantemo) URL() string   { return c.url }
func (c Cantemo) Token() string { return c.token }

type Subtrans struct {
	baseURL string
	apiKey  string
}

func (s Subtrans) BaseURL() string { return s.baseURL }
func (s Subtrans) APIKey() string  { return s.apiKey }

type Directus struct {
	baseURL        string
	apiKey         string
	shortsFolderID string
}

func (d Directus) BaseURL() string        { return d.baseURL }
func (d Directus) APIKey() string         { return d.apiKey }
func (d Directus) ShortsFolderID() string { return d.shortsFolderID }

type ClickUp struct {
	frontdoorBaseURL string
	workspaceID      string
	shortsViewID     string
	shortsViewToken  string
}

func (c ClickUp) FrontdoorBaseURL() string { return c.frontdoorBaseURL }
func (c ClickUp) WorkspaceID() string      { return c.workspaceID }
func (c ClickUp) ShortsViewID() string     { return c.shortsViewID }
func (c ClickUp) ShortsViewToken() string  { return c.shortsViewToken }

type Rclone struct {
	username string
	password string
}

func (r Rclone) Username() string { return r.username }
func (r Rclone) Password() string { return r.password }

type FileCatalyst struct {
	url      string
	taskID   string
	username string
	password string
}

func (f FileCatalyst) URL() string      { return f.url }
func (f FileCatalyst) TaskID() string   { return f.taskID }
func (f FileCatalyst) Username() string { return f.username }
func (f FileCatalyst) Password() string { return f.password }

type PlayoutFTP struct {
	address  string
	username string
	password string
}

func (p PlayoutFTP) Address() string  { return p.address }
func (p PlayoutFTP) Username() string { return p.username }
func (p PlayoutFTP) Password() string { return p.password }

type RavenDB struct {
	url         string
	database    string
	certPath    string
	certKeyPath string
}

func (r RavenDB) URL() string         { return r.url }
func (r RavenDB) Database() string    { return r.database }
func (r RavenDB) CertPath() string    { return r.certPath }
func (r RavenDB) CertKeyPath() string { return r.certKeyPath }

type Telegram struct {
	botToken      string
	chatVOD       int64
	chatOslofjord int64
	chatOther     int64
	chatBMM       int64
}

func (t Telegram) BotToken() string     { return t.botToken }
func (t Telegram) ChatVOD() int64       { return t.chatVOD }
func (t Telegram) ChatOslofjord() int64 { return t.chatOslofjord }
func (t Telegram) ChatOther() int64     { return t.chatOther }
func (t Telegram) ChatBMM() int64       { return t.chatBMM }

type Services struct {
	shorts     string
	sync       string
	vizualizer string
}

func (s Services) Shorts() string { return s.shorts }
func (s Services) Sync() string   { return s.sync }
func (s Services) Vizualizer() string {
	if s.vizualizer != "" {
		return s.vizualizer
	}
	return "http://vizualizer.lan.bcc.media"
}

type TriggerUI struct {
	massiveWebhookAPIKey string
	triggeredByHeader    string
}

func (t TriggerUI) MassiveWebhookAPIKey() string { return t.massiveWebhookAPIKey }
func (t TriggerUI) TriggeredByHeader() string    { return t.triggeredByHeader }

type Rudderstack struct {
	writeKey     string
	dataPlaneURL string
}

func (r Rudderstack) WriteKey() string     { return r.writeKey }
func (r Rudderstack) DataPlaneURL() string { return r.dataPlaneURL }

// Config is the environment as it was at boot. This file is the only place the
// process reads it; everything else takes the group it needs.
type Config struct {
	Queue    string
	Identity string
	Port     string

	ActivityCount  int
	H264Encoder    string
	SendgridAPIKey string
	BMMFileBaseURL string

	Temporal     Temporal
	Paths        Paths
	Vidispine    Vidispine
	Cantemo      Cantemo
	Subtrans     Subtrans
	Directus     Directus
	ClickUp      ClickUp
	Rclone       Rclone
	FileCatalyst FileCatalyst
	PlayoutFTP   PlayoutFTP
	RavenDB      RavenDB
	Telegram     Telegram
	Services     Services
	TriggerUI    TriggerUI
	Rudderstack  Rudderstack
}

var (
	mu      sync.RWMutex
	current *Config
)

func Load() *Config {
	cfg := &Config{
		Queue:    os.Getenv("QUEUE"),
		Identity: os.Getenv("IDENTITY"),
		Port:     os.Getenv("PORT"),

		ActivityCount:  intOr("ACTIVITY_COUNT", 5),
		H264Encoder:    os.Getenv("H264_ENCODER"),
		SendgridAPIKey: os.Getenv("SENDGRID_API_KEY"),
		BMMFileBaseURL: os.Getenv("BMM_FILE_BASE_URL"),

		Temporal: Temporal{
			hostPort:  os.Getenv("TEMPORAL_HOST_PORT"),
			namespace: os.Getenv("TEMPORAL_NAMESPACE"),
		},

		Paths: Paths{
			isilonPrefix:      os.Getenv("ISILON_PREFIX"),
			tempMount:         os.Getenv("TEMP_MOUNT_PREFIX"),
			fileCatalystMount: os.Getenv("FILECATALYST_MOUNT_PREFIX"),
			transcodeRoot:     os.Getenv("TRANSCODE_ROOT_PATH"),
			subtitleStyles:    os.Getenv("SUBTITLE_STYLES_DIR"),
			overlays:          os.Getenv("OVERLAYS_DIR"),
			masterTrigger:     os.Getenv("MASTER_TRIGGER_DIR"),
		},

		Vidispine: Vidispine{
			baseURL:  os.Getenv("VIDISPINE_BASE_URL"),
			username: os.Getenv("VIDISPINE_USERNAME"),
			password: os.Getenv("VIDISPINE_PASSWORD"),
		},

		Cantemo: Cantemo{
			url:   os.Getenv("CANTEMO_URL"),
			token: os.Getenv("CANTEMO_TOKEN"),
		},

		Subtrans: Subtrans{
			baseURL: os.Getenv("SUBTRANS_BASE_URL"),
			apiKey:  os.Getenv("SUBTRANS_API_KEY"),
		},

		Directus: Directus{
			baseURL:        os.Getenv("DIRECTUS_BASE_URL"),
			apiKey:         os.Getenv("DIRECTUS_API_KEY"),
			shortsFolderID: os.Getenv("DIRECTUS_SHORTS_FOLDER_ID"),
		},

		ClickUp: ClickUp{
			frontdoorBaseURL: os.Getenv("CLICKUP_FRONTDOOR_BASE_URL"),
			workspaceID:      os.Getenv("CLICKUP_WORKSPACE_ID"),
			shortsViewID:     os.Getenv("CLICKUP_SHORTS_VIEW_ID"),
			shortsViewToken:  os.Getenv("CLICKUP_SHORTS_VIEW_TOKEN"),
		},

		Rclone: Rclone{
			username: os.Getenv("RCLONE_USERNAME"),
			password: os.Getenv("RCLONE_PASSWORD"),
		},

		FileCatalyst: FileCatalyst{
			url:      os.Getenv("FILECATALYST_URL"),
			taskID:   os.Getenv("FILECATALYST_TASK_ID"),
			username: os.Getenv("FILECATALYST_USERNAME"),
			password: os.Getenv("FILECATALYST_PASSWORD"),
		},

		PlayoutFTP: PlayoutFTP{
			address:  os.Getenv("PLAYOUT_FTP_ADDRESS"),
			username: os.Getenv("PLAYOUT_FTP_USERNAME"),
			password: os.Getenv("PLAYOUT_FTP_PASSWORD"),
		},

		RavenDB: RavenDB{
			url:         strings.TrimRight(os.Getenv("RAVENDB_URL"), "/"),
			database:    os.Getenv("RAVENDB_DATABASE"),
			certPath:    os.Getenv("RAVENDB_CERT_PATH"),
			certKeyPath: os.Getenv("RAVENDB_CERT_KEY_PATH"),
		},

		Telegram: Telegram{
			botToken:      os.Getenv("TELEGRAM_BOT_TOKEN"),
			chatVOD:       chatID("TELEGRAM_CHAT_ID_VOD"),
			chatOslofjord: chatID("TELEGRAM_CHAT_ID_OSLOFJORD"),
			chatOther:     chatID("TELEGRAM_CHAT_ID_OTHER"),
			chatBMM:       chatID("TELEGRAM_CHAT_ID_BMM"),
		},

		Services: Services{
			shorts:     os.Getenv("SHORTS_SERVICE_URL"),
			sync:       os.Getenv("SYNC_SERVICE_URL"),
			vizualizer: os.Getenv("VIZUALIZER_BASE_URL"),
		},

		TriggerUI: TriggerUI{
			massiveWebhookAPIKey: os.Getenv("MASSIVE_WEBHOOK_API_KEY"),
			triggeredByHeader:    os.Getenv("TRIGGERED_BY_HEADER"),
		},

		Rudderstack: Rudderstack{
			writeKey:     os.Getenv("RUDDERSTACK_WRITE_KEY"),
			dataPlaneURL: os.Getenv("RUDDERSTACK_DATA_PLANE_URL"),
		},
	}

	mu.Lock()
	current = cfg
	mu.Unlock()

	return cfg
}

// init loads once so Get never has to. A main calls Load again after reading .env,
// which is what makes the file take effect.
func init() {
	Load()
}

// Get is a plain read: no main, no .env, no environment lookup on this path.
func Get() *Config {
	mu.RLock()
	defer mu.RUnlock()

	return current
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
