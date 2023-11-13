package main

import (
	"fmt"
	"regexp"
)

func main() {
	//var someSlice []string
	//updateSlice(&someSlice)
	//println(someSlice[0], someSlice[1])
	//rarArchivePartialExtTwo := ".part\\d{1-3}.rar"
	str := "hello.part1.rar"
	matched, err := regexp.MatchString("part\\+.rar", str)
	if err != nil {
		panic(err)
	} else {
		fmt.Printf("has suffix: %v\n", matched)
	}
}

func updateSlice(slice *[]string) {
	someUnderlyingOp(slice)
}

func someUnderlyingOp(slice *[]string) {
	*slice = append(*slice, "hello")
	*slice = append(*slice, "world")
}
