package utils

import (
	"encoding/xml"
	"testing"
)

func Test_ToXml(t *testing.T) {
	type test struct {
		From string `xml:"from"`
	}

	expected := "<test></test>"

	actual, _ := xml.MarshalIndent(test{
		From: "quueue",
	}, "", "\t")

	if string(actual) != expected {

	}
}
