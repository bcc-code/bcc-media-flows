package wfutils

import (
	"encoding/json"
	"encoding/xml"
	"go.temporal.io/sdk/workflow"
)

func MarshalXml(ctx workflow.Context, data any) ([]byte, error) {
	var res []byte
	err := workflow.SideEffect(ctx, func(ctx workflow.Context) any {
		xmlData, err := xml.MarshalIndent(data, "", "\t")
		if err != nil {
			panic(err)
		}

		return xmlData
	}).Get(&res)
	return res, err
}

func MarshalJson(ctx workflow.Context, data any) ([]byte, error) {
	var res []byte
	err := workflow.SideEffect(ctx, func(ctx workflow.Context) any {
		bytes, err := json.Marshal(data)
		if err != nil {
			panic(err)
		}
		return bytes
	}).Get(res)
	return res, err
}

func UnmarshalJson[T any](ctx workflow.Context, data []byte) (*T, error) {
	var res *T
	err := workflow.SideEffect(ctx, func(ctx workflow.Context) any {
		var res *T
		err := json.Unmarshal(data, res)
		if err != nil {
			panic(err)
		}
		return nil
	}).Get(res)

	return res, err
}
