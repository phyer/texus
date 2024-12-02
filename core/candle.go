package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"
	"v5sdk_go/rest"

	simple "github.com/bitly/go-simplejson"
	"github.com/go-redis/redis"
	"phyer.click/tunas/utils"
)

type Candle struct {
	core   *Core
	InstId string
	Period string
	Data   []interface{}
	From   string
}

type MaX struct {
	Core    *Core
	InstId  string
	Period  string
	KeyName string
	Count   int
	Ts      int64
	Value   float64
	Data    []interface{}
	From    string
}

type MatchCheck struct {
	Minutes int64
	Matched bool
}

func (mc *MatchCheck) SetMatched(value bool) {
	mc.Matched = value
}

func (core *Core) GetCandlesWithRest(instId string, kidx int, dura time.Duration, maxCandles int) error {
	ary := []string{}

	wsary := core.Cfg.CandleDimentions
	for k, v := range wsary {
		matched := false
		// 这个算法的目的是：越靠后的candles维度，被命中的概率越低，第一个百分之百命中，后面开始越来越低, 每分钟都会发生这样的计算，
		// 因为维度多了的话，照顾不过来
		rand.New(rand.NewSource(time.Now().UnixNano()))
		rand.Seed(time.Now().UnixNano())
		n := (k*2 + 2) * 3
		if n < 1 {
			n = 1
		}
		b := rand.Intn(n)
		if b < 8 {
			matched = true
		}
		if matched {
			ary = append(ary, v)
		}
	}

	mdura := dura/(time.Duration(len(ary)+1)) - 50*time.Millisecond
	// fmt.Println("loop4 Ticker Start instId, dura: ", instId, dura, dura/10, mdura, len(ary), " idx: ", kidx)
	// time.Duration(len(ary)+1)
	ticker := time.NewTicker(mdura)
	done := make(chan bool)
	idx := 0
	go func(i int) {
		for {
			select {
			case <-ticker.C:
				if i >= (len(ary)) {
					done <- true
					break
				}
				rand.Seed(time.Now().UnixNano())
				b := rand.Intn(2)
				maxCandles = maxCandles * (i + b) * 2

				if maxCandles < 3 {
					maxCandles = 3
				}
				if maxCandles > 30 {
					maxCandles = 30
				}
				mx := strconv.Itoa(maxCandles)
				// fmt.Println("loop4 getCandlesWithRest, instId, period,limit,dura, t: ", instId, ary[i], mx, mdura)
				go func(ii int) {
					restQ := RestQueue{
						InstId:   instId,
						Bar:      ary[ii],
						Limit:    mx,
						Duration: mdura,
						WithWs:   true,
					}
					js, _ := json.Marshal(restQ)
					core.RedisCli.LPush("restQueue", js)
				}(i)
				i++
			}
		}
	}(idx)
	time.Sleep(dura - 10*time.Millisecond)
	ticker.Stop()
	// fmt.Println("loop4 Ticker stopped instId, dura: ", instId, dura, mdura)
	done <- true
	return nil
}

// 当前的时间毫秒数 对于某个时间段，比如3分钟，10分钟，是否可以被整除，
func IsModOf(curInt int64, duration time.Duration) bool {
	vol := int64(0)
	if duration < 24*time.Hour {
		// 小于1天
		vol = (curInt + 28800000)
	} else if duration >= 24*time.Hour && duration < 48*time.Hour {
		// 1天
		vol = curInt - 1633881600000
	} else if duration >= 48*time.Hour && duration < 72*time.Hour {
		// 2天
		vol = curInt - 1633795200000
	} else if duration >= 72*time.Hour && duration < 120*time.Hour {
		// 3天
		vol = curInt - 1633708800000
	} else if duration >= 120*time.Hour {
		// 5天
		vol = curInt - 1633795200000
	} else {
		// fmt.Println("noMatched:", curInt)
	}

	mody := vol % duration.Milliseconds()
	if mody == 0 {
		return true
	}
	return false
}

func (core *Core) SaveCandle(instId string, period string, rsp *rest.RESTAPIResult, dura time.Duration, withWs bool) {
	js, err := simple.NewJson([]byte(rsp.Body))
	if err != nil {
		fmt.Println("restTicker err: ", err, rsp.Body)
		return
	}
	if len(rsp.Body) == 0 {
		fmt.Println("rsp body is null")
		return
	}
	itemList := js.Get("data").MustArray()
	Daoxu(itemList)

	leng := len(itemList)
	for _, v := range itemList {
		candle := Candle{
			InstId: instId,
			Period: period,
			Data:   v.([]interface{}),
			From:   "rest",
		}
		//保存rest得到的candle
		saveCandle := os.Getenv("TUNAS_SAVECANDLE")
		if saveCandle == "true" {
			candle.SetToKey(core)
		}
		// 发布到allCandles|publish, 给外部订阅者用于setToKey,和给其他协程用于订阅ws
		arys := []string{ALLCANDLES_PUBLISH}
		if withWs {
			arys = append(arys, ALLCANDLES_INNER_PUBLISH)
		}
		core.AddToGeneralCandleChnl(&candle, arys)
		time.Sleep(dura / time.Duration(leng))
	}
}

func Daoxu(arr []interface{}) {
	var temp interface{}
	length := len(arr)
	for i := 0; i < length/2; i++ {
		temp = arr[i]
		arr[i] = arr[length-1-i]
		arr[length-1-i] = temp
	}
}

func (cl *Candle) SetToKey(core *Core) ([]interface{}, error) {
	data := cl.Data
	tsi, err := strconv.ParseInt(data[0].(string), 10, 64)
	tss := strconv.FormatInt(tsi, 10)
	keyName := "candle" + cl.Period + "|" + cl.InstId + "|ts:" + tss
	//过期时间：根号(当前candle的周期/1分钟)*10000

	dt, err := json.Marshal(cl.Data)
	exp := core.PeriodToMinutes(cl.Period)
	// expf := float64(exp) * 60
	expf := utils.Sqrt(float64(exp)) * 100
	extt := time.Duration(expf) * time.Minute
	curVolstr, _ := data[5].(string)
	curVol, err := strconv.ParseFloat(curVolstr, 64)
	if err != nil {
		fmt.Println("err of convert ts:", err)
	}
	curVolCcystr, _ := data[6].(string)
	curVolCcy, err := strconv.ParseFloat(curVolCcystr, 64)
	curPrice := curVolCcy / curVol
	if curPrice <= 0 {
		fmt.Println("price有问题", curPrice, "dt: ", string(dt), "from:", cl.From)
		err = errors.New("price有问题")
		return cl.Data, err
	}
	redisCli := core.RedisCli
	// tm := time.UnixMilli(tsi).Format("2006-01-02 15:04")
	// fmt.Println("setToKey:", keyName, "ts: ", tm, "price: ", curPrice, "from:", cl.From)
	redisCli.Set(keyName, dt, extt).Result()
	core.SaveUniKey(cl.Period, keyName, extt, tsi)
	return cl.Data, err
}

func (mx *MaX) SetToKey() ([]interface{}, error) {
	cstr := strconv.Itoa(mx.Count)
	tss := strconv.FormatInt(mx.Ts, 10)
	keyName := "ma" + cstr + "|candle" + mx.Period + "|" + mx.InstId + "|ts:" + tss
	//过期时间：根号(当前candle的周期/1分钟)*10000
	dt := []interface{}{}
	dt = append(dt, mx.Ts)
	dt = append(dt, mx.Value)
	dj, _ := json.Marshal(dt)
	exp := mx.Core.PeriodToMinutes(mx.Period)
	expf := utils.Sqrt(float64(exp)) * 100
	extt := time.Duration(expf) * time.Minute
	// loc, _ := time.LoadLocation("Asia/Shanghai")
	// tm := time.UnixMilli(mx.Ts).In(loc).Format("2006-01-02 15:04")
	// fmt.Println("setToKey:", keyName, "ts:", tm, string(dj), "from: ", mx.From)
	_, err := mx.Core.RedisCli.GetSet(keyName, dj).Result()
	mx.Core.RedisCli.Expire(keyName, extt)
	return dt, err
}

func (core *Core) SaveUniKey(period string, keyName string, extt time.Duration, tsi int64) {
	refName := keyName + "|refer"
	refRes, _ := core.RedisCli.GetSet(refName, 1).Result()
	core.RedisCli.Expire(refName, extt)
	if len(refRes) == 0 {
		core.SaveToSortSet(period, keyName, extt, tsi)
	}
}

// tsi: 上报时间timeStamp millinSecond
func (core *Core) SaveToSortSet(period string, keyName string, extt time.Duration, tsi int64) {
	ary := strings.Split(keyName, "ts:")
	setName := ary[0] + "sortedSet"
	z := redis.Z{
		Score:  float64(tsi),
		Member: keyName,
	}
	rs, err := core.RedisCli.ZAdd(setName, z).Result()
	if err != nil {
		fmt.Println("err of ma7|ma30 add to redis:", err)
	} else {
		fmt.Println("sortedSet add to redis:", rs, keyName)
	}
}

func (cr *Core) PeriodToMinutes(period string) int64 {
	ary := strings.Split(period, "")
	beiStr := "1"
	danwei := ""
	if len(ary) == 3 {
		beiStr = ary[0] + ary[1]
		danwei = ary[2]
	} else {
		beiStr = ary[0]
		danwei = ary[1]
	}
	cheng := 1
	bei, _ := strconv.Atoi(beiStr)
	switch danwei {
	case "m":
		{
			cheng = bei
			break
		}
	case "H":
		{
			cheng = bei * 60
			break
		}
	case "D":
		{
			cheng = bei * 60 * 24
			break
		}
	case "W":
		{
			cheng = bei * 60 * 24 * 7
			break
		}
	case "M":
		{
			cheng = bei * 60 * 24 * 30
			break
		}
	case "Y":
		{
			cheng = bei * 60 * 24 * 365
			break
		}
	default:
		{
			fmt.Println("notmatch:", danwei)
		}
	}
	return int64(cheng)
}

// type ScanCmd struct {
// baseCmd
//
// page   []string
// cursor uint64
//
// process func(cmd Cmder) error
// }
func (core *Core) GetRangeKeyList(pattern string, from time.Time) ([]*simple.Json, error) {
	// 比如，用来计算ma30或ma7，倒推多少时间范围，
	redisCli := core.RedisCli
	cursor := uint64(0)
	n := 0
	allTs := []int64{}
	var keys []string
	for {
		var err error
		keys, cursor, _ = redisCli.Scan(cursor, pattern+"*", 2000).Result()
		if err != nil {
			panic(err)
		}
		n += len(keys)
		if n == 0 {
			break
		}
	}

	// keys, _ := redisCli.Keys(pattern + "*").Result()
	for _, key := range keys {
		keyAry := strings.Split(key, ":")
		key = keyAry[1]
		keyi64, _ := strconv.ParseInt(key, 10, 64)
		allTs = append(allTs, keyi64)
	}
	nary := utils.RecursiveBubble(allTs, len(allTs))
	tt := from.UnixMilli()
	ff := tt - tt%60000
	fi := int64(ff)
	mary := []int64{}
	for _, v := range nary {
		if v < fi {
			break
		}
		mary = append(mary, v)
	}
	res := []*simple.Json{}
	for _, v := range mary {
		// if k > 1 {
		// break
		// }
		nv := pattern + strconv.FormatInt(v, 10)
		str, err := redisCli.Get(nv).Result()
		if err != nil {
			fmt.Println("err of redis get key:", nv, err)
		}
		cur, err := simple.NewJson([]byte(str))
		if err != nil {
			fmt.Println("err of create newJson:", str, err)
		}
		res = append(res, cur)
	}

	return res, nil
}

func (core *Core) InitStreamGroups() {
	redisCli := core.RedisCli
	streamNames := []string{
		"candles|stream",
		"maXs|stream",
	}
	grpNames := []string{
		"sardine1",
		"sardine2",
		"sardine3",
	}
	for _, v := range streamNames {
		for _, vv := range grpNames {
			redisCli.XGroupCreate(v, vv, "0-0").Result()
		}
	}
}
func (cr *Core) AddToGeneralCandleChnl(candle *Candle, channels []string) {
	//只让特定概率的事件发生
	// r := rand.New(rand.NewSource(time.Now().UnixNano()))
	// val := r.Float64()
	// rand.Seed(time.Now().UnixNano())
	// if from == "ws" && val > 0.90 {
	// fmt.Println("drop ", from)
	// return
	// }
	// fmt.Println("not drop ", from)
	// TODO map
	redisCli := cr.RedisCli

	ab, _ := json.Marshal(candle)
	//特定的币只发送给特定的下游接收者
	// 这么设计不太好，业务逻辑和程序架构紧耦合了，
	prename := cr.DispatchDownstreamNodes(candle.InstId)

	for _, v := range channels {
		suffix := ""
		env := os.Getenv("GO_ENV")
		if env == "demoEnv" {
			suffix = "-demoEnv"
		}
		vd := v + suffix

		// TODO FIXME cli2
		_, err := redisCli.Publish(vd, string(ab)).Result()
		if len(prename) == 0 {
			continue
		}
		nodeAdd := prename + "|" + v + suffix
		// fmt.Println("nodeAdd: ", nodeAdd)

		// TODO FIXME cli2
		_, err = redisCli.Publish(nodeAdd, string(ab)).Result()
		// fmt.Println("publish, channel,res,err:", nodeAdd, res, err, "candle:", string(ab))

		if err != nil {
			fmt.Println("err of ma7|ma30 add to redis2:", err, candle.From)
		}
	}
	// TODO 下面这个先屏蔽，不订阅ws信息，否则系统压力会太大，等有更灵活的机制再说
	// redisCli.Publish("allCandlesInner|publish"+suffix, string(ab)).Result()
}
