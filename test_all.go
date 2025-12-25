package main

import "github.com/james-orcales/golang_snacks/sh"

func main() {
	sh.Spawn(
		"go",
		"test",
		"./cli",
		"./itlog",
		"./myers",
		"./sh",
		"-count=1",
	)
}
