package main

import (
	"fmt"
	"arena_experiment/pkg/buffer"
	"arena_experiment/pkg/bufstring"
)

func main() {
	buf := buffer.New()
	defer buf.Free()
	n := "jensen"

	p := bufstring.NewPerson(n, 23, buf)

	fmt.Println(p.Name)
	fmt.Println(p.Age)
}
