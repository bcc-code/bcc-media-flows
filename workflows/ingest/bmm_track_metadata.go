package ingestworkflows

import (
	"fmt"
	"net/url"
	"path"
	"strconv"

	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/services/telegram"
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vscommon"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	miscworkflows "github.com/bcc-code/bcc-media-flows/workflows/misc"
	"go.temporal.io/sdk/workflow"
)

type BmmContributor struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

type BmmAlbum struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type BmmCuratedPlaylist struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type BmmTrackMetadataParams struct {
	BmmTrackID      int                 `json:"bmmTrackId"`
	SongNumbers     []string            `json:"songNumbers"`
	Contributors    []BmmContributor    `json:"contributors"`
	Title           string              `json:"title"`
	Language        string              `json:"language"`
	PublishedDate   string              `json:"publishedDate"`
	RecordedAt      string              `json:"recordedAt,omitempty"`
	Copyright       string              `json:"copyright"`
	Album           BmmAlbum            `json:"album"`
	CuratedPlaylist *BmmCuratedPlaylist `json:"curatedPlaylist,omitempty"`
	Tags            []string            `json:"tags"`
	VXSource        string              `json:"vxSource,omitempty"`
	FileURL         string              `json:"fileUrl,omitempty"`
}

type BmmTrackMetadataResult struct {
	AssetID string `json:"assetId"`
}

type bmmTrackMetadataPayload struct {
	BmmTrackID      int                 `json:"bmmTrackId"`
	SongNumbers     []string            `json:"songNumbers"`
	Contributors    []BmmContributor    `json:"contributors"`
	Title           string              `json:"title"`
	Language        string              `json:"language"`
	PublishedDate   string              `json:"publishedDate"`
	RecordedAt      string              `json:"recordedAt,omitempty"`
	Copyright       string              `json:"copyright"`
	Album           BmmAlbum            `json:"album"`
	CuratedPlaylist *BmmCuratedPlaylist `json:"curatedPlaylist,omitempty"`
	Tags            []string            `json:"tags"`
}

func BmmTrackMetadata(ctx workflow.Context, params BmmTrackMetadataParams) (*BmmTrackMetadataResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting BmmTrackMetadata", "bmmTrackId", params.BmmTrackID, "vxSource", params.VXSource)

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	if params.VXSource == "" && params.FileURL == "" {
		return nil, fmt.Errorf("either VXSource or FileURL must be provided")
	}

	metadataJSON, err := wfutils.MarshalJson(ctx, bmmTrackMetadataPayload{
		BmmTrackID:      params.BmmTrackID,
		SongNumbers:     params.SongNumbers,
		Contributors:    params.Contributors,
		Title:           params.Title,
		Language:        params.Language,
		PublishedDate:   params.PublishedDate,
		RecordedAt:      params.RecordedAt,
		Copyright:       params.Copyright,
		Album:           params.Album,
		CuratedPlaylist: params.CuratedPlaylist,
		Tags:            params.Tags,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metadata JSON: %w", err)
	}

	assetID := params.VXSource
	ingested := false

	if assetID == "" {
		existing, err := wfutils.FindVidispineItemByMetadataField(ctx, vscommon.FieldBmmTrackID.Value, strconv.Itoa(params.BmmTrackID))
		if err != nil {
			return nil, fmt.Errorf("failed to search for existing VX item: %w", err)
		}
		if existing != "" {
			logger.Info("found existing VX item for BMM track; skipping import",
				"bmmTrackId", params.BmmTrackID, "assetId", existing)
			assetID = existing
		}
	}

	if assetID == "" {
		wfutils.SendTelegramText(ctx, telegram.ChatBMM, fmt.Sprintf("🟦 Importing BMM track to MB: `%d-%s`", params.BmmTrackID, params.Language))

		outputDir, err := wfutils.GetWorkflowRawOutputFolder(ctx)
		if err != nil {
			return nil, err
		}

		ext := extensionFromURL(params.FileURL)
		newPath := outputDir.Append(fmt.Sprintf("BMM-%d-%s%s", params.BmmTrackID, params.Language, ext))

		_, err = wfutils.Execute(ctx, activities.Util.DownloadFileFromURL, activities.DownloadFileFromURLInput{
			URL:         params.FileURL,
			Destination: newPath,
		}).Result(ctx)
		if err != nil {
			wfutils.SendTelegramErorr(ctx, telegram.ChatBMM, "", err)
			return nil, fmt.Errorf("failed to download file: %w", err)
		}

		title := fmt.Sprintf("BMM-%d %s - %s", params.BmmTrackID, params.Language, params.Title)
		res, err := ImportFileAsTag(ctx, "original", newPath, title)
		if err != nil {
			wfutils.SendTelegramErorr(ctx, telegram.ChatBMM, "", err)
			return nil, err
		}

		err = wfutils.WaitForVidispineJob(ctx, res.ImportJobID)
		if err != nil {
			wfutils.SendTelegramErorr(ctx, telegram.ChatBMM, res.AssetID, err)
			return nil, err
		}

		assetID = res.AssetID
		ingested = true
	}

	err = wfutils.SetVidispineMetaInGroup(ctx, assetID, vscommon.FieldBmmTrackMetadataJSON.Value, string(metadataJSON), "BMM Metadata")
	if err != nil {
		return nil, err
	}

	err = wfutils.SetVidispineMetaInGroup(ctx, assetID, vscommon.FieldBmmTrackID.Value, strconv.Itoa(params.BmmTrackID), "BMM Metadata")
	if err != nil {
		return nil, err
	}

	err = wfutils.SetVidispineMetaInGroup(ctx, assetID, vscommon.FieldBmmTitle.Value, params.Title, "BMM Metadata")
	if err != nil {
		return nil, err
	}

	if params.Language != "" {
		err = wfutils.SetVidispineMeta(ctx, assetID, vscommon.FieldLanguagesRecorded.Value, params.Language)
		if err != nil {
			return nil, err
		}
	}

	if ingested {
		err = workflow.ExecuteChildWorkflow(ctx, miscworkflows.TranscribeVX, miscworkflows.TranscribeVXInput{
			VXID:     assetID,
			Language: params.Language,
		}).Get(ctx, nil)
		if err != nil {
			logger.Warn("TranscribeVX failed; continuing", "error", err)
			wfutils.SendTelegramErorr(ctx, telegram.ChatBMM, assetID, err)
		}

		_ = CreatePreviews(ctx, []string{assetID})

		wfutils.SendTelegramText(ctx, telegram.ChatBMM, fmt.Sprintf("🟦 Created BMM track asset `%s` (`%d-%s`)", assetID, params.BmmTrackID, params.Language))
	}

	return &BmmTrackMetadataResult{AssetID: assetID}, nil
}

func extensionFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Path == "" {
		return ""
	}
	return path.Ext(u.Path)
}
