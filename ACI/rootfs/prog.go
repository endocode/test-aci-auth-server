package main

import (
	"fmt"
	"time"
)

func main() {
	for i := 3; i > 0; i -= 1 {
		fmt.Println(i)
		time.Sleep(time.Second)
	}
	fmt.Println("BANG!")
}
