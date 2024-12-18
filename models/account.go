package models

type Account struct {
	UTime int64 `json:"uTime"` //币种余额信息的更新时间
}

// {
// "uTime": "1597026383085",
// "totalEq": "41624.32",
// "isoEq": "3624.32",
// "adjEq": "41624.32",
// "ordFroz": "0",
// "imr": "4162.33",
// "mmr": "4",
// "notionalUsd": "",
// "mgnRatio": "41624.32",
// "details": [
// {
// "availBal": "",
// "availEq": "1",
// "ccy": "BTC",
// "cashBal": "1",
// "uTime": "1617279471503",
// "disEq": "50559.01",
// "eq": "1",
// "eqUsd": "45078.3790756226851775",
// "frozenBal": "0",
// "interest": "0",
// "isoEq": "0",
// "liab": "0",
// "maxLoan": "",
// "mgnRatio": "",
// "notionalLever": "0.0022195262185864",
// "ordFrozen": "0",
// "upl": "0",
// "uplLiab": "0",
// "crossLiab": "0",
// "isoLiab": "0",
// "coinUsdPrice": "60000",
// "stgyEq":"0",
// "isoUpl":""
// }
// ]
// }
