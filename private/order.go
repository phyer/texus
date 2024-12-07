package private

import (
	"github.com/phyer/texus/utils"
)

type OrderResp struct {
	Arg  ArgResp          `json:"arg"`
	Data []*OrderDataResp `json:"data"`
}

type ArgResp struct {
	Channel  string `json:"channel"`
	InstType string `json:"instType"`
	InstId   string `json:"instId"`
}
type OrderDataResp struct {
	InstType          string `json:"instType"`
	InstId            string `json:"instId"`
	OrdId             string `json:"ordId"`
	ClOrdId           string `json:"clOrdId"`
	Tag               string `json:"tag"`
	Px                string `json:"Px"`
	Sz                string `json:"sz"`
	NotionalUsd       string `json:"notionalUsd"`
	OrdType           string `json:"ordType"`
	Side              string `json:"side"`
	PosSide           string `json:"posSide"`
	TdMode            string `json:"tdMode"`
	TgtCcy            string `json:"tgtCcy"`
	FillSz            string `json:"fillSz"`
	FillPx            string `json:"fillPx"`
	TradeId           string `json:"tradeId"`
	AccFillSz         string `json:"accFillSz"`
	FillNotionalUsd   string `json:"FillNotionalUsd"`
	FillTime          string `json:"fillTime"`
	FillFee           string `json:"fillFee"`
	FillFeeCcy        string `json:"fillFeeCcy"`
	ExecType          string `json:"execType"`
	Source            string `json:"source"`
	State             string `json:"state"`
	AvgPx             string `json:"avgPx"`
	Lever             string `json:"lever"`
	TpTriggerPxstring string `json:"tpTriggerPxstring"`
	TpTriggerPxType   string `json:"tpTriggerPxType"`
	TpOrdPx           string `json:"tpOrdPx"`
	SlTriggerPx       string `json:"slTriggerPx"`
	SlTriggerPxType   string `json:"slTriggerPxType"`
	SlOrdPx           string `json:"slOrdPx"`
	FeeCcy            string `json:"feeCcy"`
	Fee               string `json:"fee"`
	RebateCcy         string `json:"rebateCcy"`
	Rebate            string `json:"rebate"`
	TgtCcystring      string `json:"tgtCcystring"`
	Pnl               string `json:"pnl"`
	Category          string `json:"category"`
	UTime             string `json:"uTime"`
	CTime             string `json:"cTime"`
	ReqId             string `json:"reqId"`
	AmendResult       string `json:"amendResult"`
	Code              string `json:"code"`
	Msg               string `json:"msg"`
}

type Order struct {
	CTime     int64   `json:"cTime,number"` // 订单创建时间
	UTime     int64   `json:"uTime,number"` // 订单状态更新时间
	InstId    string  `json:"instId"`
	OrdId     string  `json:"ordId"`
	ClOrdId   string  `json:"clOrdId"`          // 自定义订单ID
	Px        float64 `json:"px,number"`        // 委托价格
	Side      string  `json:"side"`             //交易方向 sell, buy
	Sz        float64 `json:"sz,number"`        // 委托总量
	AccFillSz float64 `json:"accFillSz,number"` //累计成交数量
	AvgPx     float64 `json:"avgPx,number"`     //成交均价
	State     string  `json:"state"`            //订单状态
	TgtCcy    string  `json:"tgtCcy"`           // 限定了size的单位，两个选项：base_ccy: 交易货币 ；quote_ccy：计价货币(美元)
	// canceled：撤单成功
	// live：等待成交
	// partially_filled：部分成交
	// filled：完全成交
}

func (orderResp *OrderResp) Convert() ([]*Order, error) {
	orderList := []*Order{}
	for _, v := range orderResp.Data {
		// fmt.Println("v.Sz:", v.Sz, reflect.TypeOf(v.Sz).Name())
		curOrder := Order{}
		curOrder.CTime = utils.ToInt64(v.CTime)
		curOrder.UTime = utils.ToInt64(v.UTime)
		curOrder.InstId = v.InstId
		curOrder.OrdId = v.OrdId
		curOrder.ClOrdId = v.ClOrdId
		curOrder.Side = v.Side
		curOrder.Px = utils.ToFloat64(v.Px)
		curOrder.Sz = utils.ToFloat64(v.Sz)
		curOrder.AccFillSz = utils.ToFloat64(v.AccFillSz)
		curOrder.AvgPx = utils.ToFloat64(v.AvgPx)
		curOrder.State = v.State
		orderList = append(orderList, &curOrder)
	}
	return orderList, nil
}

// func (order *Order) Process(cr *core.Core) error {
// return nil
// }
