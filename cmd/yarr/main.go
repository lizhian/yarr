package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/nkanaev/yarr/src/platform"
	"github.com/nkanaev/yarr/src/server"
	"github.com/nkanaev/yarr/src/storage"
	"github.com/nkanaev/yarr/src/worker"
)

var Version string = "0.0"
var GitHash string = "unknown"

var OptList = make([]string, 0)

func opt(envVar, defaultValue string) string {
	OptList = append(OptList, envVar)
	value := os.Getenv(envVar)
	if value != "" {
		return value
	}
	return defaultValue
}

func defaultDBPathFromExecutable(executable string) (string, error) {
	executable, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return "", err
	}

	return filepath.Join(filepath.Dir(executable), "storage.db"), nil
}

func defaultDBPath() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}

	return defaultDBPathFromExecutable(executable)
}

func main() {
	platform.FixConsoleIfNeeded()

	var addr, db, certfile, keyfile, basepath, logfile string
	var ver, open bool

	flag.CommandLine.SetOutput(os.Stdout)

	flag.Usage = func() {
		out := flag.CommandLine.Output()
		fmt.Fprintf(out, "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Fprintln(out, "\nThe environmental variables, if present, will be used to provide\nthe default values for the params above:")
		fmt.Fprintln(out, " ", strings.Join(OptList, ", "))
	}

	flag.StringVar(&addr, "addr", opt("YARR_ADDR", "127.0.0.1:7070"), "address to run server on")
	flag.StringVar(&basepath, "base", opt("YARR_BASE", ""), "base path of the service url")
	flag.StringVar(&certfile, "cert-file", opt("YARR_CERTFILE", ""), "`path` to cert file for https")
	flag.StringVar(&keyfile, "key-file", opt("YARR_KEYFILE", ""), "`path` to key file for https")
	flag.StringVar(&db, "db", opt("YARR_DB", ""), "storage file `path`")
	flag.StringVar(&logfile, "log-file", opt("YARR_LOGFILE", ""), "`path` to log file to use instead of stdout")
	flag.BoolVar(&ver, "version", false, "print application version")
	flag.BoolVar(&open, "open", false, "open the server in browser")
	flag.Parse()

	if ver {
		fmt.Printf("v%s (%s)\n", Version, GitHash)
		return
	}

	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	if logfile != "" {
		file, err := os.OpenFile(logfile, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
		if err != nil {
			log.Fatal("Failed to setup log file: ", err)
		}
		defer file.Close()
		log.SetOutput(file)
	} else {
		log.SetOutput(os.Stdout)
	}

	if open && strings.HasPrefix(addr, "unix:") {
		log.Fatal("Cannot open ", addr, " in browser")
	}

	if db == "" {
		defaultDB, err := defaultDBPath()
		if err != nil {
			log.Fatal("Failed to get default db path: ", err)
		}
		db = defaultDB
	}

	log.Printf("using db file %s", db)

	if (certfile != "" || keyfile != "") && (certfile == "" || keyfile == "") {
		log.Fatalf("Both cert & key files are required")
	}

	store, err := storage.New(db)
	if err != nil {
		log.Fatal("Failed to initialise database: ", err)
	}

	worker.SetVersion(Version)
	srv := server.NewServer(store, addr)
	srv.SetBackupService(server.NewBackupService(store, db))

	if basepath != "" {
		srv.BasePath = "/" + strings.Trim(basepath, "/")
	}

	if certfile != "" && keyfile != "" {
		srv.CertFile = certfile
		srv.KeyFile = keyfile
	}

	log.Printf("starting server at %s", srv.GetAddr())
	if open {
		platform.Open(srv.GetAddr())
	}
	platform.Start(srv)
}
