package app

import (
	"context"
	"flag"
	"fmt"
	"github.com/BRUHItsABunny/bunnlog"
	gokhttp_client "github.com/BRUHItsABunny/gOkHttp/client"
	"github.com/BRUHItsABunny/gOkHttp/download"
	go_phub "github.com/BRUHItsABunny/go-phub"
	"github.com/BRUHItsABunny/go-phub/phproto"
	"github.com/BRUHItsABunny/stringvarformatter"
	"github.com/RoyalFlyBy/PornHubDownloader/utils"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

var FMTVars = []string{
	":VIDEO_NAME", // title of the video
	":VIDEO_ID",
	":VIDEO_VIEW_KEY",
	":UPLOADER_NAME", // name of the uploader (user)
	":CHANNEL_NAME",  // name of the uploader (channel)
	// DATE will be yyyy-mm-dd
	":DATE_UPLOADED",
	":DATE_DOWNLOADED",
	// TIME hh.mm.ss
	":TIME_UPLOADED",
	":TIME_DOWNLOADED",
	// TS is a timestamp with maximum accuracy
	":TS_UPLOADED",
	":TS_DOWNLOADED",
}

type App struct {
	Cfg            *Config
	BLog           *bunnlog.BunnyLog
	Client         *utils.PHUtil
	DownloadClient *http.Client
	SessionFile    *os.File
	Stats          *download.GlobalDownloadController
	NameFormatter  *stringvarformatter.Formatter
}

func NewApp() (*App, error) {
	result := &App{}

	result.ParseCfg()
	err := result.SetupLogger()
	if err != nil {
		return nil, err
	}

	err = result.SetupHTTPClient()
	if err != nil {
		return nil, err
	}

	err = result.SetupClient()
	if err != nil {
		return nil, err
	}

	result.Stats = download.NewGlobalDownloadController(time.Duration(5) * time.Second)
	return result, nil
}

func (a *App) ParseCfg() {
	if a.Cfg == nil {
		a.Cfg = &Config{}
	}

	flag.StringVar(&a.Cfg.Video, "video", "", "A single video (can be an URL or ID)")
	flag.StringVar(&a.Cfg.Videos, "videos", "", "Path to list of videos")
	flag.StringVar(&a.Cfg.Username, "username", "", "Username")
	flag.StringVar(&a.Cfg.Password, "password", "", "Password")
	flag.StringVar(&a.Cfg.NameFMT, "namefmt", FMTVars[0], "This can be used to format the final file name using the variables: "+strings.Join(FMTVars, ", "))
	flag.IntVar(&a.Cfg.DownloadThreads, "threads", 1, "This is how many files we download in parallel (min=1, max=6)")
	flag.BoolVar(&a.Cfg.Debug, "debug", false, "This argument is for how verbose the logger will be")
	flag.BoolVar(&a.Cfg.Daemon, "daemon", false, "This argument is for how the UI feedback will be, if set to true it will print JSON")
	flag.BoolVar(&a.Cfg.Version, "version", false, "This argument will print the current version data and exit")
	flag.Parse()

	if a.Cfg.DownloadThreads > 6 {
		a.Cfg.DownloadThreads = 6
	}
	if a.Cfg.DownloadThreads < 1 {
		a.Cfg.DownloadThreads = 1
	}

	a.NameFormatter = stringvarformatter.NewFormatter(a.Cfg.NameFMT+".mp4", FMTVars...)
}

func (a *App) SetupLogger() error {
	logFile, err := os.Create("PornHubDownloader.log")
	if err != nil {
		return err
	}
	var bLog bunnlog.BunnyLog
	if a.Cfg.Debug {
		bLog = bunnlog.GetBunnLog(true, bunnlog.VerbosityDEBUG, log.Ldate|log.Ltime)
	} else {
		bLog = bunnlog.GetBunnLog(false, bunnlog.VerbosityWARNING, log.Ldate|log.Ltime)
	}
	bLog.SetOutputFile(logFile)
	a.BLog = &bLog
	return nil
}

func (a *App) SetupHTTPClient() error {
	var err error
	a.DownloadClient, err = gokhttp_client.NewHTTPClient()
	if err != nil {
		return fmt.Errorf("client.NewHTTPClient: %w", err)
	}
	return nil
}

func (a *App) SetupClient() error {

	if len(a.Cfg.Username) == 0 {
		a.Cfg.Username = os.Getenv("PHDL_USERNAME")
		if len(a.Cfg.Username) > 1 {
			a.BLog.Infof("Using username from ENV variables: %s", utils.Censor(a.Cfg.Username, "*", 4, true))
		}
	}

	if len(a.Cfg.Password) == 0 {
		a.Cfg.Password = os.Getenv("PHDL_PASSWORD")
		if len(a.Cfg.Password) > 1 {
			a.BLog.Infof("Using password from ENV variables: %s", utils.Censor(a.Cfg.Password, "*", 4, true))
		}
	}

	f, err := os.OpenFile(".session", os.O_CREATE, 0600)
	if err != nil {
		return fmt.Errorf("os.OpenFile: %w", err)
	}
	fStat, err := f.Stat()
	if err != nil {
		return fmt.Errorf("f.Stat: %w", err)
	}
	a.SessionFile = f
	var session *phproto.PHDevice
	if fStat.Size() > 0 {
		session = go_phub.NewPHDevice()
		err = session.LoadAsSession(f, "")
		if err != nil {
			return fmt.Errorf("session.LoadAsSession: %w", err)
		}
	}
	a.Client = &utils.PHUtil{Client: go_phub.NewPHClient(a.DownloadClient, session), BLog: a.BLog}
	if len(a.Cfg.Username) > 1 && len(a.Cfg.Password) > 1 || a.Client.Client.Device.ValidRefreshToken() == nil {
		err = a.Client.Client.Authenticate(context.Background(), a.Cfg.Username, a.Cfg.Password)
		if err != nil {
			return fmt.Errorf("a.Client.Client.Authenticate: %w", err)
		}
	}

	err = a.Client.Client.Device.SaveAsSession(f, "")
	if err != nil {
		return fmt.Errorf("a.Client.Client.Device.SaveAsSession: %w", err)
	}
	return nil
}

func (a *App) VersionRoutine() string {
	result := strings.Builder{}
	currentPrompt := CurrentCodeBase.PromptCurrentVersion(CurrentVersion)
	result.WriteString(currentPrompt.Output)
	latestVersion, err := CurrentCodeBase.GetLatestVersion(context.Background(), nil)
	if err != nil {
		if strings.Contains(err.Error(), "repository has no tags") {
			return result.String()
		}
		panic(fmt.Errorf("CurrentCodeBase.GetLatestVersion: %w", err))
	}
	isOutdated, latestPrompt := CurrentCodeBase.PromptLatestVersion(CurrentVersion, latestVersion)

	if isOutdated {
		result.WriteString("\n")
		result.WriteString(latestPrompt.Output)
		result.WriteString(fmt.Sprintf("You can find more here:\n%s\n", latestPrompt.UpdateURL))
	}
	return result.String()
}
