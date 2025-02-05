package miscworkflows

import (
	"context"
	"fmt"
	"github.com/bcc-code/bcc-media-flows/activities/cantemo"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/bcc-code/bcc-media-flows/environment"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"github.com/davecgh/go-spew/spew"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
	"path"
	"strings"
	"time"
)

type ContextKey string

const ClientContextKey ContextKey = "Client"

type MoveMBFileParams struct {
	VXID               string
	Shapes             []string
	DestinationStorage string
}

func MoveMBFile(ctx workflow.Context, params MoveMBFileParams) error {

	activityCtx := workflow.WithLocalActivityOptions(ctx, workflow.LocalActivityOptions{
		ScheduleToCloseTimeout: time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    5,
		},
	})

	err := workflow.ExecuteLocalActivity(activityCtx,
		StartFilesWorkerFlow, params,
	).Get(ctx, nil)

	return err
}

func StartFilesWorkerFlow(ctx context.Context, params MoveMBFileParams) error {
	c := ctx.Value(ClientContextKey).(client.Client)
	_, _ = c.SignalWithStartWorkflow(
		context.Background(),
		"move_mb_file",
		MoveMBFileSignalName,
		params,
		client.StartWorkflowOptions{
			ID:        "move_mb_file",
			TaskQueue: environment.GetQueue(),
			RetryPolicy: &temporal.RetryPolicy{
				InitialInterval:    time.Second,
				BackoffCoefficient: 2.0,
				MaximumInterval:    time.Minute,
				MaximumAttempts:    5,
			},
		},
		MoveFilesWorkerFlow,
	)

	return nil
}

const MoveMBFileSignalName = "move_mb_file"

type MBStorage struct {
	VXID     string
	BasePath string
	Name     string
}

var Storages = []MBStorage{
	{
		VXID:     "VX-1",
		BasePath: "/srv/media/media1/",
		Name:     "media1",
	},
	{
		VXID:     "VX-4",
		BasePath: "/srv/space/Production/",
		Name:     "space_production",
	},
	{
		VXID:     "VX-5",
		BasePath: "/srv/media/fcserver/Media/",
		Name:     "fcs_media",
	},
	{
		VXID:     "VX-6",
		BasePath: "/srv/media/fcserver/masters/",
		Name:     "fcs_masters",
	},
	{
		VXID:     "VX-7",
		BasePath: "/srv/media/fcserver/Roughcuts/",
		Name:     "fcs_roughcuts",
	},
	{
		VXID:     "VX-41",
		BasePath: "/mnt/isilon/system/ingestgrow/",
		Name:     "Isilon Ingestgrow",
	},
	{
		VXID:     "VX-42",
		BasePath: "/mnt/isilon/Production/",
		Name:     "Isilon Production",
	},
	{
		VXID:     "VX-47",
		BasePath: "/mnt/isilon/system/tempingest/",
		Name:     "Isilon Tempingest",
	},
	{
		VXID:     "VX-56",
		BasePath: "/mnt/isilon/Master/",
		Name:     "Isilon Master",
	},
	{
		VXID:     "VX-82",
		BasePath: "/mnt/archive/",
		Name:     "Archive",
	},
}

func FindStorageForPath(path string) *MBStorage {
	for _, s := range Storages {
		if strings.HasPrefix(path, s.BasePath) {
			return &s
		}
	}
	return nil
}

func FindStorageForVXID(vxid string) *MBStorage {
	for _, s := range Storages {
		if s.VXID == vxid {
			return &s
		}
	}
	return nil
}

func MoveFilesWorkerFlow(ctx workflow.Context) error {
	ch := workflow.GetSignalChannel(ctx, MoveMBFileSignalName)

	msg := &MoveMBFileParams{}

	for {
		ok, _ := ch.ReceiveWithTimeout(ctx, 10*time.Second, msg)
		if !ok {
			break
		}

		dstStorage := FindStorageForVXID(msg.DestinationStorage)
		if dstStorage == nil {
			workflow.GetLogger(ctx).Error("Failed to find destination storage", "vxid", msg.DestinationStorage)
			continue
		}

		meta, err := wfutils.Execute(ctx, vsactivity.Vidispine.GetShapes, vsactivity.VXOnlyParam{VXID: msg.VXID}).Result(ctx)

		if err != nil {
			workflow.GetLogger(ctx).Error("Failed to get shapes", "error", err)
			continue
		}

		for _, shapeTag := range msg.Shapes {
			s := meta.GetShape(shapeTag)

			if s == nil {
				workflow.GetLogger(ctx).Debug("No shape found for tag", "tag", shapeTag, "vxid", msg.VXID)
				continue
			}

			shapePath := s.GetPath()
			storage := FindStorageForPath(shapePath)

			trimmedName := strings.TrimPrefix(shapePath, storage.BasePath)

			depth := strings.Count(trimmedName, "/")
			if depth < 2 {
				ts, err := time.Parse("2006-01-02T15:04:05.000-0700", s.Created)
				if err == nil {
					datePath := fmt.Sprintf("%04d/%02d/%02d", ts.Year(), ts.Month(), ts.Day())
					trimmedName = path.Join(datePath, trimmedName)
				}
			}

			newPath := trimmedName

			renameParams := cantemo.RenameFileParams{
				ItemID:            msg.VXID,
				ShapeID:           s.ID,
				SourceStorage:     storage.VXID,
				DestinatinStorage: dstStorage.VXID,
				NewPath:           newPath,
			}
			spew.Dump(renameParams)

			err := wfutils.Execute(ctx, cantemo.MoveFileWait, &renameParams).Wait(ctx)
			if err != nil {
				workflow.GetLogger(ctx).Error("Failed to rename file", "error", err)
				continue
			}
		}
	}

	return nil
}
