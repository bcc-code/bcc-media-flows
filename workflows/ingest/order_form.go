package ingestworkflows

import (
	"github.com/orsinium-labs/enum"
)

// OrderForm identifies which kind of ingest a delivery is, and therefore which
// child workflow handles it. AssetJSON derives it from the form key of the
// uploaded JSON sidecar; see jsonFormSpecs.
type OrderForm enum.Member[string]

var (
	OrderFormRawMaterial  = OrderForm{Value: "Rawmaterial"}
	OrderFormVBMaster     = OrderForm{Value: "VB"}
	OrderFormVBMasterBulk = OrderForm{Value: "VB_BULK"}
	OrderFormSeriesMaster = OrderForm{Value: "Series_Masters"}
	OrderFormOtherMaster  = OrderForm{Value: "Other_Masters"}
	OrderFormLEDMaterial  = OrderForm{Value: "LED-Material"}
	OrderFormPodcast      = OrderForm{Value: "Podcast"}
	OrderFormMultitrackPB = OrderForm{Value: "MultitrackPB"}
	OrderFormUpload       = OrderForm{Value: "Upload"}
	OrderFormMusic        = OrderForm{Value: "Music"}
	OrderFormDistribution = OrderForm{Value: "Distribution"}
	OrderForms            = enum.New(
		OrderFormRawMaterial,
		OrderFormMusic,
		OrderFormUpload,
		OrderFormVBMaster,
		OrderFormVBMasterBulk,
		OrderFormSeriesMaster,
		OrderFormOtherMaster,
		OrderFormLEDMaterial,
		OrderFormPodcast,
		OrderFormMultitrackPB,
		OrderFormDistribution,
	)
)

// AssetResult is the (empty) result of an ingest entry-point workflow.
type AssetResult struct{}
