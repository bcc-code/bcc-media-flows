package baton

import (
	"fmt"
	"github.com/bcc-code/bccm-flows/paths"
)

type StartTaskResult struct {
	TaskID string `json:"taskId"`
}

func StartTask(client *Client, filePath paths.Path, testPlan TestPlan) (*StartTaskResult, error) {
	req := client.restyClient.R()

	req.SetQueryParam("mediaFilePath", filePath.BatonPath())
	req.SetQueryParam("testPlan", testPlan.Value)
	req.SetResult(&StartTaskResult{})
	res, err := req.Post("tasks")
	if err != nil {
		return nil, err
	}
	return res.Result().(*StartTaskResult), nil
}

type TaskProgress struct {
	TaskID   string `json:"taskId"`
	Progress int    `json:"progress"`
}

func GetTaskProgress(client *Client, taskID string) (*TaskProgress, error) {
	req := client.restyClient.R()

	req.SetResult(&TaskProgress{})
	res, err := req.Get(fmt.Sprintf("tasks/%s/progress", taskID))
	if err != nil {
		return nil, err
	}
	return res.Result().(*TaskProgress), nil
}

type TaskInfoResult struct {
	TaskID   string    `json:"taskId"`
	TaskInfo *TaskInfo `json:"taskInfo"`
}

type TaskInfo struct {
	TaskID            string `json:"taskId"`
	TaskUrl           string `json:"taskURL"`
	Progress          int    `json:"progress"`
	ResultDescription string `json:"resultDescription"`
	AutoQCResult      string `json:"autoQCResult"`
}

func GetTaskInfo(client *Client, taskID string) (*TaskInfo, error) {
	req := client.restyClient.R()

	req.SetResult(&TaskInfoResult{})
	res, err := req.Get(fmt.Sprintf("tasks/%s/taskinfo", taskID))
	if err != nil {
		return nil, err
	}
	return res.Result().(*TaskInfoResult).TaskInfo, nil
}

type QCReport struct {
	Hierarchy []struct {
		Name     string `json:"name"`
		Children []struct {
			Info    int    `json:"Info"`
			Name    string `json:"name"`
			Parent  int    `json:"parent"`
			Warning int    `json:"Warning"`
			Error   int    `json:"Error"`
			Id      int    `json:"id"`
		} `json:"children"`
	} `json:"hierarchy"`
	TopLevelInfo struct {
		Info     int    `json:"Info"`
		FilePath string `json:"filePath"`
		Format   string `json:"format"`
		Summary  string `json:"summary"`
		Warning  int    `json:"Warning"`
		Error    int    `json:"Error"`
	} `json:"topLevelInfo"`
	StreamNodes []struct {
		Info            int    `json:"Info"`
		Name            string `json:"name"`
		Parent          int    `json:"parent"`
		EncodedDuration int    `json:"encodedDuration"`
		Warning         int    `json:"Warning"`
		Error           int    `json:"Error"`
		StartTimecode   int    `json:"startTimecode"`
		ErrorList       []struct {
			CheckId     string `json:"checkId"`
			Synopsis    string `json:"synopsis"`
			Type        string `json:"type"`
			Severity    string `json:"severity"`
			Description string `json:"description"`
		} `json:"errorList"`
		Id int `json:"id"`
	} `json:"streamNodes"`
	LastUpdatedISO string `json:"lastUpdated_ISO"`
	LastUpdated    string `json:"lastUpdated"`
}

func GetQCReport(client *Client, taskID string) (*QCReport, error) {
	req := client.restyClient.R()

	req.SetResult(&QCReport{})
	req.SetQueryParam("type", "json")
	res, err := req.Get(fmt.Sprintf("tasks/%s/report", taskID))
	if err != nil {
		return nil, err
	}
	return res.Result().(*QCReport), nil
}
