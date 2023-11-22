package vscommon

import "github.com/orsinium-labs/enum"

type FieldType enum.Member[string]

var (
	FieldDurationSeconds       = FieldType{"durationSeconds"}
	FieldDescription           = FieldType{"portal_mf982016"}
	FieldExportAudioSource     = FieldType{"portal_mf452504"}
	FieldLangsToExport         = FieldType{"portal_mf326592"}
	FieldLanguagesRecorded     = FieldType{"portal_mf189850"}
	FieldPersonsAppearing      = FieldType{"portal_mf50574"}
	FieldSequenceSize          = FieldType{"__sequence_size"}
	FieldStartTC               = FieldType{"startTimeCode"}
	FieldStlText               = FieldType{"stl_text"}
	FieldSubclipToExport       = FieldType{"portal_mf230973"}
	FieldSubclipType           = FieldType{"portal_mf594493"}
	FieldTitle                 = FieldType{"title"}
	FieldSource                = FieldType{"portal_mf103965"}
	FieldExportAsChapter       = FieldType{"portal_mf457300"}
	FieldSubtransStoryID       = FieldType{"portal_mf397928"}
	FieldOriginalURI           = FieldType{"originalUri"}
	FieldOriginalFileName      = FieldType{"portal_mf212729"}
	FieldOriginalFileNameField = FieldType{"portal_mf49070"}
	FieldUploadedBy            = FieldType{"portal_mf381829"}
	FieldUploadJob             = FieldType{"portal_mf846642"}
	FieldGeneralTags           = FieldType{"portal_mf957223"}
	FieldEpisodeDescription    = FieldType{"portal_mf720613"}
	FieldSeason                = FieldType{"portal_mf765401"}
	FieldProgram               = FieldType{"portal_mf426791"}
	FieldEpisode               = FieldType{"portal_mf978874"}
	FieldLoudnessLUFS          = FieldType{"portal_mf127446"}
	FieldLoudnessRange         = FieldType{"portal_mf76564"}
	FieldTruePeak              = FieldType{"portal_mf687468"}
	FieldDialogLoudness        = FieldType{"portal_mf388644"}
	FieldDialogPercentage      = FieldType{"portal_mf350671"}
	FieldTypes                 = enum.New(FieldDurationSeconds, FieldDescription, FieldExportAudioSource, FieldLangsToExport,
		FieldPersonsAppearing, FieldSequenceSize, FieldStartTC, FieldSubclipToExport, FieldSubclipType, FieldTitle,
		FieldSource, FieldExportAsChapter, FieldSubtransStoryID, FieldOriginalURI, FieldUploadedBy, FieldUploadJob,
		FieldLanguagesRecorded, FieldGeneralTags, FieldOriginalFileName, FieldOriginalFileNameField,
		FieldEpisodeDescription, FieldSeason, FieldProgram, FieldEpisode, FieldStlText)
)
