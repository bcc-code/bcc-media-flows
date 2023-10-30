package ingest

import "encoding/xml"

type Metadata struct {
	XMLName       xml.Name      `xml:"Metadata"`
	JobProperty   JobProperty   `xml:"jobPropertyList"`
	FileList      FileList      `xml:"fileList"`
	JobHistoryLog JobHistoryLog `xml:"jobHistoryLog"`
}

type JobProperty struct {
	JobID            int    `xml:"jobID"`
	UserName         string `xml:"userName"`
	CompanyName      string `xml:"companyName"`
	SourceIP         string `xml:"sourceIP"`
	UserEmail        string `xml:"userEmail"`
	IngestStation    string `xml:"ingestStation"`
	UploadBitRate    string `xml:"uploadBitRate"`
	UploadTime       string `xml:"uploadTime"`
	FtpSiteID        string `xml:"ftpSiteId"`
	FileCount        int    `xml:"fileCount"`
	OrderForm        string `xml:"orderForm"`
	SubmissionDate   string `xml:"submissionDate"`
	LastDateChanged  string `xml:"lastDateChanged"`
	Status           string `xml:"status"`
	AssetType        string `xml:"asset_type"`
	SenderEmail      string `xml:"sender_email"`
	ProgramPost      string `xml:"program_post"`
	ProgramID        string `xml:"programid"`
	ReceivedFilename string `xml:"received_filename"`
	PersonsAppearing string `xml:"PersonsAppearing"`
	Tags             string `xml:"Tags"`
	PromoType        string `xml:"promo_type"`
	Language         string `xml:"language"`
}

type FileList struct {
	Files []File `xml:"file"`
}

type File struct {
	FileName string `xml:"FileName"`
	IsFolder bool   `xml:"isFolder"`
	FileSize int    `xml:"fileSize"`
	FilePath string `xml:"filePath"`
}

type JobHistoryLog struct {
	JobLogs []JobLog `xml:"jobLog"`
}

type JobLog struct {
	LogID             int    `xml:"logId"`
	JobLogDate        string `xml:"jobLogDate"`
	JobLogDescription string `xml:"jobLogDescription"`
	JobLogBy          string `xml:"jobLogBy"`
}
