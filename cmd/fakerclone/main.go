package main

import (
	"fmt"
	"github.com/bcc-code/bcc-media-flows/services/rclone"
	"github.com/davecgh/go-spew/spew"
	"github.com/gin-gonic/gin"
	"net/http"
	"os"
	"time"
)

func jobStatusHandler(c *gin.Context) {
	req := &rclone.JobStatusRequest{}
	err := c.BindJSON(req)
	if err != nil {
		spew.Dump(err)
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, rclone.JobStatus{
		Duration: 10,
		EndTime:  time.Now(),
		Error:    "",
		Finished: true,
		Group:    "",
		ID:       req.JobID,
		Output: rclone.Output{
			Bytes:               1000,
			Checks:              1,
			DeletedDirs:         0,
			Deletes:             0,
			ElapsedTime:         10,
			Errors:              0,
			Eta:                 0,
			FatalError:          false,
			LastError:           "",
			Renames:             0,
			RetryError:          false,
			ServerSideCopies:    0,
			ServerSideCopyBytes: 0,
			ServerSideMoveBytes: 0,
			ServerSideMoves:     0,
			Speed:               0,
			TotalBytes:          1000,
			TotalChecks:         1,
			TotalTransfers:      1,
			TransferTime:        10,
			Transfers:           1,
		},
		StartTime: time.Now(),
		Success:   true,
	})
}

func operationsListHandler(c *gin.Context) {
	req := &rclone.ListRequest{}
	err := c.BindJSON(req)
	if err != nil {
		spew.Dump(err)
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rclone.ListResponse{
		List: []rclone.RcloneFile{},
	})
}

func operationsStatHandler(c *gin.Context) {
	req := &rclone.ListRequest{}
	err := c.BindJSON(req)
	if err != nil {
		spew.Dump(err)
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, rclone.StatsResponse{File: &rclone.RcloneFile{}})
}

func operationCoreStatsHandler(c *gin.Context) {
	c.JSON(http.StatusOK, rclone.CoreStats{
		Bytes:       0,
		Checks:      0,
		DeletedDirs: 0,
		Deletes:     0,
		ElapsedTime: 0,
		Errors:      0,
		Eta:         0,
		FatalError:  false,
		LastError:   "",
		Renames:     0,
		RetryError:  false,

		ServerSideCopies:    0,
		ServerSideCopyBytes: 0,
		ServerSideMoveBytes: 0,
		ServerSideMoves:     0,
		Speed:               0,
		TotalBytes:          0,
		TotalChecks:         0,
		TotalTransfers:      0,
		TransferTime:        0,
		Transfers:           0,
		Transferring:        make([]rclone.Transferring, 0),
	})
}

func operationsCopyFileHandler(c *gin.Context) {
	c.JSON(http.StatusOK, rclone.JobResponse{
		JobID: 123,
	})
}

func main() {
	router := gin.Default()
	router.POST("/job/status", jobStatusHandler)
	router.POST("/operations/list", operationsListHandler)
	router.POST("/operations/stat", operationsStatHandler)
	router.POST("/core/stats", operationCoreStatsHandler)
	router.POST("/operations/copyfile", operationsCopyFileHandler)
	router.POST("/operations/movefile", operationsCopyFileHandler)
	router.POST("/sync/copy", operationsCopyFileHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8084"
	}

	fmt.Printf("Started on port %s", port)
	err := router.Run(fmt.Sprintf(":%s", port))
	if err != nil {
		panic(err)
	}
}
