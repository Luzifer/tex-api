package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
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
	"github.com/spf13/afero"
	"github.com/spf13/afero/zipfs"
)

const (
	createModeDir  = 0o700
	createModeFile = 0o600
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

	// Create the target directory
	if err := os.Mkdir(path.Dir(inputFile), createModeDir); err != nil {
		logrus.WithError(err).Errorf("Unable to create job dir %q", path.Dir(inputFile))
	}

	// Create a copy-on-write fs for the target
	targetFs := afero.NewBasePathFs(afero.NewOsFs(), path.Dir(inputFile))

	// Cache the input body
	body := new(bytes.Buffer)
	if _, err := io.Copy(body, r.Body); err != nil {
		serverErrorf(res, err, "reading input body")
		return
	}

	// Pull in the new files into the target dir
	var inputSource afero.Fs
	if hasZipHeader(body.Bytes()) {
		// We got an archive
		zipR, err := zip.NewReader(bytes.NewReader(body.Bytes()), int64(body.Len()))
		if err != nil {
			serverErrorf(res, err, "opening input body as zip")
			return
		}
		inputSource = zipfs.New(zipR)
	} else {
		// We assume that was a single TeX file
		inputSource = afero.NewMemMapFs()
		if err := afero.WriteFile(inputSource, "main.tex", body.Bytes(), createModeFile); err != nil {
			serverErrorf(res, err, "writing input to in-mem fs")
			return
		}
	}

	if err := syncFilesToOverlay(inputSource, targetFs); err != nil {
		serverErrorf(res, err, "copying files to target dir")
		return
	}

	// If set copy the default env into the target dir
	if cfg.DefaultEnv != "" {
		if err := syncFilesToOverlay(
			afero.NewReadOnlyFs(afero.NewBasePathFs(afero.NewOsFs(), cfg.DefaultEnv)),
			targetFs,
		); err != nil {
			serverErrorf(res, err, "copying default env files to target dir")
			return
		}
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
	u.RawQuery = r.URL.Query().Encode()

	if r.URL.Query().Has("report-urls") {
		if err := json.NewEncoder(res).Encode(map[string]string{
			"download": urlMust(router.Get("downloadAssets").URL("uid", jobUUID.String())).String(),
			"status":   urlMust(router.Get("getJobStatus").URL("uid", jobUUID.String())).String(),
			"wait":     urlMust(router.Get("waitForJob").URL("uid", jobUUID.String())).String(),
		}); err != nil {
			serverErrorf(res, err, "encoding url response")
		}
		return
	}

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
		<-time.After(time.Duration(math.Pow(sleepBase, float64(loop))) * time.Second)

		params := r.URL.Query()
		params.Set("loop", strconv.Itoa(loop))

		u := urlMust(router.Get("waitForJob").URL("uid", uid.String()))
		u.RawQuery = params.Encode()

		http.Redirect(res, r, u.String(), http.StatusFound)
		return

	case statusError:
		if r.URL.Query().Has("log-on-error") {
			u := urlMust(router.Get("downloadAssets").URL("uid", uid.String()))
			u.RawQuery = r.URL.Query().Encode()
			http.Redirect(res, r, u.String(), http.StatusFound)
			return
		}

		http.Error(res, "Processing ran into an error.", http.StatusInternalServerError)

	case statusFinished:
		u := urlMust(router.Get("downloadAssets").URL("uid", uid.String()))
		u.RawQuery = r.URL.Query().Encode()
		http.Redirect(res, r, u.String(), http.StatusFound)
	}
}
