package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/cinode/go/cas"
)

var usageText = `
This program requires following environmental variables to be set:

CN_CAS_LISTEN_ADDR - address to listen on, i.e. 127.0.0.1:80
CN_CAS_DATA_FOLDER - folder where data is read from / stored to

`

func main() {

	listenAddress := os.Getenv("CN_CAS_LISTEN_ADDR")
	dataFolder := os.Getenv("CN_CAS_DATA_FOLDER")

	if len(listenAddress) == 0 || len(dataFolder) == 0 {
		fmt.Print(usageText)
		os.Exit(1)
	}

	c := cas.InFileSystem(dataFolder)
	http.ListenAndServe(listenAddress, cas.WebInterface(c))

}
