package rclone

import (
	"fmt"
	cache "github.com/Code-Hex/go-generics-cache"
	"net/http"
	"time"
)

type CoreStats struct {
	Bytes               int64          `json:"bytes"`
	Checks              int            `json:"checks"`
	DeletedDirs         int            `json:"deletedDirs"`
	Deletes             int            `json:"deletes"`
	ElapsedTime         float64        `json:"elapsedTime"`
	Errors              int            `json:"errors"`
	Eta                 int            `json:"eta"`
	FatalError          bool           `json:"fatalError"`
	LastError           string         `json:"lastError"`
	Renames             int            `json:"renames"`
	RetryError          bool           `json:"retryError"`
	ServerSideCopies    int            `json:"serverSideCopies"`
	ServerSideCopyBytes int            `json:"serverSideCopyBytes"`
	ServerSideMoveBytes int            `json:"serverSideMoveBytes"`
	ServerSideMoves     int            `json:"serverSideMoves"`
	Speed               float64        `json:"speed"`
	TotalBytes          int64          `json:"totalBytes"`
	TotalChecks         int            `json:"totalChecks"`
	TotalTransfers      int            `json:"totalTransfers"`
	TransferTime        float64        `json:"transferTime"`
	Transferring        []Transferring `json:"transferring"`
	Transfers           int            `json:"transfers"`
}

type Transferring struct {
	Bytes      int64   `json:"bytes"`
	DstFs      string  `json:"dstFs"`
	Eta        int     `json:"eta"`
	Group      string  `json:"group"`
	Name       string  `json:"name"`
	Percentage int     `json:"percentage"`
	Size       int64   `json:"size"`
	Speed      float64 `json:"speed"`
	SpeedAvg   float64 `json:"speedAvg"`
	SrcFs      string  `json:"srcFs"`
}

func (s CoreStats) ForJob(job string) Transferring {
	for _, transfer := range s.Transferring {
		if transfer.Group == "job/"+job {
			return transfer
		}
	}

	return Transferring{
		Bytes:      -1,
		Eta:        -1,
		Percentage: -1,
		Size:       -1,
		Speed:      -1,
		SpeedAvg:   -1,
	}
}

var (
	statsCache = cache.New[string, Transferring](cache.WithJanitorInterval[string, Transferring](time.Minute * 1))
)

func GetJobStats(job int) (Transferring, bool) {
	return statsCache.Get(fmt.Sprintf("job/%d", job))
}

func GetRcloneStatus() (*CoreStats, error) {
	req, err := http.NewRequest(http.MethodPost, baseUrl+"/core/stats", nil)
	if err != nil {
		return nil, err
	}

	res, err := doRequest[CoreStats](req)
	if err != nil {
		return nil, err
	}

	if res.Transferring == nil {
		res.Transferring = []Transferring{}
	}

	for _, t := range res.Transferring {
		statsCache.Set(t.Group, t, cache.WithExpiration(time.Minute*60))
	}
	return res, nil
}
