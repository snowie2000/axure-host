package main

import (
	"encoding/json"
	"log"
	"os"
	"slices"
	"sync"
	"time"
)

type ProjectInfo struct {
	Id        string `json:"id"`
	Name      string `json:"name"`
	Path      string `json:"path"`
	Desc      string `json:"desc"`
	Date      string `json:"date"`
	Timestamp int64  `json:"timestamp"`
}

type projectDB map[string]*ProjectInfo

var (
	ProjectDB = make(projectDB)
	dbLock    sync.RWMutex
	dbFile    string
)

func LoadDB(f string) {
	dbFile = f
	if !fileExists(f) {
		return // new empty db
	}
	data, err := os.ReadFile(f)
	if err != nil {
		log.Fatal(err)
	}
	err = json.Unmarshal(data, &ProjectDB)
}

func SaveDB(f string) {
	data, err := json.MarshalIndent(ProjectDB, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	err = os.WriteFile(f, data, 0644)
}

func (db *projectDB) GetProject(id string) *ProjectInfo {
	dbLock.RLock()
	defer dbLock.RUnlock()

	if val, ok := (*db)[id]; ok {
		return val
	}
	return nil
}

func (db *projectDB) NewProject(id string, project *ProjectInfo) {
	dbLock.Lock()
	defer dbLock.Unlock()

	project.Date = time.Now().Format("2006-01-02 15:04:05")
	project.Timestamp = time.Now().Unix()
	(*db)[id] = project
	SaveDB(dbFile)
}

func (db *projectDB) DelProject(id string) {
	dbLock.Lock()
	defer dbLock.Unlock()

	delete(*db, id)
	SaveDB(dbFile)
}

func (db *projectDB) Projects() []*ProjectInfo {
	dbLock.RLock()
	defer dbLock.RUnlock()
	var projects []*ProjectInfo
	for _, prj := range *db {
		projects = append(projects, prj)
	}
	slices.SortFunc(projects, func(i, j *ProjectInfo) int {
		return int(j.Timestamp - i.Timestamp)
	})
	return projects
}
