package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"

	"github.com/bcc-code/bcc-media-flows/workflows"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/google/uuid"
	"github.com/invopop/jsonschema"
	"github.com/samber/lo"
	"go.temporal.io/sdk/client"
)

type WorkflowSchema struct {
	Name   string             `json:"name"`
	Schema *jsonschema.Schema `json:"schema"`
}

func getWorkflowSchemas(ctx *gin.Context) {
	var schemas []WorkflowSchema

	for _, wf := range workflows.TriggerableWorkflows {
		typ := reflect.TypeOf(wf)
		if typ.NumIn() > 1 {
			name, _ := getFunctionName(wf)
			arg2Type := typ.In(1)
			// log the name of the type
			fmt.Printf("Type: %s\n", arg2Type.Name())
			// log the fields of the type
			for i := 0; i < arg2Type.NumField(); i++ {
				field := arg2Type.Field(i)
				fmt.Printf("Type: %s, Field: %s\n", arg2Type.Name(), field.Name)
			}

			schema := jsonschema.ReflectFromType(arg2Type)
			fmt.Printf("Type: %s, Schema: %v\n", arg2Type.Name(), schema)
			fmt.Println()
			schemas = append(schemas, WorkflowSchema{
				Name:   name,
				Schema: schema,
			})
		}
	}

	ctx.JSON(http.StatusOK, schemas)
}

func triggerDynamicHandler(ctx *gin.Context) {
	wfClient, err := getClient()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	defer wfClient.Close()

	queue := getQueue()
	workflowOptions := client.StartWorkflowOptions{
		ID:        uuid.NewString(),
		TaskQueue: queue,
	}

	// use reflection to trigger an aribtrary workflow
	workflowName := getParamFromCtx(ctx, "workflow")
	if workflowName == "" {
		ctx.Status(http.StatusBadRequest)
		return
	}
	var rawMessage json.RawMessage
	if err := ctx.ShouldBindBodyWith(&rawMessage, binding.JSON); err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}

	workflows := workflows.TriggerableWorkflows
	wf, found := lo.Find(workflows, func(wf any) bool {
		name, _ := getFunctionName(wf)
		return name == workflowName
	})
	if !found {
		ctx.Status(http.StatusNotFound)
		return
	}

	typ := reflect.TypeOf(wf)
	var arg2 interface{}
	if typ.In(1).Kind() == reflect.Ptr {
		// If the second argument is a pointer, we need to create a new instance of the underlying type
		arg2 = reflect.New(typ.In(1).Elem()).Interface()
	} else {
		// Otherwise, we can just create a new instance of the type itself
		arg2 = reflect.New(typ.In(1)).Interface()
	}
	err = json.Unmarshal(rawMessage, arg2)
	if err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}

	// If arg2 was not a pointer, we need to dereference it before passing to ExecuteWorkflow
	if typ.In(1).Kind() != reflect.Ptr {
		arg2 = reflect.ValueOf(arg2).Elem().Interface()
	}

	res, err := wfClient.ExecuteWorkflow(ctx, workflowOptions, wf, arg2)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, res)
}
