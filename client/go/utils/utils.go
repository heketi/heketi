package utils

import (
	"strconv"
)

func StrArrToIntArr(nums []string) []int {
	ret := []int{}
	for _, arg := range nums {
		t1, err := strconv.Atoi(arg)
		if err != nil {
			panic(err)
		}
		ret = append(ret, t1)
	}
	return ret
}
