// Package bootstrap holds the startup steps every cmd/ entrypoint repeats:
// loading .env, dialing Temporal, naming itself, and serving HTTP.
package bootstrap

import (
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"go.temporal.io/sdk/client"
)

// LoadEnv reads .env into the environment. Call it first: anything that reads the
// environment before this runs sees the deployment's values and not the file's.
//
// A missing file is not an error — in production the values come from the deployment.
// A file that exists and will not parse is, and says so, which is the way round that
// is useful to whoever is looking at the logs.
func LoadEnv() {
	err := godotenv.Load(".env")
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return
	}

	log.Printf("WARNING: .env exists but could not be loaded: %v", err)
}

// TemporalClient dials the server named by TEMPORAL_HOST_PORT.
func TemporalClient() (client.Client, error) {
	host := os.Getenv("TEMPORAL_HOST_PORT")
	if host == "" {
		return nil, fmt.Errorf("TEMPORAL_HOST_PORT is required")
	}

	return client.Dial(client.Options{
		HostPort:  host,
		Namespace: os.Getenv("TEMPORAL_NAMESPACE"),
	})
}

// Identity names this process in analytics and worker registration.
func Identity() string {
	if identity := os.Getenv("IDENTITY"); identity != "" {
		return identity
	}
	return "worker"
}

// Serve runs router on PORT, or on defaultPort when PORT is unset.
func Serve(router *gin.Engine, defaultPort string) error {
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	fmt.Printf("Started on port %s\n", port)
	return router.Run(":" + port)
}
