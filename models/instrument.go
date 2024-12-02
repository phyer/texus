package models

type Instrument struct {
	Alias    string  `json:"alias" mapstructure:"alias"`       //合约日期别名, 目前只关注： “this_week”
	BaseCcy  string  `json:"baseCcy" mapstructure:"baseCcy"`   //交易货币币种，如 BTC-USDT 中BTC
	Category int32   `json:"category" mapstructure:"category"` // 手续费档位，每个交易产品属于哪个档位手续费,币币交易, 第1类和第3类：卖0.080%	买0.100%，第2类:0.060%	0.060%
	CtVal    float64 `json:"ctVal" mapstructure:"ctVal"`       //合约面值
	CtValCcy string  `json:"ctValCcy" mapstructure:"ctValCcy"` //合约面值计价币种
	InstId   string  `json:"instId" mapstructure:"instId"`     // 产品ID
	InstType string  `json:"instType" mapstructure:"instType"` // 产品类型 只关注SPOT
	LotSz    float64 `json:"lotSz" mapstructure:"lotSz"`       //下单数量精度
	MinSz    int32   `json:"minSz" mapstructure:"minSz"`       //最小下单数量
	QuoteCcy string  `json:"quoteCcy" mapstructure:"quoteCcy"` // 计价货币币种，如 BTC-USDT 中 USDT ，仅适用于币币
	State    string  `json:"state" mapstructure:"state"`       //产品状态:live, suspend,expired,preopen
	TickSz   float64 `json:"tickSz" mapstructure:"tickSz"`     //下单价格精度，如0.00001
}

// {
// alias: ,
// baseCcy: AE,
// category: 2,
// ctMult: ,
// ctType: ,
// ctVal: ,
// ctValCcy: ,
// expTime: ,
// instId: AE-BTC,
// instType: SPOT,
// lever: ,
// listTime: ,
// lotSz: 0.00000001,
// minSz: 10,
// optType: ,
// quoteCcy: BTC,
// settleCcy: ,
// state: live,
// stk: ,
// tickSz: 0.00000001,
// uly:
// }
