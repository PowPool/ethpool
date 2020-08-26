// +build go1.9

package main

import (
	"encoding/json"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"time"
	//"github.com/yvasiyarov/gorelic"

	"github.com/MiningPool0826/ethpool/api"
	"github.com/MiningPool0826/ethpool/payouts"
	"github.com/MiningPool0826/ethpool/proxy"
	"github.com/MiningPool0826/ethpool/storage"
	. "github.com/MiningPool0826/ethpool/util"
)

var cfg proxy.Config
var backend *storage.RedisClient

func startProxy() {
	s := proxy.NewProxy(&cfg, backend)
	s.Start()
}

func startApi() {
	s := api.NewApiServer(&cfg.Api, backend)
	s.Start()
}

func startBlockUnlocker() {
	u := payouts.NewBlockUnlocker(&cfg.BlockUnlocker, backend)
	u.Start()
}

func startPayoutsProcessor() {
	u := payouts.NewPayoutsProcessor(&cfg.Payouts, backend)
	u.Start()
}

// this function is for performance profile
//func startNewrelic() {
//	if cfg.NewrelicEnabled {
//		nr := gorelic.NewAgent()
//		nr.Verbose = cfg.NewrelicVerbose
//		nr.NewrelicLicense = cfg.NewrelicKey
//		nr.NewrelicName = cfg.NewrelicName
//		nr.Run()
//	}
//}

func readConfig(cfg *proxy.Config) {
	configFileName := "config.json"
	if len(os.Args) > 1 {
		configFileName = os.Args[1]
	}
	configFileName, _ = filepath.Abs(configFileName)
	Info.Printf("Loading config: %v", configFileName)

	configFile, err := os.Open(configFileName)
	if err != nil {
		Error.Fatal("File error: ", err.Error())
	}
	defer configFile.Close()
	jsonParser := json.NewDecoder(configFile)
	if err := jsonParser.Decode(&cfg); err != nil {
		Error.Fatal("Config error: ", err.Error())
	}
}

func main() {
	// init log file
	_ = os.Mkdir("logs", os.ModePerm)
	iLogFile := "logs/info.log"
	eLogFile := "logs/error.log"
	sLogFile := "logs/share.log"
	bLogFile := "logs/block.log"
	InitLog(iLogFile, eLogFile, sLogFile, bLogFile)

	readConfig(&cfg)
	rand.Seed(time.Now().UnixNano())

	if cfg.Threads > 0 {
		runtime.GOMAXPROCS(cfg.Threads)
		Info.Printf("Running with %v threads", cfg.Threads)
	}

	//startNewrelic()

	backend = storage.NewRedisClient(&cfg.Redis, cfg.Coin)
	pong, err := backend.Check()
	if err != nil {
		Error.Printf("Can't establish connection to backend: %v", err)
	} else {
		Error.Printf("Backend check reply: %v", pong)
	}

	if cfg.Proxy.Enabled {
		go startProxy()
	}
	if cfg.Api.Enabled {
		go startApi()
	}
	if cfg.BlockUnlocker.Enabled {
		go startBlockUnlocker()
	}
	//if cfg.Payouts.Enabled {
	//	go startPayoutsProcessor()
	//}
	quit := make(chan bool)
	<-quit
}
