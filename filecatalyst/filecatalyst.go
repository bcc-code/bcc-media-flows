package filecatalyst

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/bcc-code/bcc-media-flows/environment"
	"math/rand"
	"time"

	"github.com/go-resty/resty/v2"

	"github.com/bcc-code/bcc-media-flows/internal/httpx"
)

// FileCatalystTaskConfig represents the configuration for a FileCatalyst task
type FileCatalystTaskConfig struct {
	Href                        string `json:"href"`
	AdvancedProgressivesOptions struct {
		PostHeaderFooterOptions struct {
			Filter      string `json:"filter"`
			Timeout     int    `json:"timeout"`
			HeaderBytes int    `json:"headerBytes"`
			FooterBytes int    `json:"footerBytes"`
		} `json:"postHeaderFooterOptions"`
	} `json:"advancedProgressivesOptions"`
	AllowTransferRefresh        bool   `json:"allowTransferRefresh"`
	AlwaysOn                    bool   `json:"alwaysOn"`
	ApplyFilterToDirectories    bool   `json:"applyFilterToDirectories"`
	AutoShowActivity            bool   `json:"autoShowActivity"`
	CompressSingleArchive       bool   `json:"compressSingleArchive"`
	CompressionFileFilter       string `json:"compressionFileFilter"`
	CompressionLevel            int    `json:"compressionLevel"`
	CompressionMode             string `json:"compressionMode"`
	CongestionControlAggression int    `json:"congestionControlAggression"`
	CongestionControlStrategy   string `json:"congestionControlStrategy"`
	CurrentRate                 int    `json:"currentRate"`
	DayFilterSwitch             bool   `json:"dayFilterSwitch"`
	DeleteAfterTransfer         bool   `json:"deleteAfterTransfer"`
	DirSynchListing             bool   `json:"dirSynchListing"`
	DynamicFolder               bool   `json:"dynamicFolder"`
	EnableAutoResume            bool   `json:"enableAutoResume"`
	EnableCache                 bool   `json:"enableCache"`
	EnableCompression           bool   `json:"enableCompression"`
	EnableEmailAlerts           bool   `json:"enableEmailAlerts"`
	EnableFilePriority          bool   `json:"enableFilePriority"`
	EnableProgressive           bool   `json:"enableProgressive"`
	Enabled                     bool   `json:"enabled"`
	ErrorEmailText              string `json:"errorEmailText"`
	FileFilterMode              string `json:"fileFilterMode"`
	FileFilterTarget            string `json:"fileFilterTarget"`
	FilePriority                string `json:"filePriority"`
	ForceFileOwnershipEnabled   bool   `json:"forceFileOwnershipEnabled"`
	ForceFileOwnershipGroup     string `json:"forceFileOwnershipGroup"`
	ForceFileOwnershipUser      string `json:"forceFileOwnershipUser"`
	GenerateCache               bool   `json:"generateCache"`
	HotFolder                   struct {
		Href          string `json:"href"`
		ID            string `json:"id"`
		Location      string `json:"location"`
		Status        string `json:"status"`
		StatusMessage string `json:"statusMessage"`
	} `json:"hotFolder"`
	IgnoreFileFilter             string `json:"ignoreFileFilter"`
	IncomingDataPort             int    `json:"incomingDataPort"`
	IncrementalOption            string `json:"incrementalOption"`
	IncrementalSizeCheckOnly     bool   `json:"incrementalSizeCheckOnly"`
	IsTempExtensionPrefix        bool   `json:"isTempExtensionPrefix"`
	IsUpload                     bool   `json:"isUpload"`
	KeepFileModificationDateTime bool   `json:"keepFileModificationDateTime"`
	KeepFilePermissions          bool   `json:"keepFilePermissions"`
	LowerByteLimit               int    `json:"lowerByteLimit"`
	MaxSentItemsAgeDays          int    `json:"maxSentItemsAgeDays"`
	NewerThanDays                int    `json:"newerThanDays"`
	NextExecutionTime            string `json:"nextExecutionTime"`
	NumberOfClients              int    `json:"numberOfClients"`
	PostURL                      string `json:"postURL"`
	RealtimeMonitoring           bool   `json:"realtimeMonitoring"`
	RemoteFolder                 string `json:"remoteFolder"`
	Schedule                     struct {
		EnableFriday       bool   `json:"enableFriday"`
		EnableMonday       bool   `json:"enableMonday"`
		EnableSaturday     bool   `json:"enableSaturday"`
		EnableSunday       bool   `json:"enableSunday"`
		EnableThursday     bool   `json:"enableThursday"`
		EnableTuesday      bool   `json:"enableTuesday"`
		EnableWednesday    bool   `json:"enableWednesday"`
		RecurrenceInterval string `json:"recurrenceInterval"`
		ReferenceDate      int64  `json:"referenceDate"`
		ReferenceHour      int    `json:"referenceHour"`
		ReferenceMinute    int    `json:"referenceMinute"`
	} `json:"schedule"`
	SendEmailFilenameList bool   `json:"sendEmailFilenameList"`
	SendEmailOnError      bool   `json:"sendEmailOnError"`
	SendEmailOnSuccess    bool   `json:"sendEmailOnSuccess"`
	SentFolderLocation    string `json:"sentFolderLocation"`
	Site                  struct {
		Href  string `json:"href"`
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"site"`
	SiteAgentID              string `json:"siteAgentID"`
	SlowStartRate            int    `json:"slowStartRate"`
	SourceSync               bool   `json:"sourceSync"`
	Status                   string `json:"status"`
	StatusDetailsHref        string `json:"statusDetailsHref"`
	SuccessfulEmailText      string `json:"successfulEmailText"`
	TaskID                   string `json:"taskId"`
	TaskName                 string `json:"taskName"`
	TaskPriority             int    `json:"taskPriority"`
	TransferEmptyDirectories bool   `json:"transferEmptyDirectories"`
	TransferMode             string `json:"transferMode"`
	UpperByteLimit           int64  `json:"upperByteLimit"`
	UseIncremental           bool   `json:"useIncremental"`
	UseSentFolder            bool   `json:"useSentFolder"`
	UseSlowStart             bool   `json:"useSlowStart"`
	UseSlowStartRate         bool   `json:"useSlowStartRate"`
	UseTempName              bool   `json:"useTempName"`
	UserEmailAddress         string `json:"userEmailAddress"`
	VerifyFileIntegrity      bool   `json:"verifyFileIntegrity"`
	VerifyMode               string `json:"verifyMode"`
	ZipFileSizeLimit         int64  `json:"zipFileSizeLimit"`
	DynamicFilesOptions      string `json:"dynamicFilesOptions"`
	DayFilterSwitchSelect    string `json:"dayFilterSwitchSelect"`
}

// requestTimeout bounds one call to the FileCatalyst REST API. Both calls exchange a
// single task configuration document.
const requestTimeout = 10 * time.Second

// newClient builds the client both calls in this package use. The credentials go in a
// RESTAuthorization header rather than through basic auth, which is what FileCatalyst
// asks for.
func newClient(baseURL, username, password string) *resty.Client {
	return httpx.New(httpx.Config{
		Service: "filecatalyst",
		BaseURL: baseURL,
		Timeout: requestTimeout,
		Headers: map[string]string{
			"Accept": "application/json",
			"RESTAuthorization": base64.StdEncoding.EncodeToString(
				[]byte(fmt.Sprintf("%s:%s", username, password))),
		},
	})
}

// taskPath is where one task's configuration lives.
func taskPath(taskID string) string {
	return "/rs/tasks/" + taskID
}

// UpdateFileCatalystTask updates a FileCatalyst task configuration
func UpdateFileCatalystTask(ctx context.Context, baseURL, taskID, username, password string, config FileCatalystTaskConfig) error {
	_, err := newClient(baseURL, username, password).R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(config).
		Post(taskPath(taskID))

	return err
}

// GetFileCatalystTask retrieves a FileCatalyst task configuration
func GetFileCatalystTask(ctx context.Context, baseURL, taskID, username, password string) (FileCatalystTaskConfig, error) {
	var config FileCatalystTaskConfig

	_, err := newClient(baseURL, username, password).R().
		SetContext(ctx).
		SetResult(&config).
		Get(taskPath(taskID))
	if err != nil {
		return FileCatalystTaskConfig{}, err
	}

	return config, nil
}

// UpdateCongestionControlAggression updates only the CongestionControlAggression field
func UpdateCongestionControlAggression(ctx context.Context, baseURL, taskID, username, password string, aggression int) error {
	config, err := GetFileCatalystTask(ctx, baseURL, taskID, username, password)
	if err != nil {
		return fmt.Errorf("failed to get task config: %w", err)
	}

	config.CongestionControlAggression = aggression

	return UpdateFileCatalystTask(ctx, baseURL, taskID, username, password, config)
}

// PokeFileCatalyst gets the current MB_Grow task config,
// randomly changes CongestionControlAggression (5-7, different from current), and updates it
func PokeFileCatalyst(ctx context.Context) error {
	baseURL := environment.Get().FileCatalyst.URL()
	taskID := environment.Get().FileCatalyst.TaskID()
	username := environment.Get().FileCatalyst.Username()
	password := environment.Get().FileCatalyst.Password()

	// Validate required environment variables
	if baseURL == "" || taskID == "" || username == "" || password == "" {
		return errors.New("missing required environment variables: FILECATALYST_URL, FILECATALYST_TASK_ID, FILECATALYST_USERNAME, FILECATALYST_PASSWORD")
	}

	// Get current configuration
	config, err := GetFileCatalystTask(ctx, baseURL, taskID, username, password)
	if err != nil {
		return fmt.Errorf("failed to get current task config: %w", err)
	}

	// Parse current aggression value
	currentAggression := config.CongestionControlAggression

	// Generate new random value between 5-7, but different from current
	var newAggression int
	for {
		newAggression = rand.Intn(3) + 5 // Random between 5-7
		if newAggression != currentAggression {
			break
		}
	}

	// Update the configuration
	config.CongestionControlAggression = newAggression

	// Send updated configuration back to server
	err = UpdateFileCatalystTask(ctx, baseURL, taskID, username, password, config)
	if err != nil {
		return fmt.Errorf("failed to update task config from %d to %d: %w",
			currentAggression, newAggression, err)
	}

	return nil
}
