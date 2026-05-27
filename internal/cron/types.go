package cron

import (
	"github.com/google/uuid"
)

type Schedule struct {
	Kind    string `json:"kind"`    // "cron" | "every" | "at"
	Expr    string `json:"expr"`    // cron expression
	EveryMs int64  `json:"everyMs"` // interval in milliseconds
	AtMs    int64  `json:"atMs"`    // one-shot timestamp ms
}

type Payload struct {
	Message string `json:"message"`
	Deliver bool   `json:"deliver"`
	Channel string `json:"channel"`
	To      string `json:"to"`
}

type JobState struct {
	NextRunAtMs int64  `json:"nextRunAtMs"`
	LastRunAtMs int64  `json:"lastRunAtMs"`
	LastStatus  string `json:"lastStatus"` // "ok" | "error"
	LastError   string `json:"lastError"`
}

type CronJob struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Enabled        bool     `json:"enabled"`
	Schedule       Schedule `json:"schedule"`
	Payload        Payload  `json:"payload"`
	State          JobState `json:"state"`
	DeleteAfterRun bool     `json:"deleteAfterRun"`
}

func NewCronJob(name string, schedule Schedule, payload Payload) CronJob {
	return CronJob{
		ID:       uuid.NewString(),
		Name:     name,
		Enabled:  true,
		Schedule: schedule,
		Payload:  payload,
	}
}
