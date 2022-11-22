package utils

import (
	"fmt"
	"math"
	"reflect"
	"sort"
)

type BlockNumber int64

type numbers interface {
	int | int8 | int16 | int32 | int64 | uint | uint8 | uint16 | uint32 | uint64 | float32 | float64 | BlockNumber
}

func Contains[T comparable](value T, arr []T) (contains bool, indexes []int) {
	indexes = make([]int, 0, 1)
	for index, v := range arr {
		if v == value {
			indexes = append(indexes, index)
		}
	}
	return len(indexes) != 0, indexes
}

func Uniq[T comparable](values []T) []T {
	res := make([]T, 0, len(values)/2)
	for _, v := range values {
		if contains, _ := Contains(v, res); !contains {
			res = append(res, v)
		}
	}
	return res
}

func IndexOf[T numbers](findIt T, container []T) (index int, err error) {
	sort.Slice(container, func(i, j int) bool {
		return container[i] < container[j]
	})

	left, right := 0, len(container)-1

	for left < right-1 {
		middle := (left + right) / 2
		if container[middle] < findIt {
			left = middle
		} else {
			right = middle
		}
	}

	if container[right] != findIt {
		return right, fmt.Errorf("container doesn't contain it")
	}

	return right, nil
}

func GetClosest[T numbers](findIt T, container []T) (index int) {
	if index, err := IndexOf(findIt, container); err == nil {
		return index
	}
	index = 0
	delta := uint(math.MaxUint)
	for i, v := range container {
		if currDelta := math.Abs(float64(v - findIt)); currDelta < float64(delta) {
			delta = uint(currDelta)
			index = i
		}
	}
	return index
}

func AreEqual[T comparable](a, b []T) bool {
	return reflect.DeepEqual(a, b)
}
