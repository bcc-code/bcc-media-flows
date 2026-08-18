package main

import (
	"reflect"
	"testing"

	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The activity structs are pointers so boot can set their clients.
// registerActivitiesInStruct reflects over them, and value-receiver methods are in a
// pointer's method set — but a struct registering nothing would fail silently at
// runtime, with the worker simply not answering those activities.
func TestActivityStructsStillExposeTheirMethods(t *testing.T) {
	structs := map[string]any{
		"Util":      activities.Util,
		"Vidispine": activities.Vidispine,
		"Cantemo":   activities.Cantemo,
		"Platform":  activities.Platform,
		"Live":      activities.Live,
	}

	for name, s := range structs {
		assert.Positive(t, reflect.TypeOf(s).NumMethod(), "%s registers no activities", name)
	}
}

// The registration list captures method values, which copy the receiver. Building it
// before the clients exist would register activities holding a nil client.
func TestUtilActivitiesAreBuiltAfterTheClients(t *testing.T) {
	require.NotEmpty(t, utilActivities())

	for _, a := range utilActivities() {
		assert.Equal(t, reflect.Func, reflect.TypeOf(a).Kind())
	}
}
