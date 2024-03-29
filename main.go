// +build go1.9

package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"syscall"
	"time"
	//"github.com/yvasiyarov/gorelic"

	"github.com/PowPool/ethpool/api"
	"github.com/PowPool/ethpool/payouts"
	"github.com/PowPool/ethpool/proxy"
	"github.com/PowPool/ethpool/storage"
	. "github.com/PowPool/ethpool/util"
	"golang.org/x/crypto/ssh/terminal"
)

var (
	LatestTag           = ""
	LatestTagCommitSHA1 = ""
	LatestCommitSHA1    = ""
	BuildTime           = ""
	ReleaseType         = ""
)

var cfg proxy.Config
var backend *storage.RedisClient = nil

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
	log.Printf("Loading config: %v", configFileName)

	configFile, err := os.Open(configFileName)
	if err != nil {
		log.Fatal("File error: ", err.Error())
	}
	defer configFile.Close()
	jsonParser := json.NewDecoder(configFile)
	if err := jsonParser.Decode(&cfg); err != nil {
		log.Fatal("Config error: ", err.Error())
	}
}

func readSecurityPass() ([]byte, error) {
	fmt.Printf("Enter Security Password: ")
	var fd int
	if terminal.IsTerminal(int(syscall.Stdin)) {
		fd = int(syscall.Stdin)
	} else {
		tty, err := os.Open("/dev/tty")
		if err != nil {
			return nil, errors.New("error allocating terminal")
		}
		defer tty.Close()
		fd = int(tty.Fd())
	}

	SecurityPass, err := terminal.ReadPassword(fd)
	if err != nil {
		return nil, err
	}
	return SecurityPass, nil
}

func decryptPoolConfigure(cfg *proxy.Config, passBytes []byte) error {
	b, err := Ae64Decode(cfg.UpstreamCoinBaseEncrypted, passBytes)
	if err != nil {
		return err
	}
	cfg.UpstreamCoinBase = strings.ToLower(string(b))

	// check address
	if !IsValidHexAddress(cfg.UpstreamCoinBase) {
		return errors.New("decryptPoolConfigure: IsValidHexAddress")
	}

	if cfg.Redis.Enabled {
		b, err = Ae64Decode(cfg.Redis.PasswordEncrypted, passBytes)
		if err != nil {
			return err
		}
		cfg.Redis.Password = string(b)
	}

	if cfg.RedisFailover.Enabled {
		b, err = Ae64Decode(cfg.RedisFailover.PasswordEncrypted, passBytes)
		if err != nil {
			return err
		}
		cfg.RedisFailover.Password = string(b)
	}

	return nil
}

func getDeviceIPs() (map[string]struct{}, error) {
	ipAddrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}

	DeviceIPs := make(map[string]struct{})
	for _, addr := range ipAddrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ip4 := ipNet.IP.To4(); ip4 != nil {
				DeviceIPs[ip4.String()] = struct{}{}
			}
		}
	}
	return DeviceIPs, nil
}

func initPeerName(cfg *proxy.Config) error {
	deviceIPs, err := getDeviceIPs()
	if err != nil {
		return err
	}

	for _, c := range cfg.Cluster {
		_, ok := deviceIPs[c.NodeIp]
		if ok {
			cfg.Name = c.NodeName
			cfg.LocalIP = c.NodeIp
			return nil
		}
	}
	return errors.New("local Node is not in the Pool cluster")
}

func OptionParse() {
	var showVer bool
	flag.BoolVar(&showVer, "v", false, "show build version")

	flag.Parse()

	if showVer {
		fmt.Printf("Latest Tag: %s\n", LatestTag)
		fmt.Printf("Latest Tag Commit SHA1: %s\n", LatestTagCommitSHA1)
		fmt.Printf("Latest Commit SHA1: %s\n", LatestCommitSHA1)
		fmt.Printf("Build Time: %s\n", BuildTime)
		fmt.Printf("Release Type: %s\n", ReleaseType)
		os.Exit(0)
	}
}

func main() {
	OptionParse()
	readConfig(&cfg)
	rand.Seed(time.Now().UnixNano())

	// init log file
	_ = os.Mkdir("logs", os.ModePerm)
	iLogFile := "logs/info.log"
	eLogFile := "logs/error.log"
	sLogFile := "logs/share.log"
	bLogFile := "logs/block.log"
	InitLog(iLogFile, eLogFile, sLogFile, bLogFile, cfg.Log.LogSetLevel)

	// set rlimit nofile value
	SetRLimit(800000)

	err := initPeerName(&cfg)
	if err != nil {
		Error.Fatal("initPeerName error: ", err.Error())
	}
	Info.Println("Init Peer Name as:", cfg.Name)

	if cfg.Threads > 0 {
		runtime.GOMAXPROCS(cfg.Threads)
		Info.Printf("Running with %v threads", cfg.Threads)
	}

	//startNewrelic()

	secPassBytes, err := readSecurityPass()
	if err != nil {
		Error.Fatal("Read Security Password error: ", err.Error())
	}

	err = decryptPoolConfigure(&cfg, secPassBytes)
	if err != nil {
		Error.Fatal("Decrypt Pool Configure error: ", err.Error())
	}

	if cfg.Redis.Enabled {
		backend = storage.NewRedisClient(&cfg.Redis, cfg.Coin)
	} else if cfg.RedisFailover.Enabled {
		backend = storage.NewRedisFailoverClient(&cfg.RedisFailover, cfg.Coin)
	}

	if backend == nil {
		Error.Fatal("Backend is Nil: maybe redis/redisFailover config is invalid")
	}

	pong, err := backend.Check()
	if err != nil {
		Error.Printf("Can't establish connection to backend: %v", err)
	} else {
		Info.Printf("Backend check reply: %v", pong)
	}

	defer func() {
		if r := recover(); r != nil {
			Error.Println(string(debug.Stack()))
		}
	}()

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
