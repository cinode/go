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
		base58.Decode("1DhLfjA9ij9QFBh7J8ysnN3uvGcsNQa7vaxKEwbYEMSEXuZbgyCtUAn5AaHsUNyCu1SdS2NK5zbR5aajZmfKBVNk6yHdJYptd2h1YaMG5VKXa8KPX7poiJos8jsEA556SSt35fEU3Z3smUXkUeZADGruynLu2Q4PP8gLGi5SfWwrpVNTBZ83TdiFBfEWmxcy9eu83zaBhS9F3cvof8cRDmnr5bfVA9eJuRCkxfwbbEnVBHNZufo869hdgELceG6cubUcmJCUtUW2CzNJ9WzLjHbW1ubF"),
		base58.Decode("5AM91wasb75vs4VVcfx3NuNBazfASaGPWSkxcH6bAf2bUx7EGQarimgvnQaFB4uHpb4VQA1jf9pfGGUeBAXRjg2k6h9ZaRAH"),
	},
	{
		common.BlobName(base58.Decode("27vP1JG4VJNZvQJ4Zfhy3H5xKugurbh89B7rKTcStM9guB")),
		base58.Decode("1eE2wp1836WtQmEbjdavggJvFPU7dZbQQH5EBS2LwBL2rYjArM9mjvW9xd8FkH5Vc12r4JRRgBrRi9dWVmTn591BRHyTkvwZEEqB3Ts5rkLH3B21GWAJoAmZWtSxgb7qoiiEH5WpZyLQfdfkHWfQjvY1C5ZoN83noQtnjWEBusDVgzVKhV8kjfMPQdxPMsaTLuagBq4oGaACKDppx7aq5jSpm7RMAmYHwf63QdSYp4mVCBuWzPVftTwZVKU7fYZ8z3oa4yCvAuNMxfgKem6p9kEaNjB3w"),
		base58.Decode("mvMBBYVQrqi6zF9DpiNWsj23sqw8XzJyTm5Xsa1pE3kusSQViLS7mixjPHPNkJ5u1QAww2Ww3gmGnZG4xW2Z5ycW6Rdv5gZBP"),
	},
	{
		common.BlobName(base58.Decode("e3T1HcdDLc73NHed2SFu5XHUQx5KDwgdAYTMmmEk2Ekqm")),
		base58.Decode("1yULPpEx3gjpKNBLCEzb2oj2xRcdGfr88CztgfYfEipBGiJCijqWBEEd4RaPZkgiKULSUW88xVHHvTiLXPcNpJY73jbmit4xEibbfbcHuLR5dAgzn1FpQabFx6gqRps5XagyGbW3FZxcYv2mCosEdNx4XLrzJbuEkGJUWStPhKeRoKqkGvC9XqCNwtZ9DTfnkg548HEnZhAbJ4GhuAkQEQBNR7MYcnAMw6FrpPbPEbUKuJE8mZmivTWAoiCpNgrUj4MvtRv4o64vKUFm6MNG6N"),
		base58.Decode("2d5n9Ye1eCCNRKx5SJNx4BJPjkHsDYEKuzNCvVXp7iJDZoryQ3vuQoqt9vi8TuRTZXY2EPmtavK6a3X8Frnt1GDKZES"),
	},
}

var dynamicLinkPropagationData = []struct {
	name     common.BlobName
	data     []byte
	expected []byte
}{
	{
		common.BlobName(base58.Decode("GUnL66Lyv2Qs4baxPhy59kF4dsB9HWakvTjMBjNGFLT6g")),
		base58.Decode("17Jk3QJMCABypJWuAivYwMi43gN1KxPVy3qg1e4HYQFe8BCQPVm6GX8XaSnefzn8fZ14SKm5ESRMQVorXigesA1nmAzfHm9VQZx6QM7weHynyRktGepx7pXpwVkvtHvJxPrsH1ffyhzFtnadzzXgrhzoU4Seq87oD27dY5Mk3Xnodq6a47yhoTMUPNUmAqn6URpWyjVDuGRspvKqyBK5gy4yBUGGVokp8Jbpk1qSTnLtTyz7CEN9NXp9CQVVy1h9ivAAYuyjqa1bv6Fkw4bhYdwuBfRWS"),
		base58.Decode("KBBLJEvxr2J7tsvfwVH3ZPszBofCmhiQ8jaxyUW7ZHLXKBwJuVTQeQPU3feBFcc47B1VMvBPFZkd1d5wTWuD5oYFDcwULft2v"),
	},
	{
		common.BlobName(base58.Decode("GUnL66Lyv2Qs4baxPhy59kF4dsB9HWakvTjMBjNGFLT6g")),
		base58.Decode("17Jk3QJMCABypJWuAivYwMi43gN1KxPVy3qg1e4HYQFe8BCQPVm6GX8XaSnefzn94gicZzGGWtALZaC6WZYGHFvYXYsyohAXgQ7r3aLEPLsS9Uvo1xhBTuDKqWfY2QNj5wuCGsmU7qtu5qHmd23pVkqH3ayQE6KUZGvUQJMoxE9ozTnbTmuBP1wREd8CyECvuvWKTDy4bX87iEKFkf6sAXKjYhcAtKjeyDokBor7S6JZz4c6DABvTvNWHyxpMJAkgBeaNmFew8vED3nWrShBFnsHFiXvs"),
		base58.Decode("byMSNfjyr7hjDnTwBZ73taXvitDc5sqwEQEFieo1eM2cxxGNhtNWiJCHVddfhNi46D7YMyYUCPR2P3tw7Krdmz4TFH3Dmju1M"),
	},
	{
		common.BlobName(base58.Decode("GUnL66Lyv2Qs4baxPhy59kF4dsB9HWakvTjMBjNGFLT6g")),
		base58.Decode("17Jk3QJMCABypJWuAivYwMi43gN1KxPVy3qg1e4HYQFe8BCQPVm6GX8XaSnefzn94gkb7erPUy8QRYe32kgJpKmrUMtcATgBjSz53wY34FEJRFExJZSgTioAzqXe8zEziQjp2Vq5eh1GcHUY7UidzrBZjxZUqHjfY6U3XrUVD56gcdhmN5rXV5ewtjKBG93eircpfj1yKWUVmPaFTqZLei4BYyDtzLij3pzRXyYTNYdz3zi83cBVzWUaHoVTdCK4LcogPy3egZHhQ3mHePWSxGqR5LMAK"),
		base58.Decode("jxxcGQavWWu11wKzBpNEK3gQZoaJoctne3BkcuG33wkGkMY3wyToEHuFUzde81JyMeubeGcLGYN3W37eG8v7CUAZ6xQoMT3Y7"),
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

	errPanic := func(err error) {
		if err != nil {
			panic(err)
		}
	}

	static := func(data string) {
		content := []byte(data)
		hash := sha256.Sum256(content)
		n, err := common.BlobNameFromHashAndType(hash[:], blobtypes.Static)
		errPanic(err)
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
		errPanic(err)

		pr, _, err := dl.UpdateLinkData(bytes.NewBufferString(data), version)
		errPanic(err)

		buf, err := io.ReadAll(pr.GetPublicDataReader())
		errPanic(err)

		pr, err = dynamiclink.FromPublicData(dl.BlobName(), bytes.NewReader(buf))
		errPanic(err)

		elink, err := io.ReadAll(pr.GetEncryptedLinkReader())
		errPanic(err)

		dumpBlob(dl.BlobName(), buf, elink)
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
