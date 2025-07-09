package analytics

import (
	"fmt"
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

func (s *Service) ActivityStarted(activityName string, workerId string, queue string) {
	properties := map[string]interface{}{
		"activityName": activityName,
		"workerId":     workerId,
		"queue":        queue,
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

func (s *Service) ActivityFinished(activityName string, workerId string, queue string, succeeded bool, executionTime int64) {
	properties := map[string]interface{}{
		"activityName":  activityName,
		"workerId":      workerId,
		"queue":         queue,
		"succeeded":     succeeded,
		"executionTime": executionTime,
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
