package main

import (
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strconv"
	"time"

	"github.com/gofrs/uuid"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

func jobProcessor(uid uuid.UUID) {
	logger := logrus.WithField("uuid", uid)
	logger.Info("Started processing")

	processingDir := path.Dir(pathFromUUID(uid, filenameStatus))
	status, err := loadStatusByUUID(uid)
	if err != nil {
		logger.WithError(err).Error("Unable to load status file in processing job")
		return
	}

	cmd := exec.Command("/bin/bash", cfg.Script) // #nosec G204
	cmd.Dir = processingDir
	cmd.Stderr = logger.WriterLevel(logrus.InfoLevel) // Bash uses stderr for `-x` parameter

	status.UpdateStatus(statusStarted)
	if err := status.Save(); err != nil {
		logger.WithError(err).Error("Unable to save status file")
		return
	}

	if err := cmd.Run(); err != nil {
		logger.WithError(err).Error("Processing failed")
		status.UpdateStatus(statusError)
		if err := status.Save(); err != nil {
			logger.WithError(err).Error("Unable to save status file")
			return
		}
		return
	}

	status.UpdateStatus(statusFinished)
	if err := status.Save(); err != nil {
		logger.WithError(err).Error("Unable to save status file")
		return
	}
	logger.Info("Finished processing")
}

func startNewJob(res http.ResponseWriter, r *http.Request) {
	jobUUID := uuid.Must(uuid.NewV4())
	inputFile := pathFromUUID(jobUUID, filenameInput)
	statusFile := pathFromUUID(jobUUID, filenameStatus)

	if err := os.Mkdir(path.Dir(inputFile), 0750); err != nil {
		logrus.WithError(err).Errorf("Unable to create job dir %q", path.Dir(inputFile))
	}

	if f, err := os.Create(inputFile); err == nil {
		defer f.Close()
		if _, copyErr := io.Copy(f, r.Body); copyErr != nil {
			serverErrorf(res, copyErr, "Unable to copy input file %q", inputFile)
			return
		}
		f.Sync() // #nosec G104
	} else {
		serverErrorf(res, err, "Unable to write input file %q", inputFile)
		return
	}

	status := jobStatus{
		UUID:      jobUUID.String(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Status:    statusCreated,
	}
	if err := status.Save(); err != nil {
		serverErrorf(res, err, "Unable to create status file %q", statusFile)
		return
	}

	go jobProcessor(jobUUID)

	u := urlMust(router.Get("waitForJob").URL("uid", jobUUID.String()))
	http.Redirect(res, r, u.String(), http.StatusFound)
}

func waitForJob(res http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	uid, err := uuid.FromString(vars["uid"])
	if err != nil {
		http.Error(res, "UUID had unexpected format!", http.StatusBadRequest)
		return
	}

	var loop int
	if v := r.URL.Query().Get("loop"); v != "" {
		if pv, convErr := strconv.Atoi(v); convErr == nil {
			loop = pv
		}
	}
	loop++

	status, err := loadStatusByUUID(uid)
	if err != nil {
		serverErrorf(res, err, "Unable to read status file")
		return
	}

	switch status.Status {
	case statusCreated:
		fallthrough

	case statusStarted:
		u := urlMust(router.Get("waitForJob").URL("uid", uid.String()))
		u.Query().Set("loop", strconv.Itoa(loop))

		<-time.After(time.Duration(math.Pow(sleepBase, float64(loop))) * time.Second)

		http.Redirect(res, r, u.String(), http.StatusFound)
		return

	case statusError:
		http.Error(res, "Processing ran into an error.", http.StatusInternalServerError)

	case statusFinished:
		u := urlMust(router.Get("downloadAssets").URL("uid", uid.String()))
		http.Redirect(res, r, u.String(), http.StatusFound)
	}
}
