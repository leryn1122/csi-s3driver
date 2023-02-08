package main

import (
	"flag"
	"log"
	"os"

	"github.com/leryn1122/csi-s3/pkg/driver"
)

func init() {
	err := flag.Set("logtostderr", "true")
	if err != nil {
		panic(err)
	}
}

var (
	endpoint = flag.String("endpoint", "unix://tmp/csi.sock", "CSI Endpoint")
	nodeID   = flag.String("nodeid", "", "Node ID")
)

func main() {
	flag.Parse()

	s3driver, err := driver.New(*nodeID, *endpoint)
	if err != nil {
		log.Fatal(err)
	}
	s3driver.Run()
	os.Exit(0)
}
