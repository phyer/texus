package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"v5sdk_go/rest"
	"v5sdk_go/ws"
	"v5sdk_go/ws/wImpl"

	simple "github.com/bitly/go-simplejson"
	"github.com/go-redis/redis"
	"phyer.click/tunas/private"
	"phyer.click/tunas/utils"
)

type Core struct {
	Env           string
	Cfg           *MyConfig
	RedisCli      *redis.Client
	RedisCli2     *redis.Client
	Wg            sync.WaitGroup
	RestQueueChan chan *RestQueue
	OrderChan     chan *private.Order
}
type RestQueue struct {
	InstId   string
	Bar      string
	After    int64
	Before   int64
	Limit    string
	Duration time.Duration
	WithWs   bool
}

type SubAction struct {
	ActionName string
	ForAll     bool
	MetaInfo   map[string]interface{}
}

func (rst *RestQueue) Show(cr *Core) {
	fmt.Println("restQueue:", rst.InstId, rst.Bar, rst.Limit)
}

func (rst *RestQueue) Save(cr *Core) {
	afterSec := ""
	if rst.After > 0 {
		afterSec = fmt.Sprint("&after=", rst.After)
	}
	beforeSec := ""
	if rst.Before > 0 {
		beforeSec = fmt.Sprint("&before=", rst.Before)
	}
	limitSec := ""
	if len(rst.Limit) > 0 {
		limitSec = fmt.Sprint("&limit=", rst.Limit)
	}
	link := "/api/v5/market/candles?instId=" + rst.InstId + "&bar=" + rst.Bar + limitSec + afterSec + beforeSec

	fmt.Println("restLink: ", link)
	rsp, _ := cr.RestInvoke(link, rest.GET)
	cr.SaveCandle(rst.InstId, rst.Bar, rsp, rst.Duration, rst.WithWs)
}

func (cr *Core) ShowSysTime() {
	rsp, _ := cr.RestInvoke("/api/v5/public/time", rest.GET)
	fmt.Println("serverSystem time:", rsp)
}

func (core *Core) Init() {
	core.Env = os.Getenv("GO_ENV")
	gitBranch := os.Getenv("gitBranchName")
	commitID := os.Getenv("gitCommitID")

	fmt.Println("当前环境: ", core.Env)
	fmt.Println("gitBranch: ", gitBranch)
	fmt.Println("gitCommitID: ", commitID)
	cfg := MyConfig{}
	cfg, _ = cfg.Init()
	core.Cfg = &cfg
	cli, err := core.GetRedisCli()
	core.RedisCli = cli
	core.RestQueueChan = make(chan *RestQueue)
	core.OrderChan = make(chan *private.Order)
	if err != nil {
		fmt.Println("init redis client err: ", err)
	}
}

func (core *Core) GetWsCli() (*ws.WsClient, error) {
	url, err := core.Cfg.Config.Get("connect").Get("wsPublicBaseUrl").String()
	if err != nil {
		fmt.Println("err of json decode: ", err)
	}
	pubCli, err := ws.NewWsClient("wss://" + url)
	pubCli.AddBookMsgHook(core.PubMsgDispatcher)

	if err != nil {
		fmt.Println("err of create ublic ws cli:", err)
	}
	return pubCli, err
}

func (core *Core) DispatchDownstreamNodes(originName string) string {
	nodesStr := os.Getenv("TUNAS_DOWNSTREAM_NODES")
	if len(nodesStr) == 0 {
		return ""
	}
	nodes := strings.Split(nodesStr, "|")
	count := len(nodes)
	idx := utils.HashDispatch(originName, uint8(count))
	return nodes[idx]
}

func (core *Core) Dispatch(channel string, ctype string, instId string, data interface{}) error {
	// fmt.Println("start to SaveToRedis:", channel, ctype, instId, data)
	b, err := json.Marshal(data)
	js, err := simple.NewJson(b)
	if err != nil {
		fmt.Println("err of unMarshalJson1:", js)
	}

	isUsdt := strings.Contains(instId, "-USDT")
	instType, err := js.Get("instType").String()
	if !isUsdt {
		return err
	}

	// fmt.Println("instId: ", instId)
	redisCli := core.RedisCli
	channelType := ""
	if channel == "instruments" {
		channelType = "instruments"
	} else if strings.Contains(channel, "candle") {
		channelType = "candle"
	}
	switch channelType {
	case "instruments":
		{
			// fmt.Println("isInstrument:", instId)
			if instType != "SPOT" {
				return errors.New("instType is not SPOT")
			}
			_, err = redisCli.HSet("instruments|"+ctype+"|hash", instId, b).Result()
			if err != nil {
				fmt.Println("err of hset to redis:", err)
			}
			break
		}
	case "candle":
		{
			data := data.([]interface{})
			ary := strings.Split(channel, "candle")
			// fmt.Println("dispatch candle:", ary[1], instId)
			candle := Candle{
				InstId: instId,
				Period: ary[1],
				Data:   data,
				From:   "ws",
			}
			core.WsSubscribe(&candle)
			saveCandle := os.Getenv("TUNAS_SAVECANDLE")
			if saveCandle == "true" {
				candle.SetToKey(core)
			}
			// TODO mxLen要放到core.Cfg里
			arys := []string{ALLCANDLES_PUBLISH}
			core.AddToGeneralCandleChnl(&candle, arys)
			break
		}

	default:
		{
			// data := data.([]interface{})
			// bj, _ := json.Marshal(data)
			// fmt.Println("private data:", string(bj))
			return errors.New("channel type not catched")
		}
	}
	return nil
}

func (core *Core) PubMsgDispatcher(ts time.Time, data wImpl.MsgData) error {
	instList := data.Data
	for _, v := range instList {
		core.Dispatch(data.Arg["channel"], data.Arg["instType"], data.Arg["instId"], v)
	}
	return nil
}

func (core *Core) GetRedisCli() (*redis.Client, error) {
	ru := core.Cfg.RedisConf.Url
	rp := core.Cfg.RedisConf.Password
	ri := core.Cfg.RedisConf.Index
	re := os.Getenv("REDIS_URL")
	if len(re) > 0 {
		ru = re
	}
	client := redis.NewClient(&redis.Options{
		Addr:     ru,
		Password: rp, //默认空密码
		DB:       ri, //使用默认数据库
	})
	pong, err := client.Ping().Result()
	if pong == "PONG" && err == nil {
		return client, err
	} else {
		fmt.Println("redis状态不可用:", ru, rp, ri, err)
	}

	return client, nil
}

func (core *Core) GetAllTickerInfo() (*rest.RESTAPIResult, error) {
	// GET / 获取所有产品行情信息
	rsp, err := core.RestInvoke("/api/v5/market/tickers?instType=SPOT", rest.GET)
	return rsp, err
}

func (core *Core) GetBalances() (*rest.RESTAPIResult, error) {
	// TODO 临时用了两个实现，restInvoke，复用原来的会有bug，不知道是谁的bug
	rsp, err := core.RestInvoke2("/api/v5/account/balance", rest.GET, nil)
	return rsp, err
}
func (core *Core) GetLivingOrderList() ([]*private.Order, error) {
	// TODO 临时用了两个实现，restInvoke，复用原来的会有bug，不知道是谁的bug
	params := make(map[string]interface{})
	data, err := core.RestInvoke2("/api/v5/trade/orders-pending", rest.GET, &params)
	odrsp := private.OrderResp{}
	err = json.Unmarshal([]byte(data.Body), &odrsp)
	str, _ := json.Marshal(odrsp)
	fmt.Println("convert: err:", err, " body: ", data.Body, odrsp, " string:", string(str))
	list, err := odrsp.Convert()
	fmt.Println("loopLivingOrder response data:", str)
	fmt.Println(utils.GetFuncName(), " 当前数据是 ", data.V5Response.Code, " list len:", len(list))
	return list, err
}
func (core *Core) LoopInstrumentList() error {
	for {
		time.Sleep(3 * time.Second)
		ctype := ws.SPOT

		redisCli := core.RedisCli
		counts, err := redisCli.HLen("instruments|" + ctype + "|hash").Result()
		if err != nil {
			fmt.Println("err of hset to redis:", err)
		}
		if counts == 0 {
			continue
		}
		break
	}
	return nil
}
func (core *Core) SubscribeTicker(op string) error {
	mp := make(map[string]string)

	redisCli := core.RedisCli
	ctype := ws.SPOT
	mp, err := redisCli.HGetAll("instruments|" + ctype + "|hash").Result()
	b, err := json.Marshal(mp)
	js, err := simple.NewJson(b)
	if err != nil {
		fmt.Println("err of unMarshalJson3:", js)
	}
	// fmt.Println("ticker js: ", js)
	instAry := js.MustMap()
	for k, v := range instAry {
		b = []byte(v.(string))
		_, err := simple.NewJson(b)
		if err != nil {
			fmt.Println("err of unMarshalJson4:", js)
		}
		time.Sleep(5 * time.Second)
		go func(instId string, op string) {

			redisCli := core.RedisCli
			_, err = redisCli.SAdd("tickers|"+op+"|set", instId).Result()
			if err != nil {
				fmt.Println("err of unMarshalJson5:", js)
			}
		}(k, op)
	}
	return nil
}

func (core *Core) InnerSubscribeTicker(name string, op string, retry bool) error {
	// 在这里 args1 初始化tickerList的列表
	var args []map[string]string
	arg := make(map[string]string)
	arg["instId"] = name
	arg["instType"] = ws.SPOT
	args = append(args, arg)

	if retry {
		go func(op string, args []map[string]string) {
			core.retrySubscribe(op, args)
		}(op, args)
	} else {
		go func(op string, args []map[string]string) {
			core.OnceSubscribe(op, args)
		}(op, args)
	}
	return nil
}

func (core *Core) OnceSubscribe(op string, args []map[string]string) error {
	wsCli, _ := core.GetWsCli()
	res, _, err := wsCli.PubTickers(op, args)
	// defer wsCli.Stop()
	start := time.Now()
	if err != nil {
		fmt.Println("pubTickers err:", err)
	}
	if res {
		usedTime := time.Since(start)
		fmt.Println("订阅成功！", usedTime.String())
	} else {
		fmt.Println("订阅失败！", err)
	}
	return err
}

func (core *Core) retrySubscribe(op string, args []map[string]string) error {
	wsCli, _ := core.GetWsCli()
	res, _, err := wsCli.PubTickers(op, args)
	start := time.Now()
	if err != nil {
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		val := r.Int() / 20000000000000000
		fmt.Println("pubTickers err:", err, val, "秒后重试")
		time.Sleep(time.Duration(val) * time.Second)

		return core.retrySubscribe(op, args)
	}
	if res {
		usedTime := time.Since(start)
		fmt.Println("订阅成功！", usedTime.String())
	} else {
		fmt.Println("订阅失败！", err)
	}
	return err
}

func (core *Core) TickerScore(score float64, name string) error {

	redisCli := core.RedisCli
	mb := redis.Z{
		Score:  score,
		Member: name,
	}
	_, err := redisCli.ZAdd("tradingRank", mb).Result()
	if err != nil {
		fmt.Println("zadd err:", err)
	}
	return err
}

func (core *Core) RestInvoke(subUrl string, method string) (*rest.RESTAPIResult, error) {
	restUrl, _ := core.Cfg.Config.Get("connect").Get("restBaseUrl").String()
	//ep, method, uri string, param *map[string]interface{}
	rest := rest.NewRESTAPI(restUrl, method, subUrl, nil)
	key, _ := core.Cfg.Config.Get("credentialReadOnly").Get("okAccessKey").String()
	secure, _ := core.Cfg.Config.Get("credentialReadOnly").Get("secretKey").String()
	pass, _ := core.Cfg.Config.Get("credentialReadOnly").Get("okAccessPassphrase").String()
	isDemo := false
	if core.Env == "demoEnv" {
		isDemo = true
	}
	rest.SetSimulate(isDemo).SetAPIKey(key, secure, pass)
	response, err := rest.Run(context.Background())
	if err != nil {
		fmt.Println("restInvoke1 err:", subUrl, err)
	}
	return response, err
}

func (core *Core) RestInvoke2(subUrl string, method string, param *map[string]interface{}) (*rest.RESTAPIResult, error) {
	key, err1 := core.Cfg.Config.Get("credentialReadOnly").Get("okAccessKey").String()
	secret, err2 := core.Cfg.Config.Get("credentialReadOnly").Get("secretKey").String()
	pass, err3 := core.Cfg.Config.Get("credentialReadOnly").Get("okAccessPassphrase").String()
	userId, err4 := core.Cfg.Config.Get("connect").Get("userId").String()
	restUrl, err5 := core.Cfg.Config.Get("connect").Get("restBaseUrl").String()
	if err1 != nil || err2 != nil || err3 != nil || err4 != nil || err5 != nil {
		fmt.Println(err1, err2, err3, err4, err5)
	} else {
		fmt.Println("key:", key, secret, pass, "userId:", userId, "restUrl: ", restUrl)
	}
	// reqParam := make(map[string]interface{})
	// if param != nil {
	// reqParam = *param
	// }
	// rs := rest.NewRESTAPI(restUrl, method, subUrl, &reqParam)
	isDemo := false
	if core.Env == "demoEnv" {
		isDemo = true
	}
	// rs.SetSimulate(isDemo).SetAPIKey(key, secret, pass).SetUserId(userId)
	// response, err := rs.Run(context.Background())
	// if err != nil {
	// fmt.Println("restInvoke2 err:", subUrl, err)
	// }
	apikey := rest.APIKeyInfo{
		ApiKey:     key,
		SecKey:     secret,
		PassPhrase: pass,
	}
	cli := rest.NewRESTClient(restUrl, &apikey, isDemo)
	rsp, err := cli.Get(context.Background(), subUrl, param)
	if err != nil {
		return rsp, err
	}
	fmt.Println("response:", rsp, err)
	return rsp, err
}

func (core *Core) RestPost(subUrl string, param *map[string]interface{}) (*rest.RESTAPIResult, error) {
	key, err1 := core.Cfg.Config.Get("credentialMutable").Get("okAccessKey").String()
	secret, err2 := core.Cfg.Config.Get("credentialMutable").Get("secretKey").String()
	pass, err3 := core.Cfg.Config.Get("credentialMutable").Get("okAccessPassphrase").String()
	userId, err4 := core.Cfg.Config.Get("connect").Get("userId").String()
	restUrl, err5 := core.Cfg.Config.Get("connect").Get("restBaseUrl").String()
	if err1 != nil || err2 != nil || err3 != nil || err4 != nil || err5 != nil {
		fmt.Println(err1, err2, err3, err4, err5)
	} else {
		fmt.Println("key:", key, secret, pass, "userId:", userId, "restUrl: ", restUrl)
	}
	// 请求的另一种方式
	apikey := rest.APIKeyInfo{
		ApiKey:     key,
		SecKey:     secret,
		PassPhrase: pass,
	}
	isDemo := false
	if core.Env == "demoEnv" {
		isDemo = true
	}
	cli := rest.NewRESTClient(restUrl, &apikey, isDemo)
	rsp, err := cli.Post(context.Background(), subUrl, param)
	if err != nil {
		return rsp, err
	}
	return rsp, err
}

// 我当前持有的币，每分钟刷新
func (core *Core) GetMyFavorList() []string {
	redisCli := core.RedisCli
	opt := redis.ZRangeBy{
		Min: "10",
		Max: "100000000000",
	}
	cary, _ := redisCli.ZRevRangeByScore("private|positions|sortedSet", opt).Result()
	cl := []string{}
	for _, v := range cary {
		cry := strings.Split(v, "|")[0] + "-USDT"
		cl = append(cl, cry)
	}
	return cary
}

// 得到交易量排行榜,订阅其中前N名的各维度k线,并merge进来我已经购买的币列表，这个列表是动态更新的
// 改了，不需要交易排行榜，我手动指定一个排行即可, tickersVol|sortedSet 改成 tickersList|sortedSet
func (core *Core) GetScoreList(count int) []string {

	redisCli := core.RedisCli

	curList, err := redisCli.ZRevRange("tickersList|sortedSet", 0, int64(count-1)).Result()
	if err != nil {
		fmt.Println("zrevrange err:", err)
	}
	return curList
}

func (core *Core) SubscribeCandleWithDimention(op string, instIdList []string, dimension string) {
	wsCli, err := core.GetWsCli()
	if err != nil {
		fmt.Println("ws client err", err)
	}
	err = wsCli.Start()
	// 创建ws客户端
	if err != nil {
		fmt.Println("ws client start err", err)
	}

	var args []map[string]string
	for _, vs := range instIdList {
		arg := make(map[string]string)
		arg["instId"] = vs
		args = append(args, arg)
	}
	_, _, err = wsCli.PubKLine(op, wImpl.Period(dimension), args)
	if err != nil {
		fmt.Println("pubTickers err:", err)
	}

}

// 订阅某币，先要和配置比对，是否允许订阅此币此周期，
func (core *Core) WsSubscribe(candle *Candle) error {
	wsPeriods := []string{}
	wsary := core.Cfg.Config.Get("wsDimentions").MustArray()
	for _, v := range wsary {
		wsPeriods = append(wsPeriods, v.(string))
	}
	redisCli := core.RedisCli
	period := candle.Period
	instId := candle.InstId
	from := candle.From
	sname := instId + "|" + period + "ts|Subscribed|key"

	exists, _ := redisCli.Exists(sname).Result()
	ttl, _ := redisCli.TTL(sname).Result()
	inAry, _ := utils.In_Array(period, wsPeriods)
	if !inAry {
		estr := "subscribe 在配置中此period未被订阅: " + "," + period
		// fmt.Println(estr)
		err := errors.New(estr)
		return err
	} else {
		fmt.Println("subscribe 已经订阅: ", period)
	}
	waitWs, _ := core.Cfg.Config.Get("threads").Get("waitWs").Int64()
	willSub := false
	if exists > 0 {
		if ttl > 0 {
			if from == "ws" {
				redisCli.Expire(sname, time.Duration(waitWs)*time.Second).Result()
			}
		} else {
			willSub = true
		}
	} else {
		willSub = true
	}

	if willSub {
		// 需要订阅

		instIdList := []string{}
		instIdList = append(instIdList, instId)

		core.SubscribeCandleWithDimention(ws.OP_SUBSCRIBE, instIdList, period)
		// 如果距离上次检查此candle此维度订阅状态已经过去超过2分钟还没有发现有ws消息上报，执行订阅
		dr := 1 * time.Duration(waitWs) * time.Second
		redisCli.Set(sname, 1, dr).Result()
	} else {
		// fmt.Println("拒绝订阅candles:", keyName, "tm: ", tm, "otsi:", otsi)
	}

	return nil
}

func LoopBalances(cr *Core, mdura time.Duration) {
	//协程：动态维护topScore
	ticker := time.NewTicker(mdura)
	for {
		select {
		case <-ticker.C:
			//协程：循环执行rest请求candle
			fmt.Println("loopBalance: receive ccyChannel start")
			RestBalances(cr)
		}
	}
}
func LoopLivingOrders(cr *Core, mdura time.Duration) {
	//协程：动态维护topScore
	ticker := time.NewTicker(mdura)
	for {
		select {
		case <-ticker.C:
			//协程：循环执行rest请求candle
			fmt.Println("loopLivingOrder: receive ccyChannel start")
			RestLivingOrder(cr)
		}
	}
}

func RestBalances(cr *Core) ([]*private.Ccy, error) {
	// fmt.Println("restBalance loopBalance loop start")
	ccyList := []*private.Ccy{}
	rsp, err := cr.GetBalances()
	if err != nil {
		fmt.Println("loopBalance err00: ", err)
	}
	fmt.Println("loopBalance balance rsp: ", rsp)
	if err != nil {
		fmt.Println("loopBalance err01: ", err)
		return ccyList, err
	}
	if len(rsp.Body) == 0 {
		fmt.Println("loopBalance err03: rsp body is null")
		return ccyList, err
	}
	js1, err := simple.NewJson([]byte(rsp.Body))
	if err != nil {
		fmt.Println("loopBalance err1: ", err)
	}
	itemList := js1.Get("data").GetIndex(0).Get("details").MustArray()
	// maxTickers是重点关注的topScore的coins的数量
	cli := cr.RedisCli
	for _, v := range itemList {
		js, _ := json.Marshal(v)
		ccyResp := private.CcyResp{}
		err := json.Unmarshal(js, &ccyResp)
		if err != nil {
			fmt.Println("loopBalance err2: ", err)
		}
		ccy, err := ccyResp.Convert()
		ccyList = append(ccyList, ccy)
		if err != nil {
			fmt.Println("loopBalance err2: ", err)
		}
		z := redis.Z{
			Score:  ccy.EqUsd,
			Member: ccy.Ccy + "|position|key",
		}
		res, err := cli.ZAdd("private|positions|sortedSet", z).Result()
		if err != nil {
			fmt.Println("loopBalance err3: ", res, err)
		}
		res1, err := cli.Set(ccy.Ccy+"|position|key", js, 0).Result()
		if err != nil {
			fmt.Println("loopBalance err4: ", res1, err)
		}
		bjs, _ := json.Marshal(ccy)
		tsi := time.Now().Unix()
		tsii := tsi - tsi%600
		tss := strconv.FormatInt(tsii, 10)
		cli.Set(CCYPOSISTIONS_PUBLISH+"|ts:"+tss, 1, 10*time.Minute).Result()
		fmt.Println("ccy published: ", string(bjs))
		//TODO FIXME 50毫秒，每分钟上限是1200个订单，超过就无法遍历完成
		time.Sleep(50 * time.Millisecond)
		suffix := ""
		if cr.Env == "demoEnv" {
			suffix = "-demoEnv"
		}
		// TODO FIXME cli2
		cli.Publish(CCYPOSISTIONS_PUBLISH+suffix, string(bjs)).Result()
	}
	return ccyList, nil
}

func RestLivingOrder(cr *Core) ([]*private.Order, error) {
	// fmt.Println("restOrder loopOrder loop start")
	orderList := []*private.Order{}
	list, err := cr.GetLivingOrderList()
	if err != nil {
		fmt.Println("loopLivingOrder err00: ", err)
	}
	fmt.Println("loopLivingOrder response:", list)
	go func() {
		for _, v := range list {
			fmt.Println("order orderV:", v)
			time.Sleep(30 * time.Millisecond)
			cr.OrderChan <- v
		}
	}()
	return orderList, nil
}

func (cr *Core) ProcessOrder(od *private.Order) error {
	// publish
	go func() {
		suffix := ""
		if cr.Env == "demoEnv" {
			suffix = "-demoEnv"
		}
		cn := ORDER_PUBLISH + suffix
		bj, _ := json.Marshal(od)

		// TODO FIXME cli2
		res, _ := cr.RedisCli.Publish(cn, string(bj)).Result()
		fmt.Println("order publish res: ", res, " content: ", string(bj))
		rsch := ORDER_RESP_PUBLISH + suffix
		bj1, _ := json.Marshal(res)

		// TODO FIXME cli2
		res, _ = cr.RedisCli.Publish(rsch, string(bj1)).Result()
	}()
	return nil
}

func (cr *Core) DispatchSubAction(action *SubAction) error {
	go func() {
		suffix := ""
		if cr.Env == "demoEnv" {
			suffix = "-demoEnv"
		}
		fmt.Println("action: ", action.ActionName, action.MetaInfo)
		res, err := cr.RestPostWrapper("/api/v5/trade/"+action.ActionName, action.MetaInfo)
		if err != nil {
			fmt.Println(utils.GetFuncName(), " actionRes 1:", err)
		}
		rsch := ORDER_RESP_PUBLISH + suffix
		bj1, _ := json.Marshal(res)

		// TODO FIXME cli2
		rs, _ := cr.RedisCli.Publish(rsch, string(bj1)).Result()
		fmt.Println("action rs: ", rs)
	}()
	return nil
}

func (cr *Core) RestPostWrapper(url string, param map[string]interface{}) (rest.Okexv5APIResponse, error) {
	suffix := ""
	if cr.Env == "demoEnv" {
		suffix = "-demoEnv"
	}
	res, err := cr.RestPost(url, &param)
	fmt.Println("actionRes 2:", res.V5Response.Msg, res.V5Response.Data, err)
	bj, _ := json.Marshal(res)

	// TODO FIXME cli2
	cr.RedisCli.Publish(ORDER_RESP_PUBLISH+suffix, string(bj))
	return res.V5Response, nil
}
