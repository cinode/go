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

package datastore

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/cinode/go/pkg/common"
	"github.com/cinode/go/pkg/internal/blobtypes"
	"github.com/cinode/go/pkg/internal/blobtypes/dynamiclink"
	"github.com/jbenet/go-base58"
	"golang.org/x/crypto/chacha20"
)

var testBlobs = []struct {
	name     common.BlobName
	data     []byte
	expected string
}{
	// Static blobs
	{
		common.BlobName(base58.Decode("KDc2ijtWc9mGxb5hP29YSBgkMLH8wCWnVimpvP3M6jdAk")),
		base58.Decode("3A836b"),
		"Test",
	},
	{
		common.BlobName(base58.Decode("BG8WaXMAckEfbCuoiHpx2oMAS4zAaPqAqrgf5Q3YNzmHx")),
		base58.Decode("AXG4Ffv"),
		"Test1",
	},
	{
		common.BlobName(base58.Decode("2GLoj4Bk7SvjQngCT85gxWRu2DXCCjs9XWKsSpM85Wq3Ve")),
		base58.Decode(""),
		"",
	},
	{
		common.BlobName(base58.Decode("tK1XTjci1qkd2M5EP1AU1eRyuDJP648QMdguJzYYSk8TU")),
		base58.Decode("13GSfZovmNNfrcECwJHoYVudhrSSDm7PYnxinMKJMoBhhDPzWnpYQsRr31Jc3Bb32T7aEZyxqUunwvXJKo6yhydPpseuFQuzwUq6NsR4ATGeiHCgBqKmXtDrrEcmSYoYHTggUkHSNuxwNqUGEap5Mn6j3XswtrwQRbLFJGsj9PXxVEfMmV4FpX"),
		"Test",
	},
	{
		common.BlobName(base58.Decode("85aZqZCA6FYBL572qEYw4q78Gi51XZaYntcSmPy3mnSeu")),
		base58.Decode("17eQVPLzFwyQZmwbhL5ZRNw77gU3MbZcmYWYNbLN4YXE4LLjsEb5i7enxVYLdXedK1g5YBUqUQSQeejTkJ7Gxq29kd8mose5aoBtg1TxKBhnEdzqJkPZA8T3h93puukf9pf3NqmhYA4B4ckxqLuWzZoZY8PpdLzNZTVvAZ91XKBq7r48QC8mo6g"),
		"Test1",
	},
	{
		common.BlobName(base58.Decode("nhNM314o3HKEs8u7m2cKtBLU5odPLwUAvsqzRkRs6NBHG")),
		base58.Decode("1B5g5TUccHMxSFVswfycgEnKt4gc4uYk4LJZk5GMY8jciVEnySDCn1rjk2koiFgTLBSE2acuE3btg1Thp7W9ZLmmeSW9dT3F4davrfCnxFLEkMNLUbCYzheaWfGFqyMF11yHbENqeCCctkR8qB4QkdvYzyfXeTaQBHoar1GJ54RWBNwR"),
		"",
	},
}

var dynamicLinkPropagationData = []struct {
	name     common.BlobName
	data     []byte
	expected string
}{
	{
		common.BlobName(base58.Decode("RgZmbDndPVk8eZ1bZt2Q2aTc6LnDC3ReQ1Gan1Yx3vxas")),
		base58.Decode("128HhbN4MsH7eUgmrbMnY1haWz22gW59kFG2QQ8cDA4Z4orM6hfBRzUNiJw8okKkw15SV5q4B98FrFfJHup3gahwEooEfKa1N1iPV68q5VhdjBvovqWiaFrNrYVKv2jwHh1iy4hccZiC3yKiTCQqzSmD4oEswKA7FdsTGVTLwB7W7LtAexFgaGG"),
		"Test1",
	},
	{
		common.BlobName(base58.Decode("RgZmbDndPVk8eZ1bZt2Q2aTc6LnDC3ReQ1Gan1Yx3vxas")),
		base58.Decode("128HhbN4MsH7eUgmrbMnY1haWz22gW59kFG2QQ8cDA4Z4orM6hfBS3XCxACz7FE5wwDfYJ4ATimmZWkMegLSEjPXym852fPBgUd8w8rSGy8UWNvicKRCNTME3y6ABNgMEg6me1aVRLtEwRuq1qBP2HEpGoX9nhefzoXTRBiih1hsj9wtc1HE5nh"),
		"Test2",
	},
	{
		common.BlobName(base58.Decode("RgZmbDndPVk8eZ1bZt2Q2aTc6LnDC3ReQ1Gan1Yx3vxas")),
		base58.Decode("128HhbN4MsH7eUgmrbMnY1haWz22gW59kFG2QQ8cDA4Z4orM6hfBS3XCFh3Sh74ewDtyj4TX3vBjdXh4sdALyjLsprcS9YREsUP23BrMAw1N8SsaWvt5c1Ms9Woza7e34g13H83EVpbpo6XbmJnWbrZYGzA4yGkFHCfNrnGEjeZwaZVz124KNU2"),
		"Test3",
	},
}

func TestDatasetGeneration(t *testing.T) {
	t.SkipNow()

	dumpBlob := func(name, content []byte, expected string) {
		fmt.Printf(""+
			"	{\n"+
			"		common.BlobName(base58.Decode(\"%s\")),\n"+
			"		base58.Decode(\"%s\"),\n"+
			"		\"%s\",\n"+
			"	},\n",
			base58.Encode(name),
			base58.Encode(content),
			expected,
		)
	}

	static := func(data string) {
		content := []byte(data)
		hash := sha256.Sum256(content)
		n, err := common.BlobNameFromHashAndType(hash[:], blobtypes.Static)
		if err != nil {
			panic(err)
		}
		dumpBlob(n, content, data)
	}

	dynamicLink := func(data string, version uint64, seed int) {
		content := []byte(data)

		baseSeed := []byte{
			byte(seed >> 8), byte(seed),
			// That's some random byte sequence, fixed here to ensure we're regenerating the same dataset
			0x7b, 0x7c, 0x99, 0x35, 0x7f, 0xed, 0x93, 0xd3,
		}

		edSeed := sha256.Sum256(append([]byte{0x00}, baseSeed...))
		iv := sha256.Sum256(append([]byte{0x01}, baseSeed...))
		priv := ed25519.NewKeyFromSeed(edSeed[:ed25519.SeedSize])
		pub := priv.Public().(ed25519.PublicKey)
		dl := dynamiclink.DynamicLinkData{
			PublicKey:      pub,
			ContentVersion: version,
			IV:             iv[:chacha20.NonceSizeX],
			EncryptedLink:  content,
		}
		dl.Signature = dl.CalculateSignature(priv)

		buf := bytes.NewBuffer(nil)
		dl.SendToWriter(buf)

		dumpBlob(dl.BlobName(), buf.Bytes(), data)
	}

	fmt.Printf("" +
		"var testBlobs = []struct {\n" +
		"	name common.BlobName\n" +
		"	data []byte\n" +
		"	expected string\n" +
		"}{\n" +
		"	// Static blobs\n",
	)

	for _, staticData := range []string{
		"Test",
		"Test1",
		"",
	} {
		static(staticData)
	}

	for i, d := range []string{
		"Test",
		"Test1",
		"",
	} {
		dynamicLink(d, uint64(i), i)
	}
	fmt.Printf("" +
		"}\n" +
		"\n" +
		"var dynamicLinkPropagationData = []struct{\n" +
		"	name common.BlobName\n" +
		"	data []byte\n" +
		"	expected string\n" +
		"}{\n",
	)

	dynamicLink("Test1", 10000, 999)
	dynamicLink("Test2", 20000, 999)
	dynamicLink("Test3", 20000, 999)

	fmt.Printf("" +
		"}\n",
	)
	t.FailNow()
}
