/*
Copyright © 2023 Bartłomiej Święcki (byo)

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

	"github.com/cinode/go/pkg/blobtypes"
	"github.com/cinode/go/pkg/common"
	"github.com/cinode/go/pkg/internal/blobtypes/dynamiclink"
	"github.com/cinode/go/pkg/utilities/golang"
	"github.com/jbenet/go-base58"
)

var testBlobs = []struct {
	name     *common.BlobName
	data     []byte
	expected []byte
}{
	// Static blobs
	{
		golang.Must(common.BlobNameFromString("KDc2ijtWc9mGxb5hP29YSBgkMLH8wCWnVimpvP3M6jdAk")),
		base58.Decode("3A836b"),
		base58.Decode("3A836b"),
	},
	{
		golang.Must(common.BlobNameFromString("BG8WaXMAckEfbCuoiHpx2oMAS4zAaPqAqrgf5Q3YNzmHx")),
		base58.Decode("AXG4Ffv"),
		base58.Decode("AXG4Ffv"),
	},
	{
		golang.Must(common.BlobNameFromString("2GLoj4Bk7SvjQngCT85gxWRu2DXCCjs9XWKsSpM85Wq3Ve")),
		base58.Decode(""),
		base58.Decode(""),
	},
	{
		golang.Must(common.BlobNameFromString("251SEdnHjwyvUqX1EZnuKruta4yHMkTDed7LGoi3nUJwhx")),
		base58.Decode("1DhLfjA9ij9QFBh7J8ysnN3uvGcsNQa7vaxKEwbYEMSEXuZbgyCtUAn5M4QxmgLVnCJ6cARY5Ry2EJVXxn48D837xGxRp1M2rRnz9BHVGw2sc9Ee1DkLmsurGoKX1Evt2iuMhNQyNGh2CrsHWxdGTvZVhpHShmKRziHZEDybK4ZaJh9RvTEngYQkeHAtC3J3TW6dbpaNWBNLD6YdU5xPcaE3AUPMnk4CM1dD8XMBRQekZguNJHNZwNQCXRQodVyGLVRzi1dkTG2odnrcbZ4i3oNxyJyz"),
		base58.Decode("4wNoVjVdtJ5FKtD3ZmHW4bvTiWgZFmwmps9JEJxDdinXscjMWjjeTQo2Hzwkg6GnFp1kmNoSZR9d5hXnG4qHi6mx2KqM7SVJ"),
	},
	{
		golang.Must(common.BlobNameFromString("27vP1JG4VJNZvQJ4Zfhy3H5xKugurbh89B7rKTcStM9guB")),
		base58.Decode("1eE2wp1836WtQmEbjdavggJvFPU7dZbQQH5EBS2LwBL2rYjArM9mjvWCrAbpZDkLFx7dQ5FyejnHD1EbwofDDLa1zNmN94qws1UfhNM4KCBT4oijCfPbJHobp7h5tcZQwMZy1gA3jTQBRvem2ioNuSFwqKRwbVJs9S21QFB86XuuUggNmj6sfAsDKwvE4M5EQxSkDft3CFiUX6XUMgCJUAreBRoT32wz7ncNbFaETMscFTTjFUYYiUFuv6fQESbfDCV3rfcSmxSLbLqm2u2Pd83cnzqfH"),
		base58.Decode("8Ya88xk8C7tnYXAKJL9t1bCoBUur9dLr44SwhchsBh7UQb7TmZihVpffndCxLmhH9YjMrQj442YhiW3Hr2bBUR4vCcn6VdJLK"),
	},
	{
		golang.Must(common.BlobNameFromString("e3T1HcdDLc73NHed2SFu5XHUQx5KDwgdAYTMmmEk2Ekqm")),
		base58.Decode("1yULPpEx3gjpKNBLCEzb2oj2xRcdGfr88CztgfYfEipBGiJCijqWBEEhXTaReU6CBcbt61h2DeGoZhgAfTiEwppGkJWCJrtmkSiLiib8UhupERptC3U2j6BKDg8PLwHq113WKJWM4tr2c3WxTXTSosjk7fBhuz3GJgqdYLecBfnKMGUqw8XkBf2Lth2REAw4ccZmmYn21x1W1tFdVCe4cAzAEqc5adJC3j3prPsYvL8QSqBZE5nQcnvfGekTUqn7HDZbZvqFN3TKc8HSVK9YUQ"),
		base58.Decode("MpaLZEfQpasGN1khuvpTC6CFnJucjVmzRfZwaxJkti1uQAetXnvDL8PmrFHZkr7XX1GtKaQqB2P6M2KZjCYCTfxMZi"),
	},
}

var dynamicLinkPropagationData = []struct {
	name     *common.BlobName
	data     []byte
	expected []byte
}{
	{
		golang.Must(common.BlobNameFromString("GUnL66Lyv2Qs4baxPhy59kF4dsB9HWakvTjMBjNGFLT6g")),
		base58.Decode("17Jk3QJMCABypJWuAivYwMi43gN1KxPVy3qg1e4HYQFe8BCQPVm6GX8auaFjXhwZZQhxaHjDirXH6Ze59irpWSkBicnqigPcd6j5H9AjnPHTHRKhyLSSX5kqkVRiwSRvTojGvx6oeMqj2hyhK9LxStjtYVW7WKxoCwATgQbkUWRszH2Eff3bHND8RbknhfZDSvSmXxSR8h6tMTErcV8dGyPYUysdV6Gd9bEK8bjRs6NxhCLpQ55dvZcwEi6i7rqo2WQWhY7HMMhmKhggvLXcReaUMTByq"),
		base58.Decode("PnB1W5tcQdkzYrnvE8Z1BAsBgv9kVgdeZMp78WYxnJJKi2RDPHgx9VvzYZ1hzGhVxBetGfuxwdstH8E9oNiUQ6JDNPWYZAXE7"),
	},
	{
		golang.Must(common.BlobNameFromString("GUnL66Lyv2Qs4baxPhy59kF4dsB9HWakvTjMBjNGFLT6g")),
		base58.Decode("17Jk3QJMCABypJWuAivYwMi43gN1KxPVy3qg1e4HYQFe8BCQPVm6GX8cxHJfhdYkZjq51cCNcGKTXirXxYcaGA5XdykeezU9P6jE72kmmpLNthhndMuE9oz7p725mWqPYMbMiw4Qp54oiRWdxEvh3yKRvjRA7MFK9ZJKGY1evFGbqsaMAE715aRYvP3yNjE7FaNwkKbAn1xJm4ojF4qjtaNN5zxHRgQfdZYLgybbsYJ3TJUNMxxNPkqu2CsiieeKJpJce8U5g3HAP6jAKSiXMBcmBfGm8"),
		base58.Decode("mgbcvX3FFDqwigwuybL2misVJSLjXzZs9bumic8rFSHCD9nMqbmsTxNWnRpoVn3E2GKQaFcUdUzhMax1oiq5X9abrKYqXYMtN"),
	},
	{
		golang.Must(common.BlobNameFromString("GUnL66Lyv2Qs4baxPhy59kF4dsB9HWakvTjMBjNGFLT6g")),
		base58.Decode("17Jk3QJMCABypJWuAivYwMi43gN1KxPVy3qg1e4HYQFe8BCQPVm6GX8bbhTds8inbAV58TPXDmM19FuZEFGP1B3w9gBPoTfVQUfrhmB2A4uBrcKSxFBMNT8djhviFuunpME39ZSEZp3KS4w1jms7gKnoG237vs4vnNn4uRVF6pj5oorff4VxECGVektdbkiU2BcAUQUHbkqkcw3f3sX5Rtw5Ckv5mBzaa4zqUtLiK7eYp8Wqc5Au7mzTuXvPDpWbX85hz7EnDsuHQEoZAeFCFeWdzZSgS"),
		base58.Decode("WZpzxEiTLyv42JwAfYCTo7TckS1bLY6XmuoJWoqz8BVzYNqUSvDf58KJR6tjuEegLRYCkiprPskdP7PMFP6wazLxed8JEPAsC"),
	},
}

func TestDatasetGeneration(t *testing.T) {
	t.SkipNow()

	dumpBlob := func(name *common.BlobName, content []byte, expected []byte) {
		fmt.Printf(""+
			"	{\n"+
			"		golang.Must(common.BlobNameFromString(\"%s\")),\n"+
			"		base58.Decode(\"%s\"),\n"+
			"		base58.Decode(\"%s\"),\n"+
			"	},\n",
			name.String(),
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
