package cron

import (
	"encoding/json"

	"github.com/google/uuid"
)

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

func (j CronJob) MarshalJSON() ([]byte, error) {
	type cronJobAlias CronJob
	raw, err := marshalSchedule(j.Schedule)
	if err != nil {
		return nil, err
	}
	aux := struct {
		cronJobAlias
		Schedule json.RawMessage `json:"schedule"`
	}{
		cronJobAlias: cronJobAlias(j),
		Schedule:     raw,
	}
	return json.Marshal(aux)
}

func (j *CronJob) UnmarshalJSON(data []byte) error {
	type cronJobAlias CronJob
	aux := struct {
		*cronJobAlias
		Schedule json.RawMessage `json:"schedule"`
	}{
		cronJobAlias: (*cronJobAlias)(j),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	sch, err := unmarshalSchedule(aux.Schedule)
	if err != nil {
		return err
	}
	j.Schedule = sch
	return nil
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
