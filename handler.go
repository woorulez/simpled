package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

type handler struct {
	rootPath string
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("request %s %s", r.Method, r.RequestURI)
	switch r.Method {
	case http.MethodGet:
		h.handleGet(w, r)
	case http.MethodPost:
		h.handlePost(w, r)
	default:
		http.Error(w, fmt.Sprintf("method %s not allowed", r.Method), http.StatusBadRequest)
	}
}

func (h *handler) toFPath(urlPath string) string {
	relPath := filepath.FromSlash(urlPath)
	return filepath.Join(h.rootPath, relPath)
}

func (h *handler) handleGet(w http.ResponseWriter, r *http.Request) {
	fPath := h.toFPath(r.URL.EscapedPath())
	log.Printf("fs path = %s", fPath)

	f, err := os.Open(fPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	info, _ := f.Stat()
	mode := info.Mode()

	if mode.IsRegular() {
		io.Copy(w, f)
		return
	}

	if mode.IsDir() {
		fileInfos, err := f.Readdir(-1)
		if err != nil {
			http.Error(w, fmt.Sprintf("%v", err), http.StatusInternalServerError)
			return
		}

		html := genHtml(fileInfos, func(i, j int) bool {
			if fileInfos[i].IsDir() == fileInfos[j].IsDir() {
				return fileInfos[i].Name() < fileInfos[j].Name()
			}
			return fileInfos[i].IsDir()
		})
		w.Write([]byte(html))
		return
	}

	http.Error(w, fmt.Sprintf("unknown file mode: %s", mode.String()), http.StatusInternalServerError)
}

func (h *handler) handlePost(w http.ResponseWriter, r *http.Request) {
	fPath := h.toFPath(r.URL.EscapedPath())

	f, err := os.Open(fPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	info, _ := f.Stat()
	if info.IsDir() == false {
		http.Error(w, fmt.Sprintf("not a directory: %s", fPath), http.StatusBadRequest)
		return
	}

	formFile, formFileHeader, err := r.FormFile("upload")
	if err != nil {
		log.Printf("%v", err)
		http.Error(w, fmt.Sprintf("%v", err), http.StatusInternalServerError)
		return
	}
	defer formFile.Close()

	newFilePath := filepath.Join(fPath, formFileHeader.Filename)
	if _, err = os.Stat(newFilePath); err == nil {
		http.Error(w, fmt.Sprintf("already exists: %s", newFilePath), http.StatusForbidden)
		return
	}

	newFile, err := os.Create(newFilePath)
	if err != nil {
		log.Printf("%v", err)
		http.Error(w, fmt.Sprintf("%v", err), http.StatusInternalServerError)
		return
	}

	_, err = io.Copy(newFile, formFile)
	if err != nil {
		log.Printf("%v", err)
		http.Error(w, fmt.Sprintf("%v", err), http.StatusInternalServerError)
		return
	}

	h.handleGet(w, r)
}

func Handler(rootPath string) (http.Handler, error) {
	s := strings.TrimSpace(rootPath)
	if len(s) == 0 {
		s = "."
	}

	r, err := filepath.EvalSymlinks(s)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to evaluate: %s", rootPath)
	}

	a, err := filepath.Abs(r)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to absolute: %s", rootPath)
	}

	fi, err := os.Stat(a)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to stat: %s", rootPath)
	}
	if fi.IsDir() == false {
		return nil, errors.Wrapf(err, "not a directory: %s", rootPath)
	}

	return &handler{a}, nil
}
