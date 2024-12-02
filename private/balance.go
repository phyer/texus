package private

import (
	"strconv"
)

// {"availEq":0,"cashBal":2397.6,"ccy":"FODL","disEq":0,"eq":2397.6,"eqUsd":340.3419189984,"frozenBal":2397.6,"ordFrozen":2397.6,"uTime":1649902953671}
// {"availEq":0,"cashBal":12987,"ccy":"ANW","disEq":0,"eq":12987,"eqUsd":358.555901184,"frozenBal":12987,"ordFrozen":12987,"uTime":1649596232202}
// {"availEq":0,"cashBal":6995.8,"ccy":"ABT","disEq":0,"eq":6995.8,"eqUsd":1082.5316450676,"frozenBal":6995.8,"ordFrozen":6995.8,"uTime":1648835098048}

// "availBal string ",
// "crossLiab string`json:""`
// "interest string`json:""`
// "isoEq string`json:""`
// "isoLiab string`json:""`
// "isoUpl":"0",
// "liab string`json:""`
// "maxLoan string 1453.92289531493594",
// "mgnRatio string ",
// "notionalLever string ",
// "ordFrozen string`json:""`
// "twap string`json:""`
// "upl string 0.570822125136023",
// "uplLiab string`json:""`
// "stgyEq":"0"

type CcyResp struct {
	AvailEq   string `json:"availEq"`
	CashBal   string `json:"cashBal"`
	Ccy       string `json:"ccy"`
	DisEq     string `json:"disEq"`
	Eq        string `json:"eq"`
	EqUsd     string `json:"eqUsd"`
	FrozenBal string `json:"frozenBal"`
	OrdFrozen string `json:"ordFrozen"`
	UTime     string `json:"uTime"`
	AvailBal  string `json:"availBal"`
}

type Ccy struct {
	AvailEq   float64 `json:"availEq"`
	CashBal   float64 `json:"cashBal"`
	Ccy       string  `json:"ccy"`
	DisEq     float64 `json:"disEq"`
	Eq        float64 `json:"eq":wa`
	EqUsd     float64 `json:"eqUsd"`
	FrozenBal float64 `json:"frozenBal"`
	OrdFrozen float64 `json:"ordFrozen"`
	UTime     int64   `json:"uTime"`
	AvailBal  float64 `json:"availBal"`
}
type BalanceResp struct {
}
type Balance struct {
}

func (ccyResp *CcyResp) Convert() (*Ccy, error) {
	ccy := Ccy{}
	ccy.Ccy = ccyResp.Ccy
	ccy.AvailEq, _ = strconv.ParseFloat(ccyResp.AvailEq, 64)
	ccy.CashBal, _ = strconv.ParseFloat(ccyResp.CashBal, 64)
	ccy.DisEq, _ = strconv.ParseFloat(ccyResp.DisEq, 64)
	ccy.Eq, _ = strconv.ParseFloat(ccyResp.Eq, 64)
	ccy.EqUsd, _ = strconv.ParseFloat(ccyResp.EqUsd, 64)
	ccy.FrozenBal, _ = strconv.ParseFloat(ccyResp.FrozenBal, 64)
	ccy.OrdFrozen, _ = strconv.ParseFloat(ccyResp.OrdFrozen, 64)
	ccy.UTime, _ = strconv.ParseInt(ccyResp.UTime, 10, 64)

	return &ccy, nil
}
func (balanceResp *BalanceResp) Convert() (*Balance, error) {
	return nil, nil
}
