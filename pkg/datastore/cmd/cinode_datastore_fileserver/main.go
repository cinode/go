package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/cinode/go/pkg/datastore"
)

var usageText = `
This program requires following environmental variables to be set:

CN_DS_LISTEN_ADDR - address to listen on, i.e. 127.0.0.1:80
CN_DS_DATA_FOLDER - folder where data is read from / stored to

`

func main() {

	listenAddress := os.Getenv("CN_DS_LISTEN_ADDR")
	dataFolder := os.Getenv("CN_DS_DATA_FOLDER")

	if len(listenAddress) == 0 || len(dataFolder) == 0 {
		fmt.Print(usageText)
		os.Exit(1)
	}

	c := datastore.InFileSystem(dataFolder)
	http.ListenAndServe(listenAddress, datastore.WebInterface(c))

}
