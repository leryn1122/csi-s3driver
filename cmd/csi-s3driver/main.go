package main

import (
	"flag"
	"fmt"
	"github.com/leryn1122/csi-s3/pkg/support"
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
	endpoint = flag.String("endpoint", "unix://csi/csi.sock", "CSI Endpoint")
	nodeID   = flag.String("nodeid", "", "Node ID")
)

func main() {
	flag.Parse()

	// Fast quick if show version.
	showVersion := flag.Bool("version", false, "Show version.")
	if *showVersion {
		fmt.Println(support.Version)
		return
	}

	s3driver, err := driver.NewDriver(*nodeID, *endpoint)
	if err != nil {
		log.Fatal(err)
	}

	if err := s3driver.Run(); err != nil {
		fmt.Printf("Failed to run driver: %s", err.Error())
		os.Exit(1)
	}
}
