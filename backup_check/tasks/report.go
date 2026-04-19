package tasks

import (
	"time"

	"comic_downloader_go_playwright_stealth/siteflow"
)

// TaskState is the normalized task state string used by the UI.
type TaskState string

const (
	TaskStatePrepared  TaskState = "prepared"
	TaskStateRunning   TaskState = "running"
	TaskStateCompleted TaskState = "completed"
	TaskStateFailed    TaskState = "failed"
)

// TaskReport is the persisted task summary consumed by the UI.
type TaskReport struct {
	TaskID            string                       `json:"taskID"`
	Manifest          siteflow.TaskManifestSummary `json:"manifest"`
	State             TaskState                    `json:"state"`
	Verification      string                       `json:"verification"`
	OutputRoot        string                       `json:"outputRoot"`
	ThumbnailRoot     string                       `json:"thumbnailRoot"`
	StatePath         string                       `json:"statePath"`
	ReportPath        string                       `json:"reportPath"`
	LogPath           string                       `json:"logPath"`
	StorageState      string                       `json:"storageState"`
	VerificationState string                       `json:"verificationState"`
	InitScript        string                       `json:"initScript"`
	CreatedAt         time.Time                    `json:"createdAt"`
	StartedAt         time.Time                    `json:"startedAt"`
	FinishedAt        time.Time                    `json:"finishedAt"`
	LastError         string                       `json:"lastError"`
}
