package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Luzifer/go_helpers/str"
	"github.com/Luzifer/rconfig"
	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	uuid "github.com/satori/go.uuid"
)

var (
	cfg = struct {
		ExecutionScript string `flag:"script" default:"/go/src/github.com/Luzifer/tex-api/tex-build.sh" description:"Script to execute (needs to generate output directory)"`
		Listen          string `flag:"listen" default:":3000" description:"IP/Port to listen on"`
		StorageDir      string `flag:"storage-dir" default:"/storage" description:"Where to store uploaded ZIPs and resulting files"`
		VersionAndExit  bool   `flag:"version" default:"false" description:"Prints current version and exits"`
	}{}

	version = "dev"
	router  = mux.NewRouter()
)

type status string

const (
	statusCreated  = "created"
	statusStarted  = "started"
	statusError    = "error"
	statusFinished = "finished"
)

const (
	filenameInput     = "input.zip"
	filenameStatus    = "status.json"
	filenameOutputDir = "output"
	sleepBase         = 1.5
)

type jobStatus struct {
	UUID      string    `json:"uuid"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Status    status    `json:"status"`
}

func loadStatusByUUID(uid uuid.UUID) (*jobStatus, error) {
	statusFile := pathFromUUID(uid, filenameStatus)

	status := jobStatus{}
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
	uid, _ := uuid.FromString(s.UUID)
	f, err := os.Create(pathFromUUID(uid, filenameStatus))
	if err != nil {
		return err
	}
	defer f.Close()

	return json.NewEncoder(f).Encode(s)
}

func urlMust(u *url.URL, err error) *url.URL {
	if err != nil {
		log.Fatalf("Unable to retrieve URL from router: %s", err)
	}
	return u
}

func init() {
	if err := rconfig.Parse(&cfg); err != nil {
		log.Fatalf("Unable to parse commandline options: %s", err)
	}

	if cfg.VersionAndExit {
		fmt.Printf("tex-api %s\n", version)
		os.Exit(0)
	}
}

func main() {
	router.HandleFunc("/", apiDocs).Methods("GET").Name("apiDocs")
	router.HandleFunc("/job", startNewJob).Methods("POST").Name("startNewJob")
	router.HandleFunc("/job/{uid:[0-9a-z-]{36}}", getJobStatus).Methods("GET").Name("getJobStatus")
	router.HandleFunc("/job/{uid:[0-9a-z-]{36}}/wait", waitForJob).Methods("GET").Name("waitForJob")
	router.HandleFunc("/job/{uid:[0-9a-z-]{36}}/download", downloadAssets).Methods("GET").Name("downloadAssets")

	log.Fatalf("%s", http.ListenAndServe(cfg.Listen, router))
}

func pathFromUUID(uid uuid.UUID, filename string) string {
	return path.Join(cfg.StorageDir, uid.String(), filename)
}

func apiDocs(res http.ResponseWriter, r *http.Request) {
	http.Error(res, "Not implemented yet", http.StatusInternalServerError)
}

func startNewJob(res http.ResponseWriter, r *http.Request) {
	jobUUID := uuid.NewV4()
	inputFile := pathFromUUID(jobUUID, filenameInput)
	statusFile := pathFromUUID(jobUUID, filenameStatus)

	if err := os.Mkdir(path.Dir(inputFile), 0755); err != nil {
		log.Errorf("Unable to create job dir %q: %s", path.Dir(inputFile), err)
	}

	if f, err := os.Create(inputFile); err == nil {
		defer f.Close()
		if _, copyErr := io.Copy(f, r.Body); err != nil {
			log.Errorf("Unable to copy input file %q: %s", inputFile, copyErr)
			http.Error(res, "An error ocurred. See details in log.", http.StatusInternalServerError)
			return
		}
		f.Sync()
	} else {
		log.Errorf("Unable to write input file %q: %s", inputFile, err)
		http.Error(res, "An error ocurred. See details in log.", http.StatusInternalServerError)
		return
	}

	status := jobStatus{
		UUID:      jobUUID.String(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Status:    statusCreated,
	}
	if err := status.Save(); err != nil {
		log.Errorf("Unable to create status file %q: %s", statusFile, err)
		http.Error(res, "An error ocurred. See details in log.", http.StatusInternalServerError)
		return
	}

	go jobProcessor(jobUUID)

	u := urlMust(router.Get("waitForJob").URL("uid", jobUUID.String()))
	http.Redirect(res, r, u.String(), http.StatusFound)
}

func getJobStatus(res http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	uid, err := uuid.FromString(vars["uid"])
	if err != nil {
		http.Error(res, "UUID had unexpected format!", http.StatusBadRequest)
		return
	}

	if status, err := loadStatusByUUID(uid); err == nil {
		if encErr := json.NewEncoder(res).Encode(status); err != nil {
			log.Errorf("Unable to serialize status file: %s", encErr)
			http.Error(res, "An error ocurred. See details in log.", http.StatusInternalServerError)
			return
		}
	} else {
		log.Errorf("Unable to read status file: %s", err)
		http.Error(res, "An error ocurred. See details in log.", http.StatusInternalServerError)
		return
	}
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
		log.Errorf("Unable to read status file: %s", err)
		http.Error(res, "An error ocurred. See details in log.", http.StatusInternalServerError)
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

func shouldPackFile(extension string) bool {
	return str.StringInSlice(extension, []string{
		".log",
		".pdf",
	})
}

func buildAssetsZIP(uid uuid.UUID) (io.Reader, error) {
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)

	basePath := pathFromUUID(uid, filenameOutputDir)
	err := filepath.Walk(basePath, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !shouldPackFile(path.Ext(info.Name())) {
			return nil
		}

		zipInfo, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		zipInfo.Name = strings.TrimLeft(strings.Replace(p, basePath, "", 1), "/\\")
		zipFile, err := w.CreateHeader(zipInfo)
		if err != nil {
			return err
		}
		osFile, err := os.Open(p)
		if err != nil {
			return err
		}

		io.Copy(zipFile, osFile)
		osFile.Close()

		return nil
	})

	if err != nil {
		return nil, err
	}

	return buf, w.Close()
}

func buildAssetsTAR(uid uuid.UUID) (io.Reader, error) {
	buf := new(bytes.Buffer)
	w := tar.NewWriter(buf)

	basePath := pathFromUUID(uid, filenameOutputDir)
	err := filepath.Walk(basePath, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !shouldPackFile(path.Ext(info.Name())) {
			return nil
		}

		tarInfo, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		tarInfo.Name = strings.TrimLeft(strings.Replace(p, basePath, "", 1), "/\\")
		err = w.WriteHeader(tarInfo)
		if err != nil {
			return err
		}
		osFile, err := os.Open(p)
		if err != nil {
			return err
		}

		io.Copy(w, osFile)
		osFile.Close()

		return nil
	})

	if err != nil {
		return nil, err
	}

	return buf, w.Close()
}

func downloadAssets(res http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	uid, err := uuid.FromString(vars["uid"])
	if err != nil {
		http.Error(res, "UUID had unexpected format!", http.StatusBadRequest)
		return
	}

	var (
		content  io.Reader
		filename string
	)

	switch r.Header.Get("Accept") {
	case "application/tar", "application/x-tar", "applicaton/x-gtar", "multipart/x-tar", "application/x-compress", "application/x-compressed":
		content, err = buildAssetsTAR(uid)
		filename = uid.String() + ".tar"
	default:
		content, err = buildAssetsZIP(uid)
		filename = uid.String() + ".zip"
	}

	if err != nil {
		log.Errorf("Unable to generate downloadable asset: %s", err)
		http.Error(res, "An error ocurred. See details in log.", http.StatusInternalServerError)
		return
	}

	res.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	res.Header().Set("Content-Type", "application/octet-stream") // TODO(luzifer): Use a correct type?
	res.WriteHeader(http.StatusOK)

	io.Copy(res, content)
}

func jobProcessor(uid uuid.UUID) {
	processingDir := path.Dir(pathFromUUID(uid, filenameStatus))
	status, err := loadStatusByUUID(uid)
	if err != nil {
		log.Errorf("Unable to load status file in processing job: %s", err)
		return
	}

	cmd := exec.Command("/bin/bash", cfg.ExecutionScript)
	cmd.Dir = processingDir
	cmd.Stderr = log.StandardLogger().WriterLevel(log.ErrorLevel)

	status.UpdateStatus(statusStarted)
	if err := status.Save(); err != nil {
		log.Errorf("Unable to save status file")
		return
	}

	if err := cmd.Run(); err != nil {
		status.UpdateStatus(statusError)
		if err := status.Save(); err != nil {
			log.Errorf("Unable to save status file")
			return
		}
		return
	}

	status.UpdateStatus(statusFinished)
	if err := status.Save(); err != nil {
		log.Errorf("Unable to save status file")
		return
	}
}
