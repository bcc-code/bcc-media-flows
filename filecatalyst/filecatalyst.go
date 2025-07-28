package filecatalyst

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"time"
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

// UpdateFileCatalystTask updates a FileCatalyst task configuration
func UpdateFileCatalystTask(baseURL, taskID, username, password string, config FileCatalystTaskConfig) error {
	url := fmt.Sprintf("%s/rs/tasks/%s", baseURL, taskID)

	jsonData, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set essential headers for the API request
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("RESTAuthorization", base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", username, password))))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("request failed with status: %d", resp.StatusCode)
	}

	return nil
}

// GetFileCatalystTask retrieves a FileCatalyst task configuration
func GetFileCatalystTask(baseURL, taskID, username, password string) (FileCatalystTaskConfig, error) {
	url := fmt.Sprintf("%s/rs/tasks/%s", baseURL, taskID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return FileCatalystTaskConfig{}, fmt.Errorf("failed to create request: %w", err)
	}

	// Set essential headers for the API request
	req.Header.Set("Accept", "application/json")
	req.Header.Set("RESTAuthorization", base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", username, password))))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return FileCatalystTaskConfig{}, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return FileCatalystTaskConfig{}, fmt.Errorf("request failed with status: %d", resp.StatusCode)
	}

	var config FileCatalystTaskConfig
	err = json.NewDecoder(resp.Body).Decode(&config)
	if err != nil {
		return FileCatalystTaskConfig{}, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return config, nil
}

// UpdateCongestionControlAggression updates only the CongestionControlAggression field
func UpdateCongestionControlAggression(baseURL, taskID, username, password string, aggression int) error {
	config, err := GetFileCatalystTask(baseURL, taskID, username, password)
	if err != nil {
		return fmt.Errorf("failed to get task config: %w", err)
	}

	config.CongestionControlAggression = aggression

	return UpdateFileCatalystTask(baseURL, taskID, username, password, config)
}

// PokeFileCatalyst gets the current MB_Grow task config,
// randomly changes CongestionControlAggression (5-7, different from current), and updates it
func PokeFileCatalyst() error {
	baseURL := os.Getenv("FILECATALYST_URL")
	taskID := os.Getenv("FILECATALYST_TASK_ID")
	username := os.Getenv("FILECATALYST_USERNAME")
	password := os.Getenv("FILECATALYST_PASSWORD")

	// Validate required environment variables
	if baseURL == "" || taskID == "" || username == "" || password == "" {
		return fmt.Errorf("missing required environment variables: FILECATALYST_URL, FILECATALYST_TASK_ID, FILECATALYST_USERNAME, FILECATALYST_PASSWORD")
	}

	// Get current configuration
	config, err := GetFileCatalystTask(baseURL, taskID, username, password)
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
	err = UpdateFileCatalystTask(baseURL, taskID, username, password, config)
	if err != nil {
		return fmt.Errorf("failed to update task config: %w", err)
	}

	fmt.Printf("Successfully updated CongestionControlAggression from %d to %d\n", currentAggression, newAggression)
	return nil
}
