package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gofrs/uuid"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

const (
	statusCreated  = "created"
	statusStarted  = "started"
	statusError    = "error"
	statusFinished = "finished"
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

//revive:disable-next-line:get-return // Is not a getter
func getJobStatus(res http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	uid, err := uuid.FromString(vars["uid"])
	if err != nil {
		http.Error(res, "UUID had unexpected format!", http.StatusBadRequest)
		return
	}

	status, err := loadStatusByUUID(uid)
	if err != nil {
		serverErrorf(res, err, "reading status file")
		return
	}

	if encErr := json.NewEncoder(res).Encode(status); encErr != nil {
		serverErrorf(res, encErr, "serializing status file")
		return
	}
}

func loadStatusByUUID(uid uuid.UUID) (*jobStatus, error) {
	statusFile := pathFromUUID(uid, filenameStatus)

	status := jobStatus{}
	// #nosec G304
	f, err := os.Open(statusFile)
	if err != nil {
		return nil, fmt.Errorf("opening status file: %w", err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			logrus.WithError(err).Error("closing status file (leaked fd)")
		}
	}()

	if err = json.NewDecoder(f).Decode(&status); err != nil {
		return nil, fmt.Errorf("decoding status file: %w", err)
	}

	return &status, nil
}

func (s jobStatus) Save() error {
	uid, _ := uuid.FromString(s.UUID) // #nosec G104
	f, err := os.Create(pathFromUUID(uid, filenameStatusTemp))
	if err != nil {
		return fmt.Errorf("creating status file: %w", err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			logrus.WithError(err).Error("closing status file (leaked fd)")
		}
	}()

	if err = json.NewEncoder(f).Encode(s); err != nil {
		return fmt.Errorf("encoding status: %w", err)
	}

	if err = os.Rename(
		pathFromUUID(uid, filenameStatusTemp),
		pathFromUUID(uid, filenameStatus),
	); err != nil {
		return fmt.Errorf("moving status file in place: %w", err)
	}

	return nil
}

func (s *jobStatus) UpdateStatus(st status) {
	s.Status = st
	s.UpdatedAt = time.Now()
}
