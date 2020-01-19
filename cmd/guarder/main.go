package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/kkyr/fig"
	"github.com/mdlayher/apcupsd"

	"github.com/sirupsen/logrus"
)

// Config struct for configure the script
type Config struct {
	Server struct {
		Host string `fig:"host,default=127.0.0.1"`
		Port uint   `fig:"port,default=3551"`
	}
	Logger struct {
		Level string `fig:"level,default=info"`
		Path  string `fig:"path,default=/var/log/apcupsd_guarder.log"`
	}
	Trigger struct {
		OnFailed string `fig:"onfailed"`
		OnCheck  string `fig:"oncheck"`
	}
	Check struct {
		TimeLeft      time.Duration `fig:"timeleft,default=5m"`
		Interval      time.Duration `fig:"interval,default=1m"`
		MaxTriedTimes uint          `flg:"maxTriedTimes,default=5"`
	}
}

var (
	configure Config
	log       logrus.Logger
	timer     *time.Ticker
	ch        chan os.Signal
)

func init() {
	if err := fig.Load(&configure, fig.File("apcupsd_guarder.yaml"), fig.Dirs(".", "/etc/"), ); err != nil {
		panic(err)
	}

	log = logrus.Logger{
		Out:       os.Stdout,
		Formatter: &logrus.TextFormatter{DisableColors: true},
		Level:     logrus.DebugLevel,
	}

	if lf, err := os.OpenFile(configure.Logger.Path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0660); err != nil {
		log.Fatal(err)
	} else {
		log.SetOutput(lf)
	}
}

func main() {
	log.Infof("Start check timer for %v", configure.Check.Interval)
	timer = time.NewTicker(configure.Check.Interval)
	defer timer.Stop()

	go func() {
		for ; true; <-timer.C {
			Check()
		}
	}()

	// Wait for signals
	ch = make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGKILL)

	for s := range ch {
		abort := false

		switch s {
		case syscall.SIGHUP, syscall.SIGTERM, syscall.SIGKILL, os.Interrupt:
			abort = true
			break
		}

		if abort {
			break
		}
	}

	// fmt.Println("Gone")
}

var (
	triedTimes  uint = 0
	onlineCount uint = 0
)

// Check for checking UPS status and running scripts
func Check() {
	// Connect to UPS server
	client, err := apcupsd.Dial("tcp4", fmt.Sprintf("%s:%d", configure.Server.Host, configure.Server.Port))
	if err != nil {
		triedTimes++
		// if connect failed for first time, do not run ticker
		if onlineCount <= 0 {
			//timer.Stop()
			//ch <- os.Interrupt
			log.Fatal(err)
		}
	} else {
		log.Infof("Connect apcupsd %s:%d is success", configure.Server.Host, configure.Server.Port)
		defer client.Close()
	}

	// get status from UPS server
	status, err := client.Status()
	if err != nil {
		log.Error(err)
		triedTimes++
	}

	// parse status if not online mark tried times
	log.Debugf("UPS connected, report time left is %v", status.TimeLeft)
	if status.Status != "ONLINE" {
		log.Warning("UPS status is not online")
	} else {
		triedTimes = 0
		onlineCount++
		log.Infof("UPS is online, online count is %d", onlineCount)
	}

	// running check scripts every time
	runScript(configure.Trigger.OnCheck, status)

	// if ups has failed, RUN ON FAILED SCRIPT!
	if (status.TimeLeft < configure.Check.TimeLeft && status.Status != "ONLINE") || triedTimes > configure.Check.MaxTriedTimes {
		if triedTimes > 0 {
			log.Warnf("max tried times reached for %d", triedTimes)
		}

		if onlineCount > 0 {
			runScript(configure.Trigger.OnFailed, status)
		}
	}
}

// runScript for running external scripts and don't care the results
func runScript(path string, arg interface{}) {
	if _, err := os.Stat(path); err != os.ErrNotExist {
		log.Debugf("run scripts %v", path)
		result, _ := json.Marshal(arg)
		_ = exec.Command(path, string(result)).Start()
	}
}
