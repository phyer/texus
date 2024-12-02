package utils

import (
	"math/rand"
	"reflect"
	"strconv"
	"time"
)

func GetRandListChan(rg int, count int) []string {
	strAry := []string{}
	for i := 0; i < count; i++ {
		rand.Seed(time.Now().UnixNano())
		b := rand.Intn(rg)
		bs := strconv.Itoa(b)
		strAry = append(strAry, bs)
	}
	return strAry
}

func GetRandomString(l int) string {
	str := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	bytes := []byte(str)
	result := []byte{}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < l; i++ {
		result = append(result, bytes[r.Intn(len(bytes))])
	}
	return string(result)
}

//判断某一个值是否含在切片之中
func In_Array(val interface{}, array interface{}) (exists bool, index int) {
	exists = false
	index = -1

	switch reflect.TypeOf(array).Kind() {
	case reflect.Slice:
		s := reflect.ValueOf(array)

		for i := 0; i < s.Len(); i++ {
			if reflect.DeepEqual(val, s.Index(i).Interface()) == true {
				index = i
				exists = true
				return
			}
		}
	}

	return
}
