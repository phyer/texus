package utils

import (
	"crypto/md5"
	"fmt"
	"reflect"
	"runtime"
	"strconv"
	"time"
)

type PushRestQ func(int, []string) error

// 获取当前函数名字
func GetFuncName() string {
	pc := make([]uintptr, 1)
	runtime.Callers(2, pc)
	f := runtime.FuncForPC(pc[0])
	return f.Name()
}

func TickerWrapper(mdura time.Duration, ary []string, pr PushRestQ) {
	done := make(chan bool)
	idx := 0
	per2 := time.Duration(mdura) / time.Duration(len(ary)+2)
	// fmt.Println("mdura, len of ary, per2: ", mdura, len(ary), per2)
	if per2 < 100*time.Millisecond {
		per2 = 100 * time.Millisecond
	}
	ticker := time.NewTicker(per2)
	go func(i int) {
		for {
			select {
			case <-ticker.C:
				if i >= (len(ary) - 1) {
					done <- true
					break
				}
				go func(i int) {
					err := pr(i, ary)
					if err != nil {
						fmt.Println("inner err: ", err)
					}
				}(i)
				i++
			}
		}
	}(idx)
	time.Sleep(mdura)
}

func HashDispatch(originName string, count uint8) int {
	data := []byte(originName)
	bytes := md5.Sum(data)
	res := uint8(0)
	for _, v := range bytes {
		res += v
	}
	res = res % count
	return int(res)
}

func IsoTime() string {
	utcTime := time.Now().UTC()
	iso := utcTime.String()
	isoBytes := []byte(iso)
	iso = string(isoBytes[:10]) + "T" + string(isoBytes[11:23]) + "Z"
	return iso
}

// 闹钟，
func Fenci(count int) bool {
	tsi := time.Now().Unix()
	tsi = tsi - tsi%60
	cha := tsi % (int64(count))
	if cha == 0 {
		return true
	}
	return false
}
func ToString(val interface{}) string {
	valstr := ""
	if reflect.TypeOf(val).Name() == "string" {
		valstr = val.(string)
	} else if reflect.TypeOf(val).Name() == "float64" {
		valstr = fmt.Sprintf("%f", val)
	} else if reflect.TypeOf(val).Name() == "int64" {
		valstr = strconv.FormatInt(val.(int64), 16)
	}
	return valstr
}

func ToInt64(val interface{}) int64 {
	vali := int64(0)
	if reflect.TypeOf(val).Name() == "string" {
		vali, _ = strconv.ParseInt(val.(string), 10, 64)
	} else if reflect.TypeOf(val).Name() == "float64" {
		vali = int64(val.(float64))
	}
	return vali
}

func ToFloat64(val interface{}) float64 {
	valf := float64(0)
	ctype := reflect.TypeOf(val).Name()
	if ctype == "string" {
		valf, _ = strconv.ParseFloat(val.(string), 64)
	} else if reflect.TypeOf(val).Name() == "float64" {
		valf = val.(float64)
	} else if reflect.TypeOf(val).Name() == "int64" {
		valf = float64(val.(int64))
	} else {
		fmt.Println("convert err:", val, ":", ctype)
	}
	return valf
}
