package main

import (
	"github.com/bcc-code/bcc-media-flows/environment"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/ingest"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	ingestworkflows "github.com/bcc-code/bcc-media-flows/workflows/ingest"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

func (s *TriggerServer) getPersons() ([]string, error) {
	return singleValueArrayFromRows(s.database.Query("SELECT name FROM persons"))
}

func (s *TriggerServer) addPerson(id string) error {
	_, err := s.database.Exec("INSERT INTO persons (name) VALUES (?) ON CONFLICT DO NOTHING", id)
	return err
}

func (s *TriggerServer) getTags() ([]string, error) {
	return singleValueArrayFromRows(s.database.Query("SELECT name FROM tags"))
}

func (s *TriggerServer) addTag(id string) error {
	_, err := s.database.Exec("INSERT INTO tags (name) VALUES (?) ON CONFLICT DO NOTHING", id)
	return err
}

func (s *TriggerServer) getProgramIDs() ([]string, error) {
	return singleValueArrayFromRows(s.database.Query("SELECT name FROM program_ids"))
}

func (s *TriggerServer) addProgramID(id string) error {
	_, err := s.database.Exec("INSERT INTO program_ids (name) VALUES (?) ON CONFLICT DO NOTHING", id)
	return err
}

func (s *TriggerServer) removeProgramID(id string) error {
	_, err := s.database.Exec("DELETE FROM program_ids WHERE name = ?", id)
	return err
}

func (s *TriggerServer) uploadMasterAdminGET(ctx *gin.Context) {
	programIDs, err := s.getProgramIDs()
	if err != nil {
		renderErrorPage(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.HTML(http.StatusOK, "upload-master-admin.gohtml", gin.H{
		"programIds": programIDs,
	})
}

func (s *TriggerServer) uploadMasterAdminPOST(ctx *gin.Context) {
	code := ctx.PostForm("code")
	name := ctx.PostForm("name")

	if code != "" && name != "" {
		err := s.addProgramID(strings.ToUpper(code) + " - " + name)
		if err != nil {
			renderErrorPage(ctx, http.StatusInternalServerError, err)
			return
		}
	}

	for _, id := range ctx.PostFormArray("deleteIds[]") {
		err := s.removeProgramID(id)
		if err != nil {
			renderErrorPage(ctx, http.StatusInternalServerError, err)
			return
		}
	}

	s.uploadMasterAdminGET(ctx)
}

func (s *TriggerServer) uploadMasterGET(ctx *gin.Context) {
	filenames, err := getFilenames(environment.Get().MasterTriggerDir)
	if err != nil {
		renderErrorPage(ctx, http.StatusInternalServerError, err)
		return
	}

	tags, err := s.getTags()
	if err != nil {
		renderErrorPage(ctx, http.StatusInternalServerError, err)
		return
	}

	persons, err := s.getPersons()
	if err != nil {
		renderErrorPage(ctx, http.StatusInternalServerError, err)
		return
	}

	programIDs, err := s.getProgramIDs()
	if err != nil {
		renderErrorPage(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.HTML(http.StatusOK, "upload-master.gohtml", gin.H{
		"files":      filenames,
		"tags":       tags,
		"persons":    persons,
		"programIds": programIDs,
	})
}

type MasterPostParams struct {
	ProgramID          string   `form:"programId"`
	Tags               []string `form:"tags[]"`
	Persons            []string `form:"persons[]"`
	Path               string   `form:"path"`
	SenderEmail        string   `form:"senderEmail"`
	Language           string   `form:"language"`
	Filename           string   `form:"filename"`
	Episode            string   `form:"episode"`
	EpisodeTitle       string   `form:"episodeTitle"`
	EpisodeDescription string   `form:"episodeDescription"`
	DirectToPlayback   bool     `form:"directToPlayback"`
}

func (s *TriggerServer) uploadMasterPOST(ctx *gin.Context) {
	var params MasterPostParams
	err := ctx.ShouldBindWith(&params, binding.Form)
	if err != nil {
		renderErrorPage(ctx, http.StatusInternalServerError, err)
		return
	}

	// No VXID yet — the asset is created mid-workflow and upserted there.
	workflowOptions := wfutils.NewWorkflowOptions(getQueue(), "", getTriggeredBy(ctx))

	for _, tag := range params.Tags {
		err := s.addTag(tag)
		if err != nil {
			renderErrorPage(ctx, http.StatusInternalServerError, err)
			return
		}
	}
	for _, person := range params.Persons {
		err := s.addPerson(person)
		if err != nil {
			renderErrorPage(ctx, http.StatusInternalServerError, err)
			return
		}
	}

	rawPath := filepath.Join(environment.Get().MasterTriggerDir, params.Path)
	path, err := paths.Parse(rawPath)
	if err != nil {
		renderErrorPage(ctx, http.StatusBadRequest, err)
		return
	}

	res, err := s.wfClient.ExecuteWorkflow(ctx, workflowOptions, ingestworkflows.Masters, ingestworkflows.MasterParams{
		Metadata: &ingest.Metadata{
			JobProperty: ingest.JobProperty{
				ProgramID:          params.ProgramID,
				Tags:               strings.Join(params.Tags, ", "),
				PersonsAppearing:   strings.Join(params.Persons, ", "),
				SenderEmail:        params.SenderEmail,
				Language:           params.Language,
				ReceivedFilename:   params.Filename,
				EpisodeDescription: params.EpisodeDescription,
				EpisodeTitle:       params.EpisodeTitle,
				Episode:            params.Episode,
			},
		},
		SourceFile: &path,
	})
	if err != nil {
		renderErrorPage(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.HTML(http.StatusOK, "success.gohtml", gin.H{
		"WorkflowID": res.GetID(),
		"Title":      "Upload master",
	})
}
