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
		base58.Decode("1DhLfjA9ij9QFBh7J8ysnN3uvGcsNQa7vaxKEwbYEMSEXuZbgyCtUAn5AaHsUNyCu1TNfPmkvaMYDDU8FF7QhRTrVE7yLcR8v7WWUKmGmshs7LezBHFjQkZEd63KKcuDzpvH1NvkEuhbcx1npi6PCzuV2Dt2cvt7LPuN5vA3y5xvwDHTtWYqNvjb4Auc8bv7TL8G3rSGab2n13NvB7QDXWK3gkjKcsJrFJEMrS97k3G7dC58qf81u7nXpCvDFseauEEUk2et9Mb2Wr2ZkXtrV8ccuXun"),
		base58.Decode("4wNoVjVdtJ5FKtD3ZmHW4bvTiWgZFmwmps9JEJxDdinXscjMWjjeTQo2Hzwkg6GnFp1kmNoSZR9d5hXnG4qHi6mx2KqM7SVJ"),
	},
	{
		common.BlobName(base58.Decode("27vP1JG4VJNZvQJ4Zfhy3H5xKugurbh89B7rKTcStM9guB")),
		base58.Decode("1eE2wp1836WtQmEbjdavggJvFPU7dZbQQH5EBS2LwBL2rYjArM9mjvW9xd8FkH5Vc14YqZy9E1VABSgPxk6nKfMTMCoEHwx2ro4hstUwnTYdwK6eHzE2Dp4JTVBo97KCkV9L3zBrXPsfcwPokSjbwV2dp3HAMroGyojcWzy7fJxZdJcrKhGPch6BEbagczKqwVw4q9UwgJ7qpejcRpTHUsiQGoTwaRMZ6WewLgHf5fWgQNmRv1ZQiM5qEvnbYEQUsHB8ii8vdsfabaWx8SfdQ7LU3dUSo"),
		base58.Decode("8Ya88xk8C7tnYXAKJL9t1bCoBUur9dLr44SwhchsBh7UQb7TmZihVpffndCxLmhH9YjMrQj442YhiW3Hr2bBUR4vCcn6VdJLK"),
	},
	{
		common.BlobName(base58.Decode("e3T1HcdDLc73NHed2SFu5XHUQx5KDwgdAYTMmmEk2Ekqm")),
		base58.Decode("1yULPpEx3gjpKNBLCEzb2oj2xRcdGfr88CztgfYfEipBGiJCijqWBEEd4RaPZkgiKUXZHZ8486CFbwLxCMSnrs1iansCrZyZxS7o1kd4WU1dmLhNHTbYTSkCtrSCeNb4WTUzQuhJkDiEhVwPYq1ZQj2ibFL84RKyppzEDfSEZEFjL7zeCzoCECMUCqzv326GKCGpuy9UVueH5P5P4ZzbNtyRMwwY76a64UTxWKwpJm27iFxnNp7j5odLwPyRFvsWro7v95qZrjpqgzKk18ppRW"),
		base58.Decode("MpaLZEfQpasGN1khuvpTC6CFnJucjVmzRfZwaxJkti1uQAetXnvDL8PmrFHZkr7XX1GtKaQqB2P6M2KZjCYCTfxMZi"),
	},
}

var dynamicLinkPropagationData = []struct {
	name     common.BlobName
	data     []byte
	expected []byte
}{
	{
		common.BlobName(base58.Decode("GUnL66Lyv2Qs4baxPhy59kF4dsB9HWakvTjMBjNGFLT6g")),
		base58.Decode("17Jk3QJMCABypJWuAivYwMi43gN1KxPVy3qg1e4HYQFe8BCQPVm6GX8XaSnefzn8fYvnMhVRdd6QhsaZNyt61uQcriqTSmW4pLmxzjHh7aCZef3x8vofpgeYZ3RzYJssRFRnZtGyAK2Bcooe7wkyKS11RPVf92KwjAU5C5gdi2C9ey7kp3wRMKoGWMpXUpYgzUBRtdH3Zc4rn6cQEJ4SHmANXMEiGdmMU4dnvGhPfbfv5FRmoPchreHyEkp2ohreed9mMHSUV1ZzxBeYSr9ap15mJRi1d"),
		base58.Decode("PnB1W5tcQdkzYrnvE8Z1BAsBgv9kVgdeZMp78WYxnJJKi2RDPHgx9VvzYZ1hzGhVxBetGfuxwdstH8E9oNiUQ6JDNPWYZAXE7"),
	},
	{
		common.BlobName(base58.Decode("GUnL66Lyv2Qs4baxPhy59kF4dsB9HWakvTjMBjNGFLT6g")),
		base58.Decode("17Jk3QJMCABypJWuAivYwMi43gN1KxPVy3qg1e4HYQFe8BCQPVm6GX8XaSnefzn94gjTrXn1qfggMumzv5XMa1xKBwqeCbn9UajKUMu1SkDYWpC2jgBrYXeWagXyyUcoWvXA3AzF2FzVQdurN7tg2CvBUuZ9ebRTHu4NHVpNkHhLvPX6eaUYRtCEaeeZTYuHEkXRpjpchPxARQEmDFc4pXcnkvjb7oRR8WrG2ntk7ii5UBdXuPNnX7ph2Pvu7S6p3rZzN5Jr7eRwbFPX6qipYWXbK3T7x"),
		base58.Decode("mgbcvX3FFDqwigwuybL2misVJSLjXzZs9bumic8rFSHCD9nMqbmsTxNWnRpoVn3E2GKQaFcUdUzhMax1oiq5X9abrKYqXYMtN"),
	},
	{
		common.BlobName(base58.Decode("GUnL66Lyv2Qs4baxPhy59kF4dsB9HWakvTjMBjNGFLT6g")),
		base58.Decode("17Jk3QJMCABypJWuAivYwMi43gN1KxPVy3qg1e4HYQFe8BCQPVm6GX8XaSnefzn94gh1rZ6oP2dKrA26wyKGJmLqjiV3Z8wkusBAjA4ozjwCCWbmJzeCD9fAW9NKuwBVgLmbGKNLwRKSG5skfnREarmBahTD9iq9eZKaNkQDFr7wxepabtNZxqq3CxqzoJqsnVFcaHj4iLT42Yee8pkv1xsuPWkBVwHQZFqzcKpi5Zi3VGzHoumjsLeU6CmNCP6ftXdYmsJK7rpSsibWbBmS3WB5c2rVp"),
		base58.Decode("WZpzxEiTLyv42JwAfYCTo7TckS1bLY6XmuoJWoqz8BVzYNqUSvDf58KJR6tjuEegLRYCkiprPskdP7PMFP6wazLxed8JEPAsC"),
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
