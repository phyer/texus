package utils

func RecursiveBubble(ary []int64, length int) []int64 {
	if length == 0 {
		return ary
	}
	for idx, _ := range ary {
		if idx >= length-1 {
			break
		}
		temp := int64(0)
		if ary[idx] < ary[idx+1] { //改变成>,换成从小到大排序
			temp = ary[idx]
			ary[idx] = ary[idx+1]
			ary[idx+1] = temp
		}
	}
	length--
	RecursiveBubble(ary, length)
	return ary
}
