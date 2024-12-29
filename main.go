package main

import (
	"context"
	"encoding/json"
	"fmt"

	// "fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/phyer/v5sdkgo/rest"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	// "v5sdk_go/ws"
	// "v5sdk_go/ws/wImpl"

	simple "github.com/bitly/go-simplejson"
	// "github.com/go-redis/redis"
	"github.com/phyer/core"
	//	"github.com/phyer/texus/private"
	"github.com/phyer/texus/utils"
)

func init() {
}

// 通过rest接口，获取所有ticker信息，存入redis的stream和成交量排行榜
func RestTicker(cr *core.Core, dura time.Duration) {

	rsp := rest.RESTAPIResult{}
	js := simple.Json{}
	itemList := []interface{}{}
	// logrus.Info("getAllTickerInfo err: ")
	rsp1, err := GetAllTickerInfo(cr)
	rsp = *rsp1
	js1, err := simple.NewJson([]byte(rsp.Body))
	js = *js1
	if err != nil {
		logrus.Error("restTicker err: ", err)
		return
	}
	if len(rsp.Body) == 0 {
		logrus.Error("rsp body is null")
		return
	}
	itemList = js.Get("data").MustArray()
	// logrus.Info("itemList length:", len(itemList))
	// 关注多少个币，在这里设置, 只需要5个币
	allTicker := cr.GetScoreList(-1)
	redisCli := cr.RedisLocalCli
	// 全部币种列表，跟特定币种列表进行比对，匹配后push到redis
	for _, v := range itemList {
		tir := core.TickerInfoResp{}
		bs, err := json.Marshal(v)
		if err != nil {
			logrus.Error("restTicker marshal err: ", err)
			return
		}
		err = json.Unmarshal(bs, &tir)
		if err != nil {
			logrus.Error("restTicker unmarshal err: ", err)
			return
		}
		ti := tir.Convert()
		isUsdt := strings.Contains(ti.InstID, "-USDT")
		if !isUsdt {
			continue
		}
		if ti.InstType != "SPOT" {
			continue
		}
		ab, _ := json.Marshal(ti)
		suffix := ""
		env := os.Getenv("GO_ENV")
		if env == "demoEnv" {
			suffix = "-demoEnv"
		}
		for _, v := range allTicker {
			if v == ti.InstID {
				wg := core.WriteLog{
					Content: ab,
					Tag:     "sardine.log.ticker." + tir.InstID,
					Id:      ti.Id,
				}
				cr.WriteLogChan <- &wg
				redisCli.Publish(core.TICKERINFO_PUBLISH+suffix, string(ab)).Result()
			}
		}
	}
}

func LoopRestTicker(cr *core.Core) {
	per1 := 1 * time.Minute
	RestTicker(cr, per1)
	limiter := time.Tick(per1)
	for {
		<-limiter
		go func() {
			RestTicker(cr, per1)
		}()
	}
}

// 统一受理发起rest请求的请求
func LoopSaveCandle(cr *core.Core) {
	for {
		ary, err := cr.RedisLocalCli.BRPop(0, "restQueue").Result()
		if err != nil {
			logrus.Error("brpop err:", err)
			continue
		}
		restQ := core.RestQueue{}
		json.Unmarshal([]byte(ary[1]), &restQ)
		// logrus.Info("before: ", restQ.InstId)
		// before:  USDT|position|key
		ary1 := strings.Split(restQ.InstId, "|")
		if ary1[0] == "USDT" {
			// "USDT-USDT" 这个没有意义，忽略
			continue
		}
		if len(ary1) > 1 && ary1[1] == "position" {
			restQ.InstId = ary1[0] + "-USDT"
		}
		// logrus.Info("after: ", restQ.InstId)
		// after:  restQueue-USDT
		go func() {
			restQ.Show(cr)
			restQ.Save(cr)
		}()
	}
}

func GetAllTickerInfo(cr *core.Core) (*rest.RESTAPIResult, error) {
	rsp, err := RestInvoke(cr, "/api/v5/market/tickers?instType=SPOT", rest.GET)
	return rsp, err
}

func RestInvoke(cr *core.Core, subUrl string, method string) (*rest.RESTAPIResult, error) {
	restUrl, _ := cr.Cfg.Config.Get("connect").Get("restBaseUrl").String()
	//ep, method, uri string, param *map[string]interface{}
	rest := rest.NewRESTAPI(restUrl, method, subUrl, nil)
	key, _ := cr.Cfg.Config.Get("credentialReadOnly").Get("okAccessKey").String()
	secure, _ := cr.Cfg.Config.Get("credentialReadOnly").Get("secretKey").String()
	pass, _ := cr.Cfg.Config.Get("credentialReadOnly").Get("okAccessPassphrase").String()
	isDemo := false
	if cr.Env == "demoEnv" {
		isDemo = true
	}
	rest.SetSimulate(isDemo).SetAPIKey(key, secure, pass)
	response, err := rest.Run(context.Background())
	if err != nil {
		logrus.Error("restInvoke1 err:", subUrl, err)
	}
	return response, err
}

func ShowSysTime(cr *core.Core) {
	rsp, _ := RestInvoke(cr, "/api/v5/public/time", rest.GET)
	logrus.Info("serverSystem time:", rsp)
}

// period: 每个循环开始的时间点，单位：秒
// delay：延时多少秒后去取此值, 单位：秒
// mdura：多少个秒之内，遍历完获取到的goins列表, 单位：秒
// barPeriod: 周期名字
// onceCount：每次获取这个coin几个当前周期的candle数据
// range: 随机的范围，从0开始到range个周期，作为查询的after值，也就是随机n个周期，去取之前的记录,对于2D，5D等数据，可以用来补全数据, range值越大，随机散点的范围越大, 越失焦

func LoopAllCoinsList(period int64, delay int64, mdura int, barPeriod string, onceCount int, rge int) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cr := core.Core{}
	cr.Init()

	// Increase buffer size to accommodate multiple updates
	// Buffer size should be large enough to handle typical batch size
	// but not so large that it consumes too much memory
	const channelBufferSize = 100
	allScoreChan := make(chan []string, channelBufferSize)

	// Add channel overflow protection
	sendWithTimeout := func(scores []string) {
		select {
		case allScoreChan <- scores:
			// Successfully sent
		case <-time.After(5 * time.Second):
			logrus.Warn("Failed to send scores to channel - buffer full")
		case <-ctx.Done():
			return
		}
	}

	mainTicker := time.NewTicker(time.Duration(period) * time.Second)
	defer mainTicker.Stop()

	var eg errgroup.Group

	// Producer goroutine
	eg.Go(func() error {
		for {
			select {
			case <-ctx.Done():
				return nil
			case <-mainTicker.C:
				if time.Now().Unix()%period != delay {
					continue
				}

				_, cancel := context.WithTimeout(ctx, 5*time.Second)
				list := cr.GetScoreList(-1)
				cancel()

				// Use the new send function with timeout
				if len(list) > 0 {
					sendWithTimeout(list)
				}
			}
		}
	})

	// Consumer goroutine
	eg.Go(func() error {
		for {
			select {
			case <-ctx.Done():
				return nil
			case allScore := <-allScoreChan:
				if len(allScore) == 0 {
					continue
				}

				// Process in batches to avoid overwhelming the system
				if err := processScores(&cr, allScore, mdura, barPeriod, onceCount, rge); err != nil {
					logrus.WithError(err).Error("Failed to process scores")
					// Consider implementing exponential backoff retry here
				}
			}
		}
	})

	if err := eg.Wait(); err != nil {
		logrus.WithError(err).Error("LoopAllCoinsList failed")
	}
}

func processScores(cr *core.Core, allScore []string, mdura int, barPeriod string, onceCount int, rge int) error {
	utils.TickerWrapper(time.Duration(mdura)*time.Second, allScore, func(i int, ary []string) error {
		nw := time.Now()
		// 使用更安全的随机数生成
		randSource := rand.NewSource(nw.UnixNano())
		randGen := rand.New(randSource)
		ct := randGen.Intn(rge)

		minutes, err := cr.PeriodToMinutes(barPeriod)
		if err != nil {
			return fmt.Errorf("failed to get period minutes: %w", err)
		}

		tmi := nw.UnixMilli()
		tmi = tmi - tmi%60000
		tmi = tmi - (int64(ct) * minutes * 60000)

		limit := onceCount
		if limit == 0 {
			limit = 100
		}

		restQ := core.RestQueue{
			InstId: ary[i],
			Bar:    barPeriod,
			WithWs: false,
			Limit:  strconv.Itoa(limit),
			After:  tmi,
		}

		js, err := json.Marshal(restQ)
		if err != nil {
			return fmt.Errorf("failed to marshal RestQueue: %w", err)
		}
		logrus.WithFields(logrus.Fields{
			"instId": ary[i],
			"bar":    barPeriod,
			"limit":  limit,
		}).Debug("Pushing to restQueue")

		return cr.RedisLocalCli.LPush("restQueue", js).Err()
	})
	return nil
}

func main() {
	cr := core.Core{}
	// level := os.Getenv("TEXUS_LOGLEVEL")
	logrus.SetLevel(logrus.DebugLevel)
	cr.Init()
	ShowSysTime(&cr)
	// 从rest接口获取的ticker记录种的交量计入排行榜，指定周期刷新一次
	go func() {
		logrus.Info("LoopRestTicker")
		LoopRestTicker(&cr)
	}()
	// 全员5m
	go func() {
		logrus.Info("LoopAllCoinsList1")
		LoopAllCoinsList(6, 0, 60, "5m", 6, 113)
	}()
	// 全员15m candle
	go func() {
		logrus.Info("LoopAllCoinsList2")
		LoopAllCoinsList(19, 90, 190, "15m", 10, 123)
	}()
	// 全员30m candle
	go func() {
		logrus.Info("LoopAllCoinsList2")
		LoopAllCoinsList(25, 0, 250, "30m", 15, 117)
	}()
	// 全员1H candle
	go func() {
		logrus.Info("LoopAllCoinsList2")
		LoopAllCoinsList(38, 0, 380, "1H", 15, 131)
	}()
	// 全员2H candle
	go func() {
		logrus.Info("LoopAllCoinsList2")
		LoopAllCoinsList(41, 0, 410, "2H", 20, 183)
	}()
	// 全员4小时candle
	go func() {
		logrus.Info("LoopAllCoinsList1")
		LoopAllCoinsList(69, 0, 690, "4H", 20, 191)
	}()
	// 全员6小时candle
	go func() {
		logrus.Info("LoopAllCoinsList1")
		LoopAllCoinsList(72, 0, 720, "6H", 20, 203)
	}()
	// 全员12小时candle
	go func() {
		logrus.Info("LoopAllCoinsList1")
		LoopAllCoinsList(89, 0, 880, "12H", 25, 217)
	}()
	// 全员1Day candle & maX
	go func() {
		logrus.Info("LoopAllCoinsList1")
		LoopAllCoinsList(94, 4, 940, "1D", 25, 221)
	}()
	// 全员2Day candle & maX
	go func() {
		logrus.Info("LoopAllCoinsList1")
		LoopAllCoinsList(192, 4, 1920, "2D", 25, 243)
	}()
	// 全员5Day candle & maX
	go func() {
		logrus.Info("LoopAllCoinsList1")
		LoopAllCoinsList(320, 4, 3200, "5D", 30, 279)
	}()
	go func() {
		LoopSaveCandle(&cr)
	}()
	go func() {
		core.WriteLogProcess(&cr)
	}()
	// 永久阻塞
	select {}
}
