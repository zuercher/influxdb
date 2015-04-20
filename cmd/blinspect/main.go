package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/influxdb/influxdb/messaging"
)

func main() {

	var filePath string
	var showData bool
	flag.StringVar(&filePath, "f", "", "file to inspect")
	flag.BoolVar(&showData, "showData", false, "show log data")
	flag.Parse()

	f, e := os.Open(filePath)
	if e != nil {
		log.Fatal(e)
	}

	dec := messaging.NewMessageDecoder(f)
	index := uint64(0)
	for {
		var m messaging.Message
		if err := dec.Decode(&m); err == io.EOF {
			log.Fatalf("End of file, index: %d", index)
		}
		index = m.Index
		fmt.Printf("%s\t%d\t%d\t", m.Type, m.TopicID, m.Index)
		if showData {
			fmt.Printf(string(m.Data))
		}
		fmt.Println()
	}

}
