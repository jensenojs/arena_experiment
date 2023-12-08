package main

import (
	"arena_experiment/pkg/buffer"
	"arena_experiment/pkg/example2"
	"fmt"
	"go/constant"
)

func main() {
	fmt.Println("hello world")

	buf := buffer.New()
	defer buf.Free()

	id := constant.MakeInt64(1)
	_ = example2.NewPerson(id, buf)
}
