package main

import (
	"fmt"
	"time"
)

func main() {
	time.Sleep(100 * time.Millisecond)
	fmt.Println("HELLO WORLD")
	time.Sleep(5 * time.Second)
}
