package main

import (
	"fmt"
	"html/template"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gobuffalo/packr"
	"github.com/pkg/errors"
)

type handler struct {
	rootPath string
	indexTpl *template.Template
}

type DirInfo struct {
	Path   string
	FInfos []FInfo
}

type FInfo struct {
	Name  string
	MTime string
	Size  string
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

	box := packr.NewBox("box")
	tpl, err := template.New("index").Parse(box.String("index.gohtml"))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse template")
	}

	return &handler{a, tpl}, nil
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

		// directories first and sort by name
		sort.Slice(fileInfos, func(i, j int) bool {
			if fileInfos[i].IsDir() == fileInfos[j].IsDir() {
				return fileInfos[i].Name() < fileInfos[j].Name()
			}
			return fileInfos[i].IsDir()
		})

		h.writeHtml(w, r.URL.Path, fileInfos)
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

	mediaType, params, err := mime.ParseMediaType(r.Header.Get("content-type"))
	if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
		log.Printf("content-type parse error: %s, %v", r.Header.Get("content-type"), err)
		http.Error(w, fmt.Sprintf("%v", err), http.StatusInternalServerError)
		return
	}

	var part *multipart.Part
	mr := multipart.NewReader(r.Body, params["boundary"])
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("multipart parse err: %v", err)
			http.Error(w, fmt.Sprintf("%v", err), http.StatusInternalServerError)
			return
		}

		if p.FormName() == "upload" {
			part = p
			break
		}
	}

	if part == nil || part.FileName() == "" {
		log.Printf("part nil")
		http.Error(w, "empty form", http.StatusBadRequest)
		return
	}
	defer part.Close()

	newFilePath := filepath.Join(fPath, part.FileName())
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
	defer newFile.Close()

	_, err = io.Copy(newFile, part)
	if err != nil {
		log.Printf("%v", err)
		http.Error(w, fmt.Sprintf("%v", err), http.StatusInternalServerError)
		return
	}

	h.handleGet(w, r)
}

func (h *handler) writeHtml(w http.ResponseWriter, fPath string, fileInfos []os.FileInfo) {
	fInfos := make([]FInfo, 0, len(fileInfos))
	for _, i := range fileInfos {
		var nameStr string
		var mtimeStr string
		var sizeStr string

		m := i.Mode()
		if m.IsRegular() {
			nameStr = i.Name()
			mtimeStr = i.ModTime().Format("2006-01-02 15:04")
			sizeStr = fmt.Sprintf("%d", i.Size())
		} else if m.IsDir() {
			nameStr = i.Name() + "/"
		} else {
			continue
		}
		fInfos = append(fInfos, FInfo{nameStr, mtimeStr, sizeStr})
	}

	if err := h.indexTpl.Execute(w, DirInfo{fPath, fInfos}); err != nil {
		log.Printf("error: %v", err)
	}
}
