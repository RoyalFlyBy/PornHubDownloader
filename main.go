package main

import (
	"context"
	"fmt"
	"github.com/BRUHItsABunny/bunterm"
	"github.com/BRUHItsABunny/gOkHttp/download"
	"github.com/BRUHItsABunny/gOkHttp/requests"
	"github.com/BRUHItsABunny/go-phub/api"
	"github.com/RoyalFlyBy/PornHubDownloader/app"
	"io"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"time"
)

func main() {
	appData, err := app.NewApp()
	if err != nil {
		panic(err)
	}

	versionOutput := appData.VersionRoutine()
	if appData.Cfg.Version {
		fmt.Println(versionOutput)
		os.Exit(0)
	}
	appData.BLog.Debug(versionOutput)

	videos := []string{}
	if appData.Cfg.Videos != "" {
		f, err := os.Open(appData.Cfg.Videos)
		if err != nil {
			panic(err)
		}
		fileBytes, err := io.ReadAll(f)
		if err != nil {
			panic(err)
		}
		videos = strings.Split(string(fileBytes), "\n")
	}
	if appData.Cfg.Video != "" {
		videos = []string{appData.Cfg.Video}
	}
	notification := make(chan struct{}, 1)
	notification <- struct{}{}
	sort.Sort(sort.StringSlice(videos))
	appData.Stats.TotalFiles.Store(uint64(len(videos)))

	// UI
	appData.Stats.PollIP(appData.DownloadClient)
	go func() {
		appData.BLog.Debug("Starting the UI thread")

		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		ticker := time.Tick(time.Second)

		term := bunterm.DefaultTerminal
		continueLoop := true
		for continueLoop {
			shouldStop := appData.Stats.GraceFulStop.Load()
			select {
			case <-c:
				appData.BLog.Debug("UI Thread: SIGTERM detected")
				shouldStop = true
				appData.Stats.Stop()
				break
			case <-ticker:
				if !appData.Cfg.Daemon {
					// Human-readable means we clear the spam
					term.ClearTerminal()
					term.MoveCursor(0, 0)
				}
				fmt.Println(appData.Stats.Tick(!appData.Cfg.Daemon))
				break
			}

			if shouldStop || appData.Stats.IdleTimeoutExceeded() {
				continueLoop = false
				if shouldStop {
					appData.BLog.Warn("UI Thread graceful stop")
				}
				appData.BLog.Warn("Downloaded all files")
				break
			}
		}
		appData.BLog.Debug("Stopping the UI thread")
	}()

	for videoIDX := 0; videoIDX < len(videos); videoIDX++ {

		if appData.Stats.GraceFulStop.Load() {
			break
		}

		videoURL := videos[videoIDX]
		appData.BLog.Debug(fmt.Sprintf("MAIN THREAD: Current URL: %s", videoURL))
		video, err := appData.Client.GetVideo(strings.TrimSpace(videoURL))
		if err != nil {
			err = fmt.Errorf("appData.Client.GetVideo: %w", err)
			appData.BLog.Warn(fmt.Sprintf("MAIN Thread: %s", err.Error()))
			continue
		}
		nameVars := appData.NameFormatter.GetVarMap()
		nameVars[app.FMTVars[0]] = video.Title
		nameVars[app.FMTVars[1]] = video.Id
		nameVars[app.FMTVars[2]] = video.VKey
		nameVars[app.FMTVars[3]] = video.User.Username
		nameVars[app.FMTVars[4]] = video.ChannelTitle
		nameVars[app.FMTVars[5]] = video.AddedOn.AsTime().Format("2006-01-02")
		nameVars[app.FMTVars[6]] = time.Now().Format("2006-01-02")
		nameVars[app.FMTVars[7]] = video.AddedOn.AsTime().Format("15-04-05")
		nameVars[app.FMTVars[8]] = time.Now().Format("15-04-05")
		nameVars[app.FMTVars[9]] = strconv.FormatInt(video.AddedOn.AsTime().UnixMilli(), 10)
		nameVars[app.FMTVars[10]] = strconv.FormatInt(video.AddedOn.AsTime().UnixMilli(), 10)
		fileName := appData.NameFormatter.Format(4, nameVars) // should sanitize
		encoding := video.GetBestEncoding()
		appData.BLog.Debug(fmt.Sprintf("Downloading file: %s", fileName))

		reqOpts := []requests.Option{
			requests.NewHeaderOption(api.GetDefaultMediaHeaders(appData.Client.Client.Device)),
		}

		newTask, err := download.NewDownloadTaskController(appData.DownloadClient, appData.Stats, fileName, fileName, encoding.Url, uint64(appData.Cfg.DownloadThreads), 0, reqOpts...)
		if err != nil {
			err = fmt.Errorf("download.NewDownloadTaskController: %w", err)
			appData.BLog.Warn(fmt.Sprintf("MAIN Thread: %s", err.Error()))
			continue
		}
		err = download.DownloadTask(context.Background(), appData.Stats, newTask, appData.DownloadClient, reqOpts...)
		if err != nil {
			err = fmt.Errorf(" download.DownloadTask: %w", err)
			appData.BLog.Warn(fmt.Sprintf("MAIN Thread: %s", err.Error()))
			continue
		}
	}
	appData.BLog.Debug("MAIN Thread: Waiting for threads to finish")
	appData.Stats.Stop()
}
