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

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/sirupsen/logrus"
)

// Config struct for configure the script
type Config struct {
	Server struct {
		Host string `fig:"host,default=127.0.0.1"`
		Port uint   `fig:"port,default=3551"`
	}
	Logger struct {
		Level  string        `fig:"level,default=info"`
		MaxAge time.Duration `flg:"maxAge,default=168h"`
		Path   string        `fig:"path,default=/var/log/apcupsd_guarder.log"`
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
	if err := fig.Load(&configure, fig.File("apcupsd_guarder.yaml"), fig.Dirs(".", "/etc/")); err != nil {
		panic(err)
	}

	log = logrus.Logger{
		Out:       os.Stdout,
		Formatter: &logrus.TextFormatter{DisableColors: true},
		Level:     logrus.DebugLevel,
	}

	// @see https://github.com/lestrrat-go/file-rotatelogs
	logf, err := rotatelogs.New(
		configure.Logger.Path+"_%Y%m%d",
		rotatelogs.WithLinkName(configure.Logger.Path),
		rotatelogs.WithMaxAge(configure.Logger.MaxAge),
	)

	if err != nil {
		log.Fatal("failed to create rotatelogs: %s", err)
	} else {
		log.SetOutput(logf)
	}

	log.Debugf("logger is initial with file path %s (max age is %v)", configure.Logger.Path, configure.Logger.MaxAge)
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
}

var (
	triedTimes  uint = 0
	onlineCount uint = 0
	lastStatus  *apcupsd.Status
)

// Check for checking UPS status and running scripts
func Check() {
	// Connect to UPS server
	if client, err := apcupsd.Dial("tcp4", fmt.Sprintf("%s:%d", configure.Server.Host, configure.Server.Port)); err != nil {
		triedTimes++
		// if connect failed for first time, do not run ticker
		if onlineCount <= 0 {
			// timer.Stop()
			// ch <- os.Interrupt
			log.Fatal(err)
		}
	} else {
		log.Infof("Connect apcupsd %s:%d is success", configure.Server.Host, configure.Server.Port)
		defer client.Close()

		// get status from UPS server
		if lastStatus, err = client.Status(); err != nil {
			log.Error(err)
			triedTimes++
		} else {
			// parse status if not online mark tried times
			log.Debugf("UPS connected, report time left is %v", lastStatus.TimeLeft)
			if lastStatus.Status == "ONLINE" {
				onlineCount++
				log.Infof("UPS is online, online count is %d", onlineCount)
				triedTimes = 0
			} else {
				if lastStatus.TimeLeft <= 0 && onlineCount <= 0 {
					log.Fatal("it seems apcupsd is not initialized")
				}

				log.Warning("UPS status is not online")
				triedTimes++
			}
		}
	}

	// if ups has failed, RUN ON FAILED SCRIPT!
	if triedTimes >= configure.Check.MaxTriedTimes ||
		(lastStatus != nil && lastStatus.Status != "ONLINE" && lastStatus.TimeLeft <= configure.Check.TimeLeft) {
		if triedTimes > 0 {
			log.Warnf("max tried times reached for %d", triedTimes)
		}

		log.Warnf("running failed callback script, tried times is %d, online count is %d", triedTimes, onlineCount)
		runScript(configure.Trigger.OnFailed, lastStatus)
	} else {
		// running check scripts every time
		log.Infof("running check callback script, tried times is %d", triedTimes)
		runScript(configure.Trigger.OnCheck, lastStatus)
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
