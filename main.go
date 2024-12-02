package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"
	"v5sdk_go/rest"
	"v5sdk_go/ws"
	"v5sdk_go/ws/wImpl"

	simple "github.com/bitly/go-simplejson"
	"github.com/go-redis/redis"
	"phyer.click/tunas/core"
	"phyer.click/tunas/private"
	"phyer.click/tunas/utils"
)

func init() {
	// //fmt.Println("inited")
}

// 通过rest接口，获取所有ticker信息，存入redis的stream和成交量排行榜
func RestTicker(cr *core.Core, dura time.Duration) {

	rsp := rest.RESTAPIResult{}
	js := simple.Json{}
	itemList := []interface{}{}
	fmt.Println("getAllTickerInfo err: ")
	rsp1, err := cr.GetAllTickerInfo()
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
	// maxTickers是重点关注的topScore的coins的数量
	length, _ := cr.Cfg.Config.Get("threads").Get("maxTickers").Int()
	fmt.Println("itemList length:", len(itemList))
	if len(itemList) < length {
		return
	}
	// 关注多少个币，在这里设置, 只需要5个币
	allTicker := cr.GetScoreList(5)
	redisCli := cr.RedisCli
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
		isUsdt := strings.Contains(ti.InstId, "-USDT")
		if !isUsdt {
			continue
		}
		if ti.InstType != "SPOT" {
			continue
		}
		// 把单个ticker信息小时交易量存到交易量排行榜
		// fmt.Println("ticker item: ", item)
		// 不需要排行榜了
		// _, err = redisCli.ZAdd("tickersVol|sortedSet", redis.Z{ti.VolCcy24h, ti.InstId}).Result()
		// if err != nil {
		// 	fmt.Println("restTicker redis err: ", err)
		// }

		ab, _ := json.Marshal(ti)
		suffix := ""
		env := os.Getenv("GO_ENV")
		if env == "demoEnv" {
			suffix = "-demoEnv"
		}
		for _, v := range allTicker {
			if v == ti.InstId {
				redisCli.Publish(core.TICKERINFO_PUBLISH+suffix, string(ab)).Result()
			}
		}
	}
}

// 私有订阅: 订阅订单频道
func wsPriv(core *core.Core) error {
	cfg := core.Cfg
	url, _ := cfg.Config.Get("connect").Get("wsPrivateBaseUrl").String()
	priCli, err := ws.NewWsClient("wss://" + url)
	err = priCli.Start()
	if err != nil {
		log.Println(err)
		return err
	}
	priCli.SetDailTimeout(time.Second * 10)
	defer func() {
		priCli.Stop()
	}()
	var res bool
	key, _ := core.Cfg.Config.Get("credentialReadOnly").Get("okAccessKey").String()
	secure, _ := core.Cfg.Config.Get("credentialReadOnly").Get("secretKey").String()
	pass, _ := core.Cfg.Config.Get("credentialReadOnly").Get("okAccessPassphrase").String()
	// TODO 这里订阅收听订单频道结果，但是ws请求不可靠，这个所谓冗余机制存在，有比没有强，restPost函数 是保底方案
	priCli.AddBookMsgHook(func(ts time.Time, data wImpl.MsgData) error {
		// 添加你的方法
		fmt.Println("这是自定义AddBookMsgHook")
		bj, _ := json.Marshal(data)
		fmt.Println("当前数据是", string(bj))
		resp := private.OrderResp{}
		// TODO FIXME 这里默认所有的数据都是OrderResp类型，但是ws请求有其他类型的，这个地方将来肯定得改
		err := json.Unmarshal(bj, &resp)
		if err != nil {
			fmt.Println("Canvert MsgData to OrderResp err:", err)
			return nil
		}
		fmt.Println("order resp: ", resp)
		list, _ := resp.Convert()
		fmt.Println("order list: ", list)
		go func() {
			for _, v := range list {
				fmt.Println("order orderV:", v)
				core.OrderChan <- v
			}
		}()
		return nil
	})

	fmt.Println("key, secure, pass:", key, secure, pass)
	res, _, err = priCli.Login(key, secure, pass)

	if res {
		fmt.Println("私有订阅登录成功！")
	} else {
		fmt.Println("私有订阅登录失败！", err)
		return err
	}

	var args []map[string]string
	arg := make(map[string]string)
	arg["instType"] = ws.ANY
	args = append(args, arg)
	// 订阅订单频道动作执行
	res, msg, err := priCli.PrivBookOrder("subscribe", args)
	bj, _ := json.Marshal(msg)
	fmt.Println("PrivBookOrder res:", res, " msg:", string(bj), " err:", err)
	for {
		fmt.Println(utils.GetFuncName(), url+" is in subscribing")
		time.Sleep(10 * time.Minute)
		priCli.Stop()
		wsPriv(core)
	}
	return err
}

// 统一受理发起rest请求的请求
func LoopSaveCandle(cr *core.Core) {
	for {
		ary, err := cr.RedisCli.BRPop(0, "restQueue").Result()
		if err != nil {
			fmt.Println("brpop err:", err)
			continue
		}
		restQ := core.RestQueue{}
		json.Unmarshal([]byte(ary[1]), &restQ)
		// fmt.Println("before: ", restQ.InstId)
		// before:  USDT|position|key
		ary1 := strings.Split(restQ.InstId, "|")
		if ary1[0] == "USDT" {
			// "USDT-USDT" 这个没有意义，忽略
			continue
		}
		if len(ary1) > 1 && ary1[1] == "position" {
			restQ.InstId = ary1[0] + "-USDT"
		}
		// fmt.Println("after: ", restQ.InstId)
		// after:  restQueue-USDT
		go func() {
			restQ.Show(cr)
			restQ.Save(cr)
		}()
	}
}

// 这个函数被废弃了, 根据现有逻辑，redisCli.Scan(cursor, "*"+pattern+"*", 2000) 得不到任何内容
func LoopRestQ(cr *core.Core) {
	redisCli := cr.RedisCli
	cursor := uint64(0)
	n := 0
	// allTs := []int64{}
	rstr := []string{}
	pattern := "restQueue"
	fmt.Println("LoopRestQ start")
	for {
		var err error
		rstr, cursor, err = redisCli.Scan(cursor, "*"+pattern+"*", 2000).Result()
		if err != nil {
			panic(err)
		}
		n += len(rstr)
		if n == 0 {
			break
		}
		if n > 200000 {
			break
		}
		if n > 0 {
			fmt.Println("LoopRestQ rstr: ", rstr)
		}
	}
	// fmt.Println("LoopRestQ rstr: ", rstr)
	fmt.Println("LoopRestQ rstr len: ", len(rstr))
	for _, keyN := range rstr {
		val, err := redisCli.Get(keyN).Result()
		fmt.Println("LoopRestQ val:", val)
		restQ := core.RestQueue{}
		if err != nil || len(val) == 0 {
			continue
		}
		err = json.Unmarshal([]byte(val), &restQ)
		if err != nil {
			fmt.Println("RestQueue Unmarshal err: ", err)
		}
		// res, err := redisCli.LPush("restQueue", val).Result()
		fmt.Println("restQueue will LPush: ", val)
		if err != nil {
			redisCli.Del(keyN)
		}
	}
	time.Sleep(10 * time.Second)
	LoopRestQ(cr)
}

func LoopSubscribe(cr *core.Core) {
	redisCli := cr.RedisCli
	suffix := ""
	if cr.Env == "demoEnv" {
		suffix = "-demoEnv"
	}
	pubsub := redisCli.Subscribe(core.ALLCANDLES_INNER_PUBLISH + suffix)
	_, err := pubsub.Receive()
	if err != nil {
		log.Fatal(err)
	}
	// 用管道来接收消息
	ch := pubsub.Channel()
	// 处理消息

	for msg := range ch {
		cd := core.Candle{}
		json.Unmarshal([]byte(msg.Payload), &cd)
		fmt.Println("msg.PayLoad:", msg.Payload, "candle:", cd)
		go func() {
			cr.WsSubscribe(&cd)
		}()
	}
}

// 订阅并执行sardine端传来的订单相关的动作
func LoopSubscribeSubAction(cr *core.Core) {
	redisCli := cr.RedisCli
	suffix := ""
	if cr.Env == "demoEnv" {
		suffix = "-demoEnv"
	}
	// TODO FIXME cli2
	prisub := redisCli.Subscribe(core.SUBACTIONS_PUBLISH + suffix)
	_, err := prisub.Receive()
	if err != nil {
		log.Fatal(err)
	}
	// 用管道来接收消息
	ch := prisub.Channel()
	// 处理消息

	for msg := range ch {
		action := core.SubAction{}
		json.Unmarshal([]byte(msg.Payload), &action)
		fmt.Println("actionMsg.PayLoad:", msg.Payload, "action:", action)
		go func() {
			cr.DispatchSubAction(&action)
		}()
	}
}

// period: 每个循环开始的时间点，单位：秒
// delay：延时多少秒后去取此值, 单位：秒
// mdura：多少个分钟之内，遍历完获取到的goins列表, 单位：秒
// onceCount：每次获取这个coin几个当前周期的candle数据
// range: 随机的范围，从0开始到range个周期，作为查询的after值，也就是随机n个周期，去取之前的记录,对于2D，5D等数据，可以用来补全数据, range值越大，随机散点的范围越大, 越失焦

func LoopAllCoinsList(period int64, delay int64, mdura int, barPeriod string, onceCount int, rge int) {
	cr := core.Core{}
	cr.Init()
	allScoreChan := make(chan []string)
	// fmt.Println("allCoins1")
	per1 := 1 * time.Minute
	ticker := time.NewTicker(per1)
	go func() {
		for {
			tsi := time.Now().Unix()
			// fmt.Println("tsi, period, delay, tsi%(period): ", tsi, period, delay, tsi%(period))
			if tsi%(period) != delay {
				time.Sleep(1 * time.Second)
				continue
			}
			select {
			case <-ticker.C:
				go func() {
					// -1 是获取全部coin列表
					list := cr.GetScoreList(5)
					// fmt.Println("allCoins3", list)
					allScoreChan <- list
				}()
			}
		}
	}()
	for {
		allScore, _ := <-allScoreChan
		// fmt.Println("allCoins allScore", allScore)
		if len(allScore) == 0 {
			continue
		}

		utils.TickerWrapper(time.Duration(mdura)*time.Second, allScore, func(i int, ary []string) error {
			nw := time.Now()
			rand.Seed(nw.UnixNano())
			ct := rand.Intn(rge)
			minutes := cr.PeriodToMinutes(barPeriod)
			tmi := nw.UnixMilli()
			tmi = tmi - tmi%60000
			tmi = tmi - (int64(ct) * minutes * 60000)
			fmt.Println("instId: ", ary[i])
			restQ := core.RestQueue{
				InstId: ary[i],
				Bar:    barPeriod,

				WithWs: false,
				After:  tmi,
			}
			js, err := json.Marshal(restQ)
			// fmt.Println("allCoins lpush js:", string(js))
			cr.RedisCli.LPush("restQueue", js)
			return err
		})
	}
}

func LoopCheckMyFavorList(core *core.Core) {
	maxTickers, _ := core.Cfg.Config.Get("threads").Get("maxTickers").Int()
	myFavorChan := make(chan []string)
	//协程：动态维护topScore
	go func(mx int) {
		myFavor := []string{}
		for {
			myFavor = core.GetMyFavorList()
			if len(myFavor) < mx {
				fmt.Println("topScore 长度不符合预期")
				break
			} else {
				fmt.Println("topScore 长度符合预期")
			}
			myFavorChan <- myFavor
			time.Sleep(12 * time.Minute)
		}
	}(maxTickers)
	//协程：循环执行rest请求candle

	for {
		myFavor, _ := <-myFavorChan
		go func() {
			loop2(core, myFavor, maxTickers)
		}()
	}
}

func loop2(core *core.Core, topScore []string, maxTickers int) {
	restPeriod, _ := core.Cfg.Config.Get("threads").Get("restPeriod").Int()
	maxCandles, _ := core.Cfg.Config.Get("threads").Get("maxCandles").Int()
	if len(topScore) < maxTickers {
		return
	}
	// fmt.Println("loop1 ", 12*time.Minute, " topScore2: ", topScore)
	// fmt.Println("topScore: ", topScore, len(topScore), mx, ok)
	//每隔Period1，重新发起一次rest请求的大循环。里面还有60秒一次的小循环. 作为ws请求的备份(保底)机制,实效性差了一点，但是稳定性高于ws
	dura := time.Duration(restPeriod) * time.Second

	mdura := dura/time.Duration(len(topScore)) - 20*time.Millisecond
	ticker := time.NewTicker(mdura)
	done := make(chan bool)
	idx := 0
	go func(i int) {
		for {
			select {
			case <-ticker.C:
				if i >= 4 { //: 12分钟 / 3分钟 = 4
					done <- true
					break
				}
				for {
					//内层循环3分钟一圈，3分钟内遍历完topScore限定的candle列表, 12分钟能够跑4圈
					if i >= 4 { //: 12分钟 / 3分钟 = 4
						done <- true
						break
					}
					go func() {
						// fmt.Println("loop2 :", "dura:", dura, "i:", i)
						// core.InVokeCandle(topScore, per1, maxCandles)
						mdura := dura / time.Duration(len(topScore)+1)
						for k, v := range topScore {
							go func(k int, v string) {
								time.Sleep(mdura*time.Duration(k) - 10*time.Millisecond)
								core.GetCandlesWithRest(v, k, mdura, maxCandles)
							}(k, v)
						}
						i++
					}()
					time.Sleep(dura)
					continue
				}
			}
		}
	}(idx)
	time.Sleep(dura - 100*time.Millisecond)
	done <- true
	ticker.Stop()
}

// 订阅公共频道
func wsPub(core *core.Core) {
	wsCli, _ := core.GetWsCli()
	err := wsCli.Start()
	// 创建ws客户端
	if err != nil {
		//fmt.Println("ws client err", err)
		return
	}

	// 设置连接超时

	wsCli.SetDailTimeout(time.Second * 20)

	defer wsCli.Stop()
	// 订阅产品频道
	// 在这里初始化instrument列表
	var args []map[string]string
	arg := make(map[string]string)
	arg["instType"] = ws.SPOT
	//arg["instType"] = OPTION
	args = append(args, arg)

	// start := time.Now()
	//订阅

	//设置订阅事件的event handler
	wsCli.AddBookMsgHook(core.PubMsgDispatcher)
	res, _, err := wsCli.PubInstruemnts(ws.OP_SUBSCRIBE, args)
	//fmt.Println("args:", args)
	if res {
		// usedTime := time.Since(start)
		//fmt.Println("订阅成功！", usedTime.String())
	} else {
		//fmt.Println("订阅失败！", err)
	}
	core.LoopInstrumentList()

	// start = time.Now()
	// res, _, err = r.PubInstruemnts(OP_UNSUBSCRIBE, args)
	// if res {
	// usedTime := time.Since(start)
	// //fmt.Println("取消订阅成功！", usedTime.String())
	// } else {
	// //fmt.Println("取消订阅失败！", err)
	// }
}

// 处理 ws 订单请求相关订阅回调
func subscribeOrder(cr *core.Core) {
	fmt.Println("subscribeOrder order:")
	for {
		order := <-cr.OrderChan
		go func() {
			bj, _ := json.Marshal(order)
			fmt.Println("process order:", string(bj))
			cr.ProcessOrder(order)
		}()
	}
}
func main() {
	cr := core.Core{}
	cr.Init()
	cr.ShowSysTime()
	// 从rest接口获取的ticker记录种的交量计入排行榜，指定周期刷新一次
	go func() {
		per1 := 1 * time.Minute
		RestTicker(&cr, per1)
		limiter := time.Tick(per1)
		for {
			<-limiter
			go func() {
				RestTicker(&cr, per1)
			}()
		}
	}()
	//全员1分钟candle & maX
	// period: 每个循环开始的时间点，单位：秒
	// delay：延时多少秒后去取此值
	// mdura：多少秒之内，遍历完获取到的goins列表
	// onceCount：每次获取这个coin几个当前周期的candle数据
	// 全员3m
	// go func() {
	// fmt.Println("LoopAllCoinsList1")
	// LoopAllCoinsList(180, 0, 180, "3m", 5, 6)
	// }()

	// 全员15m candle
	//	go func() {
	//		fmt.Println("LoopAllCoinsList2")
	//		LoopAllCoinsList(380, 90, 380, "15m", 4, 7)
	//	}()
	// 全员30m candle
	// go func() {
	// fmt.Println("LoopAllCoinsList2")
	// LoopAllCoinsList(510, 90, 500, "30m", 5, 8)
	// }()
	// 全员1H candle
	//	go func() {
	//		fmt.Println("LoopAllCoinsList2")
	//		LoopAllCoinsList(770, 0, 760, "1H", 9, 12)
	//	}()
	// 全员2H candle
	go func() {
		fmt.Println("LoopAllCoinsList2")
		LoopAllCoinsList(820, 0, 820, "2H", 12, 15)
	}()
	// 全员4小时candle
	//	go func() {
	//		fmt.Println("LoopAllCoinsList1")
	//		LoopAllCoinsList(1280, 150, 1280, "4H", 15, 19)
	//	}()
	// 全员6小时candle
	//go func() {
	//	fmt.Println("LoopAllCoinsList1")
	//	LoopAllCoinsList(1440, 180, 1440, "6H", 17, 21)
	//}()
	// 全员12小时candle
	go func() {
		fmt.Println("LoopAllCoinsList1")
		LoopAllCoinsList(1680, 180, 1680, "12H", 19, 23)
	}()
	// 全员1Day candle & maX
	//go func() {
	//		fmt.Println("LoopAllCoinsList1")
	//		LoopAllCoinsList(1920, 4, 1920, "1D", 25, 30)
	//	}()
	// 全员2Day candle & maX
	go func() {
		fmt.Println("LoopAllCoinsList1")
		LoopAllCoinsList(3840, 220, 3840, "2D", 26, 31)
	}()
	// 全员5Day candle & maX
	go func() {
		fmt.Println("LoopAllCoinsList1")
		LoopAllCoinsList(6400, 4, 6400, "5D", 28, 35)
	}()
	// 循环检查tickersVol|sortedSet,并执行订阅candles
	go func() {
		LoopCheckMyFavorList(&cr)
	}()

	go func() {
		LoopSaveCandle(&cr)
	}()
	go func() {
		LoopSubscribe(&cr)
	}()
	// 订阅下游sardine发过来的要执行的动作
	go func() {
		LoopSubscribeSubAction(&cr)
	}()
	// 废弃
	// go func() {
	// 	LoopRestQ(&cr)
	// }()

	//-----------
	//私有部分
	go func() {
		core.LoopLivingOrders(&cr, 1*time.Minute)
	}()
	go func() {
		core.LoopBalances(&cr, 1*time.Minute)
	}()

	// 公共订阅
	// wsPub(&cr)

	// 停止私有订阅

	//	go func() {
	//		//正常情况下 wsPrive不会主动退出，如果不慎退出了，自动重新运行
	//		for {
	//			wsPriv(&cr)
	//		}
	//}()

	go func() {
		subscribeOrder(&cr)
	}()
	// gcl := map[string]models.GlobalCoin{}
	// msgr := cr.Messager{}
	// msgr.Init()
	// msgr.Login()
	// msgr.Alive()
	// msgr.GlobalSubscribe() // 订阅全局该订阅的公共和私有内容
	// msgr.Dispatcher(gcl)
	// msgr.Pop()

	// //fmt.Println("listenning ... ")
	// 这个地方为了让main不退出, 将来可以改成一个http的listener
	// ip := "0.0.0.0:6066"
	// if err := http.ListenAndServe(ip, nil); err != nil {
	// fmt.Printf("start pprof failed on %s\n", ip)
	// }
	// 永久阻塞
	select {}
}
