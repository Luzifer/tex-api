package main

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/gofrs/uuid"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
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
		return nil, errors.Wrap(err, "opening status file")
	}
	defer func() {
		if err := f.Close(); err != nil {
			logrus.WithError(err).Error("closing status file (leaked fd)")
		}
	}()

	if err = json.NewDecoder(f).Decode(&status); err != nil {
		return nil, errors.Wrap(err, "decoding status file")
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
		return errors.Wrap(err, "creating status file")
	}
	defer func() {
		if err := f.Close(); err != nil {
			logrus.WithError(err).Error("closing status file (leaked fd)")
		}
	}()

	if err = json.NewEncoder(f).Encode(s); err != nil {
		return errors.Wrap(err, "encoding status")
	}

	return errors.Wrap(
		os.Rename(
			pathFromUUID(uid, filenameStatusTemp),
			pathFromUUID(uid, filenameStatus),
		),
		"moving status file in place",
	)
}
