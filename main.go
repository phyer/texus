package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/phyer/v5sdkgo/rest"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"
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
	fmt.Println("getAllTickerInfo err: ")
	rsp1, err := GetAllTickerInfo(cr)
	rsp = *rsp1
	js1, err := simple.NewJson([]byte(rsp.Body))
	js = *js1
	if err != nil {
		fmt.Println("restTicker err: ", err)
		return
	}
	if len(rsp.Body) == 0 {
		fmt.Println("rsp body is null")
		return
	}
	itemList = js.Get("data").MustArray()
	fmt.Println("itemList length:", len(itemList))
	// 关注多少个币，在这里设置, 只需要5个币
	allTicker := cr.GetScoreList(-1)
	redisCli := cr.RedisLocalCli
	// 全部币种列表，跟特定币种列表进行比对，匹配后push到redis
	for _, v := range itemList {
		tir := core.TickerInfoResp{}
		bs, err := json.Marshal(v)
		if err != nil {
			fmt.Println("restTicker marshal err: ", err)
			return
		}
		err = json.Unmarshal(bs, &tir)
		if err != nil {
			fmt.Println("restTicker unmarshal err: ", err)
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
			fmt.Println("brpop err:", err)
			continue
		}
		restQ := core.RestQueue{}
		json.Unmarshal([]byte(ary[1]), &restQ)
		fmt.Println("before: ", restQ.InstId)
		// before:  USDT|position|key
		ary1 := strings.Split(restQ.InstId, "|")
		if ary1[0] == "USDT" {
			// "USDT-USDT" 这个没有意义，忽略
			continue
		}
		if len(ary1) > 1 && ary1[1] == "position" {
			restQ.InstId = ary1[0] + "-USDT"
		}
		fmt.Println("after: ", restQ.InstId)
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
		fmt.Println("restInvoke1 err:", subUrl, err)
	}
	return response, err
}

func ShowSysTime(cr *core.Core) {
	rsp, _ := RestInvoke(cr, "/api/v5/public/time", rest.GET)
	fmt.Println("serverSystem time:", rsp)
}

// period: 每个循环开始的时间点，单位：秒
// delay：延时多少秒后去取此值, 单位：秒
// mdura：多少个秒之内，遍历完获取到的goins列表, 单位：秒
// barPeriod: 周期名字
// onceCount：每次获取这个coin几个当前周期的candle数据
// range: 随机的范围，从0开始到range个周期，作为查询的after值，也就是随机n个周期，去取之前的记录,对于2D，5D等数据，可以用来补全数据, range值越大，随机散点的范围越大, 越失焦

func LoopAllCoinsList(period int64, delay int64, mdura int, barPeriod string, onceCount int, rge int) {
	cr := core.Core{}
	cr.Init()
	allScoreChan := make(chan []string)
	fmt.Println("start LoopAllCoinsList: period: ", period, " delay: ", delay, " mdura:", mdura, " barPeriod: ", barPeriod, " onceCount: ", onceCount, " rge:", rge)
	per1 := 1 * time.Minute
	ticker := time.NewTicker(per1)
	go func() {
		for {
			tsi := time.Now().Unix()
			//fmt.Println("tsi, period, delay, tsi%(period): ", tsi, period, delay, tsi%(period))
			if tsi%(period) != delay {
				time.Sleep(1 * time.Second)
				continue
			}
			select {
			case <-ticker.C:
				go func() {
					// -1 是获取全部coin列表
					list := cr.GetScoreList(-1)
					fmt.Println("allCoins3", list)
					allScoreChan <- list
				}()
			}
		}
	}()
	for {
		allScore, _ := <-allScoreChan
		fmt.Println("allCoins allScore", allScore)
		if len(allScore) == 0 {
			continue
		}

		utils.TickerWrapper(time.Duration(mdura)*time.Second, allScore, func(i int, ary []string) error {
			nw := time.Now()
			rand.Seed(nw.UnixNano())
			ct := rand.Intn(rge)
			minutes, _ := cr.PeriodToMinutes(barPeriod)
			tmi := nw.UnixMilli()
			tmi = tmi - tmi%60000
			tmi = tmi - (int64(ct) * minutes * 60000)
			lm := strconv.Itoa(onceCount)
			fmt.Println("instId: ", ary[i], " limit: ", lm, " onceCount:", onceCount)
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
			fmt.Println("allCoins lpush js:", string(js))
			cr.RedisLocalCli.LPush("restQueue", js)
			return err
		})
	}
}

func main() {
	cr := core.Core{}
	cr.Init()
	ShowSysTime(&cr)
	// 从rest接口获取的ticker记录种的交量计入排行榜，指定周期刷新一次
	go func() {
		fmt.Println("LoopRestTicker")
		LoopRestTicker(&cr)
	}()
	// 全员5m
	go func() {
		fmt.Println("LoopAllCoinsList1")
		LoopAllCoinsList(6, 0, 6, "5m", 6, 32)
	}()
	// 全员15m candle
	go func() {
		fmt.Println("LoopAllCoinsList2")
		LoopAllCoinsList(19, 90, 19, "15m", 10, 33)
	}()
	// 全员30m candle
	go func() {
		fmt.Println("LoopAllCoinsList2")
		LoopAllCoinsList(25, 0, 25, "30m", 15, 34)
	}()
	// 全员1H candle
	go func() {
		fmt.Println("LoopAllCoinsList2")
		LoopAllCoinsList(38, 0, 38, "1H", 15, 36)
	}()
	// 全员2H candle
	go func() {
		fmt.Println("LoopAllCoinsList2")
		LoopAllCoinsList(41, 0, 41, "2H", 20, 37)
	}()
	// 全员4小时candle
	go func() {
		fmt.Println("LoopAllCoinsList1")
		LoopAllCoinsList(69, 0, 69, "4H", 20, 38)
	}()
	// 全员6小时candle
	go func() {
		fmt.Println("LoopAllCoinsList1")
		LoopAllCoinsList(72, 0, 72, "6H", 20, 41)
	}()
	// 全员12小时candle
	go func() {
		fmt.Println("LoopAllCoinsList1")
		LoopAllCoinsList(89, 0, 88, "12H", 25, 43)
	}()
	// 全员1Day candle & maX
	go func() {
		fmt.Println("LoopAllCoinsList1")
		LoopAllCoinsList(94, 4, 94, "1D", 25, 45)
	}()
	// 全员2Day candle & maX
	go func() {
		fmt.Println("LoopAllCoinsList1")
		LoopAllCoinsList(192, 4, 192, "2D", 25, 47)
	}()
	// 全员5Day candle & maX
	go func() {
		fmt.Println("LoopAllCoinsList1")
		LoopAllCoinsList(320, 4, 320, "5D", 30, 49)
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
