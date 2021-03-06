// import github.com/dmcsorley/goblin
package main

import (
	"flag"
	"fmt"
	"github.com/dmcsorley/goblin/cibuild"
	"github.com/dmcsorley/goblin/gobdocker"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"time"
)

const (
	CONFIG_FILE = "goblin.hcl"
	LISTEN_ADDR = ":80"
	ENV_IMAGE   = "IMAGE"
	DEBUG_FLAG  = "debug"
	RUN_FLAG    = "run"
	TIME_FLAG   = "time"
)

func dumpRequest(r *http.Request) string {
	bytes, err := httputil.DumpRequest(r, true)
	if err != nil {
		fmt.Println(err)
		return ""
	}

	return string(bytes)
}

var debugFlag bool
var runFlag string
var timeFlag string

func init() {
	flag.BoolVar(&debugFlag, DEBUG_FLAG, false, "turn on debug mode")
	flag.StringVar(&runFlag, RUN_FLAG, "", "build to run from config file")
	flag.StringVar(&timeFlag, TIME_FLAG, "", "timestamp of build to run")
}

func main() {
	flag.Parse()

	cfg, err := loadConfig(CONFIG_FILE)
	if err != nil {
		log.Fatal("Error loading server config: " + err.Error())
	}

	if runFlag != "" && timeFlag != "" {
		runBuild(cfg, runFlag, timeFlag)
	} else {
		serve(cfg)
	}
}

func cleanupBuild(eb *gobdocker.ExitedBuild) {
	fmt.Println(
		eb.Id,
		"EXITED",
		eb.Exit,
	)

	cibuild.Cleanup(eb)
}

func serve(cfg *Goblin) {
	image := os.Getenv(ENV_IMAGE)
	if image == "" {
		log.Fatal(ENV_IMAGE + " environment variable is required")
	}

	hostname, _ := os.Hostname()
	fmt.Printf("Listening on %s%s\n", hostname, LISTEN_ADDR)

	r := mux.NewRouter()
	posts := r.Methods("POST").Subrouter()

	for _, bc := range cfg.builds {
		localConfig := bc
		fmt.Printf("Build configured on /%s\n", localConfig.Name)
		posts.HandleFunc("/"+localConfig.Name, func(w http.ResponseWriter, r *http.Request) {
			now := time.Now()
			if debugFlag {
				fmt.Println("DEBUG", dumpRequest(r))
			}
			build := cibuild.New(now, localConfig, cfg.values)
			fmt.Println(build.Id, "Received build for", r.URL.Path)
			w.WriteHeader(http.StatusOK)
			go build.DockerRun(image)
		})
	}

	go gobdocker.ListenForBuildExits(cleanupBuild)
	log.Fatal("Error starting http server: " + http.ListenAndServe(LISTEN_ADDR, r).Error())
}

func runBuild(cfg *Goblin, buildName string, timeStamp string) {
	bc := cfg.builds[buildName]
	if bc == nil {
		log.Fatal("No build found with name " + buildName)
	}

	t, err := time.Parse(cibuild.TimeFormat, timeStamp)
	if err != nil {
		log.Fatal(err)
	}

	build := cibuild.New(t, bc, cfg.values)
	build.Run()
}
