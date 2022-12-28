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
	name common.BlobName
	data []byte
}{
	// Static blobs
	{
		common.BlobName(base58.Decode("KDc2ijtWc9mGxb5hP29YSBgkMLH8wCWnVimpvP3M6jdAk")),
		[]byte("Test"),
	},
	{
		common.BlobName(base58.Decode("BG8WaXMAckEfbCuoiHpx2oMAS4zAaPqAqrgf5Q3YNzmHx")),
		[]byte("Test1"),
	},
	{
		common.BlobName(base58.Decode("2GLoj4Bk7SvjQngCT85gxWRu2DXCCjs9XWKsSpM85Wq3Ve")),
		[]byte(""),
	},

	// Dynamic blobs
	{
		common.BlobName(base58.Decode("TgMhE7Gn6uw2NVft18egKvCMu8aLqJL2YyyrrdtMm3k3N")),
		base58.Decode("12AAfqZLzPuNgPr1y3n9DX8DV8myf5u5XQ4EpgcWpMGiQfgjsJyRAuQcQZ8cG9sh8S1qkn9A7MNnau9jaj3DmzTtH1rdDPu6gomu1rjVZGKSp2MyU9R3TFtRvwgFgj5qVP4LAzaUiTXDeA96jt6cRvDxKewLwdbh7jL9QmdLMV8P7U4AaXSptb"),
	},
	{
		common.BlobName(base58.Decode("2E1nuns1Q7qxVDHy4WfaYJuTTYUYX4GajU5a56ZH2dTYf4")),
		base58.Decode("14fFC3vQH1VYpqcXLE7Wr6cPif6qcLYBXbhdNSu3DQkRiQAELiBArnT1XhCwjqBGBZ8qFckt8yu14iuiaTX31Z7USQV1um69frA9wPpscuwfHjNy3tS5cqPuEVaS6dCMvwHZa6UNdeY4sYcbaF25dB7sfihC1AxEvFgNRZMuPgrMobEj14bmfE8"),
	},
	{
		common.BlobName(base58.Decode("29ntXUeXc5oB8gfpgpmyv1atGuWEooHMGekfvpJS9trRbb")),
		base58.Decode("19Fs4jfYBBpP3nHDkYpoiU7DoN9fTc2DFF3XAuVk5gm8Hwp7CKxbekTfmc6hZo9nKkRxcZVchic7BBaYSEdUR9T4Vteur1ndsUzxDyzNezoLCZaByH3gak2f9pP5hKJtSsFWNCvpCEk5D32muJhoWsra6LUidrALBX4e7wRaYfzs2feo"),
	},
}

func TestDatasetGeneration(t *testing.T) {
	t.SkipNow()

	staticBlobName := func(content []byte) string {
		hash := sha256.Sum256(content)
		n, err := common.BlobNameFromHashAndType(hash[:], blobtypes.Static)
		if err != nil {
			panic(err)
		}
		return base58.Encode(n)
	}

	dynamicLink := func(content []byte, version uint64, num int) (string, string) {

		baseSeed := []byte{
			byte(num >> 8), byte(num),
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

		return base58.Encode(dl.BlobName()), base58.Encode(buf.Bytes())
	}

	for i, b := range testBlobs {
		if i == 0 {
			fmt.Printf("\n\t// Static blobs\n")
		}
		if i == 3 {
			fmt.Printf("\n\t// Dynamic blobs\n")
		}
		if i < 3 {
			fmt.Printf(
				"\t{\n\t\tcommon.BlobName(base58.Decode(\"%s\")),\n\t\t[]byte(\"%s\"),\n\t},\n",
				staticBlobName(b.data),
				string(b.data),
			)
		} else {
			name, data := dynamicLink(b.data, uint64(i), i)
			fmt.Printf(
				"\t{\n\t\tcommon.BlobName(base58.Decode(\"%s\")),\n\t\tbase58.Decode(\"%s\"),\n\t},\n",
				name,
				data,
			)
		}
	}

	t.FailNow()
}
