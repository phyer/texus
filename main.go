package main

import (
	"context"
	"encoding/json"
	"strconv"

	// "fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/go-redis/redis"
	"github.com/phyer/v5sdkgo/rest"
	"github.com/sirupsen/logrus"

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

// CheckSortedSet 函数需要相应修改
func CheckSortedSet(cr *core.Core) {
	keys, err := cr.RedisLocalCli.Keys("candle*|sortedSet").Result()
	if err != nil {
		logrus.Error("获取Redis键失败:", err)
		return
	}

	for _, key := range keys {
		period, instId := extractInfo(key)
		if period == "" || instId == "" {
			logrus.Warnf("无法从键 %s 中提取周期信息或instId", key)
			continue
		}

		members, err := cr.RedisLocalCli.ZRangeWithScores(key, 0, -1).Result()
		if err != nil {
			logrus.Errorf("获取SortedSet %s 的成员失败: %v", key, err)
			continue
		}

		missingTimes := findMissingPeriods(members, period)
		if len(missingTimes) > 0 {
			restQueue := &core.RestQueue{
				InstId: instId,
				Bar:    period,
				After:  missingTimes[0].UnixMilli(),
				Limit:  strconv.Itoa(len(missingTimes)),
				WithWs: false,
			}

			jsonData, err := json.Marshal(restQueue)
			if err != nil {
				logrus.Error("序列化RestQueue失败:", err)
				continue
			}

			err = cr.RedisLocalCli.RPush("restQueue", jsonData).Err()
			if err != nil {
				logrus.Error("推送到restQueue失败:", err)
			}
		}
	}
}

// extractInfo 从键名中提取周期信息和instId
func extractInfo(key string) (period string, instId string) {
	parts := strings.Split(key, "|")
	if len(parts) != 3 {
		return "", ""
	}
	periodPart := strings.TrimPrefix(parts[0], "candle")
	return periodPart, parts[1]
}

// findMissingPeriods 找出缺失的时间段，返回连续缺失的时间戳
func findMissingPeriods(members []redis.Z, period string) []time.Time {
	if len(members) == 0 {
		return nil
	}

	duration := parseDuration(period)
	var missingTimes []time.Time

	// 检查是否有中间缺失的部分
	for i := 1; i < len(members); i++ {
		prev := int64(members[i-1].Score)
		curr := int64(members[i].Score)
		expected := prev + duration.Milliseconds()

		if expected < curr {
			for t := expected; t < curr; t += duration.Milliseconds() {
				missingTimes = append(missingTimes, time.UnixMilli(t))
			}
			return missingTimes // 返回第一个缺失段
		}
	}

	// 如果没有中间缺失，但记录数少于300，从最后一条记录开始向后补充
	if len(members) < 300 {
		lastTimestamp := int64(members[len(members)-1].Score)
		for i := len(members); i < 300; i++ {
			lastTimestamp += duration.Milliseconds()
			missingTimes = append(missingTimes, time.UnixMilli(lastTimestamp))
		}
	}

	return missingTimes
}

// parseDuration 解析周期字符串为time.Duration
func parseDuration(period string) time.Duration {
	unit := period[len(period)-1:]
	value, _ := strconv.Atoi(period[:len(period)-1])

	switch unit {
	case "m":
		return time.Duration(value) * time.Minute
	case "H":
		return time.Duration(value) * time.Hour
	case "D":
		return time.Duration(value) * 24 * time.Hour
	default:
		return 0
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

func LoopAllCoinsList(mdura int, barPeriod string, rge int) {
	cr := core.Core{}
	cr.Init()
	allScoreChan := make(chan []string)
	// logrus.Info("start LoopAllCoinsList: period: ", period, " delay: ", delay, " mdura:", mdura, " barPeriod: ", barPeriod, " onceCount: ", onceCount, " rge:", rge)
	per1 := 1 * time.Minute
	ticker := time.NewTicker(per1)
	go func() {
		for {
			tsi := time.Now().Unix()
			if tsi%int64(mdura) != 0 {
				time.Sleep(1 * time.Second)
				continue
			}
			select {
			case <-ticker.C:
				go func() {
					// -1 是获取全部coin列表
					list := cr.GetScoreList(-1)
					// logrus.Info("allCoins3", list)
					allScoreChan <- list
				}()
			}
		}
	}()
	for {
		allScore := <-allScoreChan
		logrus.Debug("allCoins allScore", allScore)
		if len(allScore) == 0 {
			continue
		}

		utils.TickerWrapper(time.Duration(mdura)*time.Second, allScore, func(i int, ary []string) error {
			nw := time.Now()
			rand.Seed(nw.UnixNano())

			// 修改随机逻辑
			// 将随机范围分成两部分：80%的概率获取最近30%的数据，20%的概率获取剩余历史数据
			var ct int
			randVal := rand.Float64()
			switch {
			case randVal < 0.7:
				// 70%的概率获取最近15%的数据
				ct = rand.Intn(rge * 15 / 100)
			case randVal < 0.9:
				// 20%的概率获取最近15%~55%的数据
				ct = rand.Intn(rge*40/100) + (rge * 15 / 100)
			default:
				// 10%的概率获取最近55%~100%的数据
				ct = rand.Intn(rge*45/100) + (rge * 55 / 100)
			}

			minutes, _ := cr.PeriodToMinutes(barPeriod)
			tmi := nw.UnixMilli()
			tmi = tmi - tmi%60000
			tmi = tmi - (int64(ct) * minutes * 60000)
			lm := "100"
			// logrus.Info("instId: ", ary[i], " limit: ", lm, " onceCount:", onceCount)
			if lm == "0" {
				lm = "100"
			}
			restQ := core.RestQueue{
				InstId: ary[i],
				Bar:    barPeriod,
				WithWs: false,
				Limit:  lm,
				After:  tmi,
			}
			js, err := json.Marshal(restQ)
			logrus.Debug("allCoins lpush js:", string(js))
			cr.RedisLocalCli.LPush("restQueue", js)
			return err
		})
	}
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
		logrus.Info("LoopAllCoinsList - 5m")
		LoopAllCoinsList(180, "5m", 50)
	}()
	// 全员15m candle
	go func() {
		logrus.Info("LoopAllCoinsList - 15m")
		LoopAllCoinsList(360, "15m", 100)
	}()
	// 全员30m candle
	go func() {
		logrus.Info("LoopAllCoinsList - 30m")
		LoopAllCoinsList(600, "30m", 150)
	}()
	// 全员1H candle
	go func() {
		logrus.Info("LoopAllCoinsList - 1H")
		LoopAllCoinsList(900, "1H", 200)
	}()
	// 全员2H candle
	go func() {
		logrus.Info("LoopAllCoinsList - 2H")
		LoopAllCoinsList(1200, "2H", 250)
	}()
	// 全员4小时candle
	go func() {
		logrus.Info("LoopAllCoinsList - 4H")
		LoopAllCoinsList(1500, "4H", 300)
	}()
	// 全员6小时candle
	go func() {
		logrus.Info("LoopAllCoinsList - 6H")
		LoopAllCoinsList(1800, "6H", 350)
	}()
	// 全员12小时candle
	go func() {
		logrus.Info("LoopAllCoinsList - 12H")
		LoopAllCoinsList(2100, "12H", 400)
	}()
	// 全员1Day candle & maX
	go func() {
		logrus.Info("LoopAllCoinsList - 1D")
		LoopAllCoinsList(2400, "1D", 500)
	}()
	// 全员2Day candle & maX
	go func() {
		logrus.Info("LoopAllCoinsList - 2D")
		LoopAllCoinsList(3000, "2D", 600)
	}()
	// 全员5Day candle & maX
	go func() {
		logrus.Info("LoopAllCoinsList - 5D")
		LoopAllCoinsList(3600, "5D", 700)
	}()
	go func() {
		LoopSaveCandle(&cr)
	}()
	go func() {
		core.WriteLogProcess(&cr)
	}()
	go func() {
		for {
			CheckSortedSet(&cr)
			time.Sleep(30 * time.Minute)
		}
	}()
	// 永久阻塞
	select {}
}
