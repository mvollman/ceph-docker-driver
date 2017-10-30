package main

import (
	"fmt"

	"github.com/docker/go-plugins-helpers/volume"
)

const (
	VERSION = "0.1.0"
)

func main() {

	name := "ceph"

	fmt.Println("Starting ceph-docker-driver")
	d := New()
	h := volume.NewHandler(d)
	fmt.Println(h.ServeUnix(name, 0))
}
