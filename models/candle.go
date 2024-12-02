package models

type Candle struct {
	InstId string  `json:"instId"` //开始时间
	Ts     int64   `json:"ts"`     // 最高价格
	O      float64 `json:"o"`      //最低价格
	C      string  `json:"c"`      //收盘价格
	Vol    string  `json:"vol"`    // 交易量，以张为单位
	VolCcy string  `json:"volCcy"` // 交易量，以币为单位
}
