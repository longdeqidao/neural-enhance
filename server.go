package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"time"
)

func log(format string, a ...interface{}) (n int, err error) {
	return fmt.Printf(format+"\n", a...)
}

const (
	TMP_DIR   = "./tmp"
	ZOOM_SIZE = 1
	EXEC_TIME = 1 * time.Minute
)

var fileCount uint64 = 0

func inFilename() string {
	count := atomic.AddUint64(&fileCount, 1)
	return filepath.Join(TMP_DIR, fmt.Sprintf("%d_%s", count, time.Now().Format("15:04:05")))
}

func outFilename(in string) string {
	return fmt.Sprintf("%s_ne%dx.png", in, ZOOM_SIZE)
}

func sendError(w http.ResponseWriter, code int, msg string) {
	w.WriteHeader(code)
	w.Write([]byte(msg))
}

func execEnhance(data []byte) ([]byte, error) {
	in := inFilename()
	err := ioutil.WriteFile(in, data, 0600)
	if err != nil {
		log("[write file %s failed: %v]", in, err)
		return nil, err
	}

	done := make(chan error)
	cmd := exec.Command("/opt/conda/bin/python3", "./enhance.py", "--type=photo", "--model=repair",
		fmt.Sprintf("--zoom=%d", ZOOM_SIZE), in)
	// TODO: read out of stderr
	// cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr

	go func() {
		done <- cmd.Run()
	}()

	// TODO: clean tmp files
	select {
	case err := <-done:
		if err != nil {
			log("[exec enhance command failed: %v]", err)
			return nil, err
		}
		out := outFilename(in)
		enhancedData, err := ioutil.ReadFile(out)
		if err != nil {
			log("[read file %s failed: %v]", out, err)
			return nil, err
		}
		return enhancedData, nil
	case <-time.After(EXEC_TIME):
		return nil, fmt.Errorf("timeout")
	}
}

func enhancePicturehandler(w http.ResponseWriter, req *http.Request) {
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		sendError(w, http.StatusBadRequest, "bad body")
		return
	}

	enhancedBody, err := execEnhance(body)
	if err != nil {
		// TODO: refine fine-grained status code
		sendError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Write(enhancedBody)
}

func main() {
	err := os.MkdirAll(TMP_DIR, 0700)
	if err != nil {
		log("mkdir %s failed: %v", TMP_DIR, err)
	}
	http.HandleFunc("/enhance-picture", enhancePicturehandler)
	http.ListenAndServe("0.0.0.0:5000", nil)
}
