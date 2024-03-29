package workflows

import (
	"fmt"
	"strings"
	"time"

	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/paths"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"go.temporal.io/sdk/workflow"
)

type HandleMultitrackFileInput struct {
	Path string
}

func makeLucidMultitrackPath(ctx workflow.Context, path paths.Path) paths.Path {
	out := paths.Path{
		Drive: paths.LucidLinkDrive,
	}

	path, _ = wfutils.StandardizeFileName(ctx, path)

	if path.Drive == paths.IsilonDrive {
		out.Path = strings.Replace(path.Path, "system/multitrack/Ingest/tempFraBrunstad", "", 1)
	} else if path.Drive == paths.DMZShareDrive {
		out.Path = strings.Replace(path.Path, "multitrack/Ingest/tempFraBrunstad", "", 1)
	}

	return out.Append(path.Base()).Prepend("01 Liveopptak fra Brunstad/01 RAW/" + time.Now().Format("2006-01-02"))
}

func makeMultitrackIsilonArchivePath(ctx workflow.Context, path paths.Path) paths.Path {
	out := paths.Path{
		Drive: paths.IsilonDrive,
	}

	path, _ = wfutils.StandardizeFileName(ctx, path)

	if path.Drive == paths.IsilonDrive {
		out.Path = strings.Replace(path.Dir().Path, "system/multitrack/Ingest/tempFraBrunstad", "", 1)
	} else if path.Drive == paths.DMZShareDrive {
		out.Path = strings.Replace(path.Dir().Path, "multitrack/Ingest/tempFraBrunstad", "", 1)
	}

	return out.Prepend(fmt.Sprintf("AudioArchive/%d/%d", time.Now().Year(), time.Now().Month())).Append(path.Base())
}

func HandleMultitrackFile(
	ctx workflow.Context,
	params HandleMultitrackFileInput,
) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting HandleMultitrackFile workflow")

	options := wfutils.GetDefaultActivityOptions()
	ctx = workflow.WithActivityOptions(ctx, options)

	path, err := paths.Parse(params.Path)
	if err != nil {
		return err
	}

	lucidPath := makeLucidMultitrackPath(ctx, path)

	jobID, err := wfutils.ExecuteWithLowPrioQueue(ctx, activities.Util.RcloneCopyFile, activities.RcloneFileInput{
		Source:      path,
		Destination: lucidPath,
	}).Result(ctx)
	if err != nil {
		return err
	}

	_, err = wfutils.ExecuteWithLowPrioQueue(ctx, activities.Util.RcloneWaitForJob, jobID).Result(ctx)
	if err != nil {
		return err
	}

	isilonArchivePath := makeMultitrackIsilonArchivePath(ctx, path)
	err = wfutils.ExecuteWithLowPrioQueue(ctx, activities.Util.MoveFile, activities.MoveFileInput{
		Source:      path,
		Destination: isilonArchivePath,
	}).Get(ctx, nil)

	return err
}
