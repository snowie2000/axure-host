package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

func deleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	detail := ProjectDB.GetProject(id)
	if detail == nil {
		http.Error(w, "project not found", http.StatusNotFound)
		return
	}
	ProjectDB.DelProject(id)
	os.RemoveAll(filepath.Join(projectFolder, detail.Path))
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	// only allow post
	if r.Method != http.MethodPost {
		http.Error(w, "", http.StatusMethodNotAllowed)
		return
	}

	projectName := r.FormValue("name")
	if projectName == "" {
		http.Error(w, "project name is required", http.StatusBadRequest)
		return
	}

	switch r.FormValue("action") {
	case "update":
		{
			projectId := r.FormValue("id")
			detail := ProjectDB.GetProject(projectId)
			if detail == nil {
				http.Error(w, "project doesn't exists", http.StatusBadRequest)
				return
			}

			r.ParseMultipartForm(10240 * 1024)
			projectPath := detail.Path

			file, header, err := r.FormFile("file")
			if err == nil { // for update request, file is optional
				defer file.Close()
				// only allow zip files
				if !strings.Contains(header.Header.Get("Content-Type"), "zip") {
					http.Error(w, "Only zip files are accepted", http.StatusBadRequest)
					return
				}
				projectPath = generateRandomString(8)
				// generate a random name for unnamed project
				for {
					// check if it already exists
					if !fileExists(filepath.Join(projectFolder, projectPath)) {
						break
					}
					projectPath = generateRandomString(8)
				}
				fullPath := filepath.Join(projectFolder, projectPath)
				if err := unzip(file, header.Size, fullPath); err != nil {
					os.RemoveAll(fullPath)
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				os.RemoveAll(filepath.Join(projectFolder, detail.Path))
			}

			// create a new project to overwrite the old one, if no new file is uploaded, the projectPath will be the same as detail.Path
			ProjectDB.NewProject(detail.Id, &ProjectInfo{
				Id:     detail.Id,
				Name:   projectName,
				Path:   projectPath,
				PinYin: r.FormValue("pinyin"),
				PY:     r.FormValue("py"),
				Desc:   r.FormValue("desc"),
			})
		}
	case "add":
		if ProjectDB.GetProject(projectName) != nil {
			http.Error(w, "project already exists", http.StatusBadRequest)
			return
		}

		r.ParseMultipartForm(10240 * 1024)
		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer file.Close()
		// only allow zip files
		if !strings.Contains(header.Header.Get("Content-Type"), "zip") {
			http.Error(w, "Only zip files are accepted", http.StatusBadRequest)
			return
		}

		projectPath := generateRandomString(8)
		// generate a random name for unnamed project
		for {
			// check if it already exists
			if !fileExists(filepath.Join(projectFolder, projectPath)) {
				break
			}
			projectPath = generateRandomString(8)
		}
		fullPath := filepath.Join(projectFolder, projectPath)
		if err := unzip(file, header.Size, fullPath); err != nil {
			os.RemoveAll(fullPath)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		ProjectDB.NewProject(projectPath, &ProjectInfo{
			Id:     projectPath,
			Name:   projectName,
			Path:   projectPath,
			PinYin: r.FormValue("pinyin"),
			PY:     r.FormValue("py"),
			Desc:   r.FormValue("desc"),
		})
	default:
		http.Error(w, "action can not be empty", http.StatusBadRequest)
	}
}

func projectListHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "", http.StatusMethodNotAllowed)
	}
	projects := ProjectDB.Projects()
	w.Header().Set("Content-Type", "application/json")
	jw := json.NewEncoder(w)
	jw.Encode(projects)
}

func renderHandler(w http.ResponseWriter, r *http.Request) {
	e := regexp.MustCompile("/project/([^/]+)")

	matches := e.FindStringSubmatch(r.URL.Path)
	info := ProjectDB.GetProject(matches[1])
	if info == nil {
		http.Error(w, "Project not found", http.StatusNotFound)
		return
	}
	fileName := r.URL.Path[len(matches[0]):]
	if fileName == "" {
		http.Redirect(w, r, r.URL.Path+"/", http.StatusFound)
		return
	}
	if fileName == "/" {
		fileName = "/index.html"
	}
	fullPath := filepath.Join(projectFolder, info.Path, fileName)
	http.ServeFile(w, r, fullPath)
}

func redirHandler(w http.ResponseWriter, r *http.Request) {
	projectName := strings.TrimPrefix(r.URL.Path, "/")
	if info := ProjectDB.GetProject(projectName); info == nil {
		assetsServer.ServeHTTP(w, r)
		return
	}
	http.Redirect(w, r, "/project/"+projectName+"/", http.StatusFound)
}

func generateRandomString(length int) string {
	rand.Seed(time.Now().UnixNano()) // Seed the random number generator

	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

func fileExists(f string) bool {
	if _, err := os.Stat(f); err == nil {
		return true
	}
	return false
}

func getFileName(f *zip.File) string {
	if f.Flags == 0 {
		i := bytes.NewReader([]byte(f.Name))
		decoder := transform.NewReader(i, simplifiedchinese.GB18030.NewDecoder())
		content, _ := ioutil.ReadAll(decoder)
		return string(content)
	} else {
		return f.Name
	}
}

func unzip(zipStream io.ReaderAt, zipSize int64, destDir string) error {
	r, err := zip.NewReader(zipStream, zipSize)
	if err != nil {
		return err
	}

	var rootFolder string
	for _, f := range r.File {
		if rootFolder == "" && strings.HasSuffix(f.Name, "/") {
			fName := getFileName(f)
			//check if the file is a directory
			if strings.Count(fName, "/") == 1 {
				//if there is only one slash, then it is a root folder.
				rootFolder = fName
				break
			}

		}
	}

	for _, f := range r.File {
		fName := getFileName(f)
		if fName == rootFolder {
			continue
		}
		fpath := filepath.Join(destDir, strings.TrimPrefix(fName, rootFolder))

		if !filepath.HasPrefix(fpath, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("%s: illegal file path", fpath)
		}

		if f.FileInfo().IsDir() {
			// Make Folder
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		// Make File
		if err = os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.FileInfo().Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}

		_, err = io.Copy(outFile, rc)

		// Close the file without defer to close before next iteration of loop
		outFile.Close()
		rc.Close()

		if err != nil {
			return err
		}
	}
	return nil
}
