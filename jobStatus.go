package main

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/gofrs/uuid"
	"github.com/gorilla/mux"
)

type (
	jobStatus struct {
		UUID      string    `json:"uuid"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Status    status    `json:"status"`
	}

	status string
)

const (
	statusCreated  = "created"
	statusStarted  = "started"
	statusError    = "error"
	statusFinished = "finished"
)

func getJobStatus(res http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	uid, err := uuid.FromString(vars["uid"])
	if err != nil {
		http.Error(res, "UUID had unexpected format!", http.StatusBadRequest)
		return
	}

	if status, err := loadStatusByUUID(uid); err == nil {
		if encErr := json.NewEncoder(res).Encode(status); encErr != nil {
			serverErrorf(res, encErr, "Unable to serialize status file")
			return
		}
	} else {
		serverErrorf(res, err, "Unable to read status file")
		return
	}
}

func loadStatusByUUID(uid uuid.UUID) (*jobStatus, error) {
	statusFile := pathFromUUID(uid, filenameStatus)

	status := jobStatus{}
	// #nosec G304
	if f, err := os.Open(statusFile); err == nil {
		defer f.Close()
		if err = json.NewDecoder(f).Decode(&status); err != nil {
			return nil, err
		}
	} else {
		return nil, err
	}

	return &status, nil
}

func (s *jobStatus) UpdateStatus(st status) {
	s.Status = st
	s.UpdatedAt = time.Now()
}

func (s jobStatus) Save() error {
	uid, _ := uuid.FromString(s.UUID) // #nosec G104
	f, err := os.Create(pathFromUUID(uid, filenameStatusTemp))
	if err != nil {
		return err
	}
	defer f.Close()

	if err = json.NewEncoder(f).Encode(s); err != nil {
		return err
	}

	return os.Rename(
		pathFromUUID(uid, filenameStatusTemp),
		pathFromUUID(uid, filenameStatus),
	)
}
