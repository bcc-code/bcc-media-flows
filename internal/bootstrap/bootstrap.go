// Package bootstrap holds the startup steps every cmd/ entrypoint repeats:
// loading .env, dialing Temporal, naming itself, and serving HTTP.
package bootstrap

import (
	"errors"
	"fmt"
	"github.com/bcc-code/bcc-media-flows/environment"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"go.temporal.io/sdk/client"
)

// LoadEnv reads .env, and must run before anything reads the environment. A missing
// file is fine; one that will not parse is not.
func LoadEnv() {
	err := godotenv.Load(".env")
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return
	}

	log.Printf("WARNING: .env exists but could not be loaded: %v", err)
}

// TemporalClient dials the server named by TEMPORAL_HOST_PORT.
func TemporalClient() (client.Client, error) {
	host := environment.Get().Temporal.HostPort()
	if host == "" {
		return nil, fmt.Errorf("TEMPORAL_HOST_PORT is required")
	}

	return client.Dial(client.Options{
		HostPort:  host,
		Namespace: environment.Get().Temporal.Namespace(),
	})
}

// Identity names this process in analytics and worker registration.
func Identity() string {
	if identity := environment.Get().Identity; identity != "" {
		return identity
	}
	return "worker"
}

// Serve runs router on PORT, or on defaultPort when PORT is unset.
func Serve(router *gin.Engine, defaultPort string) error {
	port := environment.Get().Port
	if port == "" {
		port = defaultPort
	}

	fmt.Printf("Started on port %s\n", port)
	return router.Run(":" + port)
}
