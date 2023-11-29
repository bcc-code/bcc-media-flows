package main

import (
	"net/http"
	"path/filepath"
	"strings"

	"github.com/bcc-code/bccm-flows/paths"
	"github.com/bcc-code/bccm-flows/services/ingest"
	ingestworkflows "github.com/bcc-code/bccm-flows/workflows/ingest"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.temporal.io/sdk/client"
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

func (s *TriggerServer) uploadMasterAdminGET(c *gin.Context) {
	programIDs, err := s.getProgramIDs()
	if err != nil {
		renderErrorPage(c, http.StatusInternalServerError, err)
		return
	}

	c.HTML(http.StatusOK, "upload-master-admin.gohtml", gin.H{
		"programIds": programIDs,
	})
}

func (s *TriggerServer) uploadMasterAdminPOST(c *gin.Context) {
	addIDs := []string{c.PostForm("code") + " - " + c.PostForm("name")}

	if addIDs[0] != " - " {
		for _, id := range addIDs {
			err := s.addProgramID(id)
			if err != nil {
				renderErrorPage(c, http.StatusInternalServerError, err)
				return
			}
		}
	}

	removeIDs := c.PostFormArray("deleteIds[]")

	if removeIDs != nil {
		for _, id := range removeIDs {
			err := s.removeProgramID(id)
			if err != nil {
				renderErrorPage(c, http.StatusInternalServerError, err)
				return
			}
		}
	}

	s.uploadMasterAdminGET(c)
}

func (s *TriggerServer) uploadMasterGET(c *gin.Context) {
	filenames, err := getFilenames(masterTriggerDir)
	if err != nil {
		renderErrorPage(c, http.StatusInternalServerError, err)
		return
	}

	tags, err := s.getTags()
	if err != nil {
		renderErrorPage(c, http.StatusInternalServerError, err)
		return
	}

	persons, err := s.getPersons()
	if err != nil {
		renderErrorPage(c, http.StatusInternalServerError, err)
		return
	}

	programIDs, err := s.getProgramIDs()
	if err != nil {
		renderErrorPage(c, http.StatusInternalServerError, err)
		return
	}

	c.HTML(http.StatusOK, "upload-master.gohtml", gin.H{
		"files":      filenames,
		"tags":       tags,
		"persons":    persons,
		"programIds": programIDs,
	})
}

func (s *TriggerServer) uploadMasterPOST(c *gin.Context) {
	queue := getQueue()
	workflowOptions := client.StartWorkflowOptions{
		ID:        uuid.NewString(),
		TaskQueue: queue,
	}

	for _, tag := range c.PostFormArray("tags[]") {
		err := s.addTag(tag)
		if err != nil {
			renderErrorPage(c, http.StatusInternalServerError, err)
			return
		}
	}
	for _, person := range c.PostFormArray("persons[]") {
		err := s.addPerson(person)
		if err != nil {
			renderErrorPage(c, http.StatusInternalServerError, err)
			return
		}
	}

	rawPath := filepath.Join(masterTriggerDir, c.PostForm("path"))
	path, err := paths.Parse(rawPath)
	if err != nil {
		renderErrorPage(c, http.StatusBadRequest, err)
		return
	}

	_, err = s.wfClient.ExecuteWorkflow(c, workflowOptions, ingestworkflows.Masters, ingestworkflows.MasterParams{
		Metadata: &ingest.Metadata{
			JobProperty: ingest.JobProperty{
				ProgramID:        c.PostForm("programId"),
				Tags:             strings.Join(c.PostFormArray("tags[]"), ", "),
				PersonsAppearing: strings.Join(c.PostFormArray("persons[]"), ", "),
				SenderEmail:      c.PostForm("senderEmail"),
				Language:         c.PostForm("language"),
				ReceivedFilename: c.PostForm("filename"),
			},
		},
		SourceFile: &path,
	})
	if err != nil {
		renderErrorPage(c, http.StatusInternalServerError, err)
		return
	}

}
