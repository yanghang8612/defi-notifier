package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"defi-notifier/bot"
	"defi-notifier/config"
	"defi-notifier/log"
	"defi-notifier/net"
	"defi-notifier/utils"

	"github.com/ethereum/go-ethereum/common"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

func main() {
	log.Init()

	trackers := make([]*bot.Tracker, 0)
	for _, trackerCfg := range config.C.WatchList {
		var (
			HE        common.Address
			addresses []common.Address

			chain = strings.ToLower(trackerCfg.Chain)
		)

		if chain == "tron" {
			HE = utils.MustDecodeBase58(config.C.HE.Tron)
			addresses = utils.ConvertTronAddresses(trackerCfg.Contracts)
		} else {
			HE = common.HexToAddress(config.C.HE.Eth)
			addresses = utils.ConvertEthAddresses(trackerCfg.Contracts)
		}
		tracker := bot.NewTracker(chain, trackerCfg.Endpoint, addresses, HE)
		trackers = append(trackers, tracker)
	}

	c := cron.New(cron.WithSeconds())

	_, _ = c.AddFunc("@every 1m", func() {
		for _, tracker := range trackers {
			go tracker.GetFilterLogs()
		}
	})

	_, _ = c.AddFunc("0 0 4 * * *", func() {
		msg := "Daily Health Check\n"
		for _, tracker := range trackers {
			msg += fmt.Sprintf("> `%s` - latest tracked block: %d\n", tracker.GetChain(), tracker.GetTrackedBlockNum())
		}
		net.ReportNotificationToSlack(msg, false)
	})

	c.Start()

	if !net.ReportNotificationToSlack("🚀 DeFi Notifier is started 🚀", false) {
		zap.S().Fatal("Failed to send startup notification to Slack Channel")
	}

	watchOSSignal(trackers)
}

func watchOSSignal(tracker []*bot.Tracker) {
	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	<-c

	for _, t := range tracker {
		t.Stop()
	}
}
