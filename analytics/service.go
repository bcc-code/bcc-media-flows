package analytics

import (
	"fmt"
	"os"
	"sync"
	"time"

	r "github.com/rudderlabs/analytics-go/v4"
)

var (
	Instance *Service
	once     sync.Once
)

func Init(config Config) {
	once.Do(func() {
		Instance = newService(config)
	})
}

func GetService() *Service {
	return Instance
}

type Service struct {
	rudderClient r.Client
}

type Config struct {
	WriteKey  string
	DataPlane string
	Verbose   bool
}

func newService(config Config) *Service {
	if config.WriteKey == "" || config.DataPlane == "" {
		fmt.Printf("WARN: Rudderstack is not configured, data will not be sent to Rudderstack")
	}

	c, err := r.NewWithConfig(config.WriteKey,
		r.Config{
			DataPlaneUrl: config.DataPlane,
			Interval:     1 * time.Second,
			BatchSize:    100,
			Verbose:      config.Verbose,
			DisableGzip:  false,
		})

	if err != nil {
		fmt.Printf("FATAL: Failed to create rudderstack client: %v", err)
		panic(err)
	}

	return &Service{
		rudderClient: c,
	}
}

func (s *Service) ActivityStarted(activityName string, queue string, parentWorkflow string) {
	identity := os.Getenv("IDENTITY")
	if identity == "" {
		identity = "worker"
	}

	properties := map[string]interface{}{
		"activityName":   activityName,
		"workerId":       identity,
		"queue":          queue,
		"parentWorkflow": parentWorkflow,
	}

	err := s.rudderClient.Enqueue(r.Track{
		Event:      "ActivityStarted",
		UserId:     "analytics",
		Properties: properties,
	})

	if err != nil {
		fmt.Printf("WARN: Failed to enqueue ActivityStarted event: %v\n", err)
	}
}

func (s *Service) ActivityFinished(activityName string, workerId string, queue string, parentWorkflow string, status bool, executionTime int64) {
	properties := map[string]interface{}{
		"activityName":   activityName,
		"workerId":       workerId,
		"queue":          queue,
		"parentWorkflow": parentWorkflow,
		"status":         status,
		"executionTime":  executionTime,
	}

	err := s.rudderClient.Enqueue(r.Track{
		Event:      "ActivityFinished",
		UserId:     "analytics",
		Properties: properties,
	})

	if err != nil {
		fmt.Printf("WARN: Failed to enqueue ActivityFinished event: %v\n", err)
	}
}

func (s *Service) WorkflowStarted(workflowName string, workflowId string, parentWorkflow string) {
	properties := map[string]interface{}{
		"workflowName":   workflowName,
		"workflowId":     workflowId,
		"parentWorkflow": parentWorkflow,
	}

	err := s.rudderClient.Enqueue(r.Track{
		Event:      "WorkflowStarted",
		UserId:     "analytics",
		Properties: properties,
	})

	if err != nil {
		fmt.Printf("WARN: Failed to enqueue WorkflowStarted event: %v\n", err)
	}
}

func (s *Service) WorkflowFinished(workflowName string, workflowId string, parentWorkflow string, status string, executionTime int64) {
	properties := map[string]interface{}{
		"workflowName":   workflowName,
		"workflowId":     workflowId,
		"parentWorkflow": parentWorkflow,
		"status":         status,
		"executionTime":  executionTime,
	}

	err := s.rudderClient.Enqueue(r.Track{
		Event:      "WorkflowFinished",
		UserId:     "analytics",
		Properties: properties,
	})

	if err != nil {
		fmt.Printf("WARN: Failed to enqueue WorkflowFinished event: %v\n", err)
	}
}
