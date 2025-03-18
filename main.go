package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
)

const (
	VERSION = "1.0.0"
)

var (
	projectFolder string
	assetsServer  http.Handler
	appDir        string
)

func main() {
	fmt.Println("Self-hosted axure gallery", VERSION)
	host := flag.String("l", "0.0.0.0:80", "Address to bind")
	flag.StringVar(&projectFolder, "d", ".", "Destination folder")
	flag.Parse()

	l, err := net.Listen("tcp", *host)
	if err != nil {
		log.Fatal(err)
	}

	appDir, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	if projectFolder == "." {
		projectFolder = appDir
	}
	LoadDB(appDir + string(os.PathSeparator) + "db.json")
	routes()
	http.Serve(l, nil)
}

func routes() {
	http.HandleFunc("/api/upload", uploadHandler)
	http.HandleFunc("/api/list", projectListHandler)
	http.HandleFunc("/api/delete", deleteHandler)
	http.HandleFunc("/project/", renderHandler)
	http.HandleFunc("/", redirHandler)

	assetsServer = http.FileServer(http.Dir(filepath.Join(appDir, "web")))
}
