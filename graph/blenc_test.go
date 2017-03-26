package graph

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"testing"
)

func dumpBe(n Node) {
	bn := toBlencNodeBase(n)
	fmt.Printf("Node: %v, bid: %s\n", bn, bn.bid)
	if bn.isEmpty() {
		fmt.Printf("Empty blob")
	} else {
		blob, _ := bn.rawReader()
		blobData, _ := ioutil.ReadAll(blob)
		blob.Close()
		fmt.Printf("Blob:\n%s\n", hex.Dump(blobData))
	}
	switch n := n.(type) {
	case *blencDirNode:
		for i := n.ListEntries(); i.Next(); {
			child, _, _, _ := i.GetEntry()
			dumpBe(child)
		}
	default:
	}
}

func TestBlencSimpleSerialization(t *testing.T) {
	for _, paths := range [][][]string{
		{{"a"}},
		{{"a", "b"}},
		{{"a", "b"}, {"c", "d"}},
		{{"a", "b"}, {"a", "c"}},
		{{"a", "b", "a", "c", "d", "e", "f", "fghi", "aaaaaaaaaaaaaaaaaaaaa"}},
	} {
		ep := blencTest()
		for _, p := range paths {
			mkDir(t, ep, p)
		}
		errCheck(t, ep.sync(), nil)
		ep2a, err := FromBE(ep.be, ep.p)
		ep2, _ := ep2a.(*blencEP)
		errCheck(t, err, nil)
		for _, p := range paths {
			ensureIsDir(t, ep2, p)
		}
	}
}

func TestBlencNodeFactoryTypes(t *testing.T) {
	ep := blencTest()

	for _, ty := range []uint64{
		dirTypeBasicDir,
		dirTypeBasicFile,
	} {
		n := blencNewNode(ty, ep)
		if n == nil {
			t.Fatalf("Couldn't create node of type: %v", ty)
		}

		if blencNodeType(n) != ty {
			t.Fatalf("Generated node type: %v is not correctly detected", ty)
		}
	}
}

func TestBlencNodeFactoryErrors(t *testing.T) {
	ep := blencTest()

	for _, ty := range []uint64{
		0, 3, 100,
	} {
		n := blencNewNode(ty, ep)
		if n != nil {
			t.Fatalf("Created node of unknown type: %v", ty)
		}
	}

	mustPanic(t, func() {
		blencNodeType(nil)
	})

	mustPanic(t, func() {
		blencNodeType(&dummyNode{})
	})

}

func TestBlencMetadataLoading(t *testing.T) {
	ep := blencTest()

	root, err := ep.Root()
	errCheck(t, err, nil)

	d := mkDir(t, ep, []string{"a"})

	root.SetEntry("a", d, &MetadataChange{
		Set: MetadataMap{
			"k1": "v1",
			"k2": "v2",
			"k3": "v3",
			"k4": "v4",
		},
	})

	errCheck(t, ep.sync(), nil)

	ep2a, err := FromBE(ep.be, ep.p)
	ep2, _ := ep2a.(*blencEP)
	errCheck(t, err, nil)

	ensureIsDir(t, ep2, []string{"a"})
	root2, err := ep2.Root()
	errCheck(t, err, nil)
	ensureMetadata(t, root2, "a", []string{"a"}, MetadataMap{
		"k1": "v1",
		"k2": "v2",
		"k3": "v3",
		"k4": "v4",
	})
}

func TestMalformedBlencDirBlobs(t *testing.T) {
	ep := blencTest()
	for _, d := range []struct {
		hex string
		err error
	}{
		{ // Empty blob - no entries count value
			"", io.EOF,
		},
		{ // Empty directory - valid one`
			"00", nil,
		},
		{ // Missing directory entries
			"01", io.EOF,
		},
		{ // To many directory entries
			"FFFFFF0F", errBlencToManyDirEntries,
		},
		{ // Correct single-entry direcotry
			"0101610100010000", nil,
		},
		{ // Incorrect entry type
			"0101617F00010000", errBlencIncorectEntryType,
		},
		{ // Incorrect key info type
			"0101610100000000", errBlencIncorectKeyInfoType,
		},
		{ // Empty entry name
			"01000100000000", errBlencEmptyEntryName,
		},
		{ // Entry name to long
			"01FFFFFFFF7F", errBlencEntryNameToLong,
		},
		{ // Duplicated entry name
			"020161010001000001610100010000", errBlencDuplicatedEntry,
		},
		{ // Correct dir with two entries
			"020161010001000001620100010000", nil,
		},
		{ // Correct dir with one metadata value
			"0101610100010001016100", nil,
		},
		{ // Empty metadata key
			"01016101000100010000", errBlencEmptyMetadataKey,
		},
		{ // Duplicated metadata key
			"0101610100010002016100016100", errBlencDuplicatedMetadataKey,
		},
		{ // Correct two metadata entries
			"0101610100010002016100016200", nil,
		},
		{ // Incorrectly ordered metadata keys
			"0101610100010002016200016100", errBlencUnorderedMetadataKeys,
		},
		{ // Too many metadata keys
			"01016101000100FFFFFFFF7F", ErrTooManyMetadataKeys,
		},
		{ // Metadata key to long
			"0101610100010001FFFFFFFF7F", ErrInvalidMetadataKey,
		},
		{ // Metadata value to long
			"01016101000100010161FFFFFFFF7F", ErrInvalidMetadataValue,
		},
	} {
		b, err := hex.DecodeString(d.hex)
		errCheck(t, err, nil)
		_, err = blencDirBlobFormatDeserialize(bytes.NewReader(b), ep)
		errCheck(t, err, d.err)
	}
}
