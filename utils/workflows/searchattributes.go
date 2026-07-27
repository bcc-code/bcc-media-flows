package wfutils

import (
	"context"
	"os"

	"github.com/google/uuid"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/operatorservice/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

var (
	VXIDKey        = temporal.NewSearchAttributeKeyKeyword("VXID")
	TriggeredByKey = temporal.NewSearchAttributeKeyKeyword("TriggeredBy")
	// Legacy attribute kept so existing UI queries keep working.
	LegacyVXIDKey = temporal.NewSearchAttributeKeyString("CustomStringField")
)

// EnsureSearchAttributes registers the custom search attributes on the
// namespace if they are missing. Meant to run once at worker startup.
func EnsureSearchAttributes(ctx context.Context, c client.Client) error {
	namespace := os.Getenv("TEMPORAL_NAMESPACE")
	resp, err := c.OperatorService().ListSearchAttributes(ctx, &operatorservice.ListSearchAttributesRequest{
		Namespace: namespace,
	})
	if err != nil {
		return err
	}
	wanted := map[string]enums.IndexedValueType{
		VXIDKey.GetName():        enums.INDEXED_VALUE_TYPE_KEYWORD,
		TriggeredByKey.GetName(): enums.INDEXED_VALUE_TYPE_KEYWORD,
		// Newer servers no longer pre-register the Custom* attributes.
		LegacyVXIDKey.GetName(): enums.INDEXED_VALUE_TYPE_TEXT,
	}
	missing := map[string]enums.IndexedValueType{}
	for name, typ := range wanted {
		if _, ok := resp.CustomAttributes[name]; !ok {
			missing[name] = typ
		}
	}
	if len(missing) == 0 {
		return nil
	}
	_, err = c.OperatorService().AddSearchAttributes(ctx, &operatorservice.AddSearchAttributesRequest{
		Namespace:        namespace,
		SearchAttributes: missing,
	})
	return err
}

// TypedSearchAttributes builds the standard attribute set for workflow
// starts. Empty values are omitted.
func TypedSearchAttributes(vxID, triggeredBy string) temporal.SearchAttributes {
	var updates []temporal.SearchAttributeUpdate
	if vxID != "" {
		updates = append(updates, VXIDKey.ValueSet(vxID), LegacyVXIDKey.ValueSet(vxID))
	}
	if triggeredBy != "" {
		updates = append(updates, TriggeredByKey.ValueSet(triggeredBy))
	}
	return temporal.NewSearchAttributes(updates...)
}

// NewWorkflowOptions is the shared options helper for root workflow starts.
func NewWorkflowOptions(queue, vxID, triggeredBy string) client.StartWorkflowOptions {
	return client.StartWorkflowOptions{
		ID:                    uuid.NewString(),
		TaskQueue:             queue,
		TypedSearchAttributes: TypedSearchAttributes(vxID, triggeredBy),
	}
}

// WithChildSearchAttributes returns ctx with the given VXID and the parent's
// TriggeredBy stamped on the child workflow options, leaving all other child
// options untouched.
func WithChildSearchAttributes(ctx workflow.Context, vxID string) workflow.Context {
	triggeredBy, _ := workflow.GetTypedSearchAttributes(ctx).GetKeyword(TriggeredByKey)
	opts := workflow.GetChildWorkflowOptions(ctx)
	opts.TypedSearchAttributes = TypedSearchAttributes(vxID, triggeredBy)
	return workflow.WithChildOptions(ctx, opts)
}

// UpsertVXID sets the VXID search attribute once the asset exists mid-flow.
// First VXID wins: VXID is a single Keyword, so multi-asset ingests keep the
// first created asset. Logs instead of failing the workflow.
func UpsertVXID(ctx workflow.Context, vxID string) {
	if vxID == "" {
		return
	}
	if existing, ok := workflow.GetTypedSearchAttributes(ctx).GetKeyword(VXIDKey); ok && existing != "" {
		return
	}
	err := workflow.UpsertTypedSearchAttributes(ctx, VXIDKey.ValueSet(vxID), LegacyVXIDKey.ValueSet(vxID))
	if err != nil {
		workflow.GetLogger(ctx).Error("failed to upsert VXID search attribute", "error", err)
	}
}
