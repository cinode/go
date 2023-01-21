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
	"crypto/sha256"
	"fmt"
	"io"
	"testing"

	"github.com/cinode/go/pkg/common"
	"github.com/cinode/go/pkg/internal/blobtypes"
	"github.com/cinode/go/pkg/internal/blobtypes/dynamiclink"
	"github.com/jbenet/go-base58"
)

var testBlobs = []struct {
	name     common.BlobName
	data     []byte
	expected []byte
}{
	// Static blobs
	{
		common.BlobName(base58.Decode("KDc2ijtWc9mGxb5hP29YSBgkMLH8wCWnVimpvP3M6jdAk")),
		base58.Decode("3A836b"),
		base58.Decode("3A836b"),
	},
	{
		common.BlobName(base58.Decode("BG8WaXMAckEfbCuoiHpx2oMAS4zAaPqAqrgf5Q3YNzmHx")),
		base58.Decode("AXG4Ffv"),
		base58.Decode("AXG4Ffv"),
	},
	{
		common.BlobName(base58.Decode("2GLoj4Bk7SvjQngCT85gxWRu2DXCCjs9XWKsSpM85Wq3Ve")),
		base58.Decode(""),
		base58.Decode(""),
	},
	{
		common.BlobName(base58.Decode("251SEdnHjwyvUqX1EZnuKruta4yHMkTDed7LGoi3nUJwhx")),
		base58.Decode("12g3HAVNg3GnK7FiS5g12bpw6odTsNHeYvvrfbRUxx25rYzfektvqf9UyiPe5kjFaFkwNKSdMbfngBzB4y2P6qhX4oiAqsQRmk1AheARYo2qUPLZN4sVNDKVLEd33NaCTzr1cXtWDRAW8uicR79Vgrn8dps7f5zGUbeYCrpH6sWcng4cHqsqAcGXUSGKVMD9h"),
		base58.Decode("6WEcU7"),
	},
	{
		common.BlobName(base58.Decode("27vP1JG4VJNZvQJ4Zfhy3H5xKugurbh89B7rKTcStM9guB")),
		base58.Decode("15uXnGAfaTRinodHD8e7SzCRzbhRSeSg6EsieiBqWX3MefqRt6TsNYPJNaJKcYGPWbgtdxU7Hen9wTzgSEaSpCbTPsDtYfvThS1SGxxxAfg3dsaNco8Mr747Cum7opv4oKC5GejcuWPHt3JSrtPC88B39ApSonSWZMCDPfCfDZjbPxUAeFaYKtuGWYuvaV2op5"),
		base58.Decode("8iSPVoo"),
	},
	{
		common.BlobName(base58.Decode("e3T1HcdDLc73NHed2SFu5XHUQx5KDwgdAYTMmmEk2Ekqm")),
		base58.Decode("18SeLKZHYihSA344RpmwWK4dRKqZwhebt5Ldowo45b8j9eTabJgqW9oVtad52yZUiGXiwAax2QgkYp821evdd3eS1pBvcqiAK73vkW6ZGHh4YgzphhXMg1pbjmWHR5om5gJ6vmL9AtCPNKexhEdq4arVL4inJFc8RgtDAxaXJpW1y3Y2EvM1mg1gxbW"),
		base58.Decode(""),
	},
}

var dynamicLinkPropagationData = []struct {
	name     common.BlobName
	data     []byte
	expected []byte
}{
	{
		common.BlobName(base58.Decode("GUnL66Lyv2Qs4baxPhy59kF4dsB9HWakvTjMBjNGFLT6g")),
		base58.Decode("1qCavS1Y7uDsgWrH1h6vsqKyYSQ9WHHnvvrhD4DDkMLrmas8dXpxMJwDTJ2NzEBPCiQ45xzuK63uAnuJfy387mEwyuvZRbSuysV4ojfvnbK54XYt5CwookwwkxgX59Uu1rVgxmaKaXF2H1t3A4WJEYtc2Q5ZcrYQkZWGTAUomDtPwH4wEDuX8WmZf3maMaVHB"),
		base58.Decode("NezjDGD"),
	},
	{
		common.BlobName(base58.Decode("GUnL66Lyv2Qs4baxPhy59kF4dsB9HWakvTjMBjNGFLT6g")),
		base58.Decode("1qCavS1Y7uDsgWrH1h6vsqKyYSQ9WHHnvvrhD4DDkMLrmas8dXpxMJwDTJ2NzEBSFXaFYptxvNDX6pAVC7ddCAf5utbQrEKKrp5s6H8vKRYqz9cQVLhUwYSzH3KjJcvNPBQdETByiz7ndcpJQFRD38Uqc76UBNN6bdL1mFo4AFPngoLG4NKN75s68GAywtASN"),
		base58.Decode("JAZ3wwQ"),
	},
	{
		common.BlobName(base58.Decode("GUnL66Lyv2Qs4baxPhy59kF4dsB9HWakvTjMBjNGFLT6g")),
		base58.Decode("1qCavS1Y7uDsgWrH1h6vsqKyYSQ9WHHnvvrhD4DDkMLrmas8dXpxMJwDTJ2NzEBSFY4secQjMpcd3CY2juW63ensdZutxx3sn8QYku5aonWzRgDwxKk5A4RdR4gxGZ3sCi2XACkUmJmprLgGw8DWtAkVT7jXJKRQgYJXydJcfZqbJ3DcA49QAeXELdULWqthu"),
		base58.Decode("4iThMBD"),
	},
}

func TestDatasetGeneration(t *testing.T) {
	t.SkipNow()

	dumpBlob := func(name, content []byte, expected []byte) {
		fmt.Printf(""+
			"	{\n"+
			"		common.BlobName(base58.Decode(\"%s\")),\n"+
			"		base58.Decode(\"%s\"),\n"+
			"		base58.Decode(\"%s\"),\n"+
			"	},\n",
			base58.Encode(name),
			base58.Encode(content),
			base58.Encode(expected),
		)
	}

	static := func(data string) {
		content := []byte(data)
		hash := sha256.Sum256(content)
		n, err := common.BlobNameFromHashAndType(hash[:], blobtypes.Static)
		if err != nil {
			panic(err)
		}
		dumpBlob(n, content, []byte(data))
	}

	dynamicLink := func(data string, version uint64, seed int) {

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
		if err != nil {
			panic(err)
		}
		dl.UpdateLinkData(bytes.NewBufferString(data), version)

		buf, err := io.ReadAll(dl.CreateReader())
		if err != nil {
			panic(err)
		}

		dumpBlob(dl.BlobName(), buf, dl.EncryptedLink)
	}

	fmt.Printf("" +
		"var testBlobs = []struct {\n" +
		"	name common.BlobName\n" +
		"	data []byte\n" +
		"	expected []byte\n" +
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
		"	expected []byte\n" +
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
