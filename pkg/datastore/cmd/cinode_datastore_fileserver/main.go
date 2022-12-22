/*
Copyright © 2022 Bartłomiej Święcki (byo)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
