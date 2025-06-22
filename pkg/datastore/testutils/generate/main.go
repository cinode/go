/*
Copyright © 2025 Bartłomiej Święcki (byo)

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

//go:generate go run .
import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"fmt"
	"html/template"
	"io"
	"os"

	"github.com/cinode/go/pkg/blobtypes"
	"github.com/cinode/go/pkg/common"
	"github.com/cinode/go/pkg/internal/blobtypes/dynamiclink"
	"github.com/cinode/go/pkg/utilities/golang"
	"github.com/jbenet/go-base58"
)

func errPanic(err error) {
	if err != nil {
		panic(err)
	}
}

type blobData struct {
	Name     string
	Data     string
	Expected string
}

func static(data string) blobData {
	content := []byte(data)
	hash := sha256.Sum256(content)
	n, err := common.BlobNameFromHashAndType(hash[:], blobtypes.Static)
	errPanic(err)

	return blobData{
		Name:     n.String(),
		Data:     base58.Encode(content),
		Expected: base58.Encode(content),
	}
}

func dynamicLink(data string, version uint64, seed int) blobData {
	baseSeed := []byte{
		byte(seed >> 8), byte(seed),
		// That's some random byte sequence, fixed here to ensure we're regenerating the same dataset
		0x7b, 0x7c, 0x99, 0x35, 0x7f, 0xed, 0x93, 0xd3,
	}

	pseudoRandBuffer := []byte{}
	for i := byte(0); i < 5; i++ {
		h := sha256.Sum256(append([]byte{i}, baseSeed...))
		pseudoRandBuffer = append(pseudoRandBuffer, h[:]...)
	}

	dl, err := dynamiclink.Create(bytes.NewReader(pseudoRandBuffer))
	errPanic(err)

	pr, _, err := dl.UpdateLinkData(bytes.NewBufferString(data), version)
	errPanic(err)

	buf, err := io.ReadAll(pr.GetPublicDataReader())
	errPanic(err)

	pr, err = dynamiclink.FromPublicData(dl.BlobName(), bytes.NewReader(buf))
	errPanic(err)

	elink, err := io.ReadAll(pr.GetEncryptedLinkReader())
	errPanic(err)

	return blobData{
		Name:     dl.BlobName().String(),
		Data:     base58.Encode(buf),
		Expected: base58.Encode(elink),
	}
}

//go:embed tesblobs.go.tpl
var templateString string
var tmpl = golang.Must(template.New("testblobs").Parse(templateString))

func main() {
	fl := golang.Must(os.Create("../testblobs.go"))
	defer fl.Close()

	err := tmpl.Execute(fl, map[string]any{
		"TestBlobs": []blobData{
			static("Test"),
			static("Test1"),
			static(""),
			dynamicLink("Test", 0, 0),
			dynamicLink("Test1", 1, 1),
			dynamicLink("", 2, 2),
		},
		"DynamicLinkPropagationData": []blobData{
			dynamicLink("Test1", 10000, 999),
			dynamicLink("Test2", 20000, 999),
			dynamicLink("Test3", 20000, 999),
		},
	})
	errPanic(err)

	fmt.Println("Successfully generated testblobs.go")
}
