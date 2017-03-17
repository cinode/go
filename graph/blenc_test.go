package graph

import (
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"testing"
)

func dumpBe(n Node) {
	bn := toBlencNodeBase(n)
	fmt.Printf("Node: %s, bid: %s\n", bn, bn.bid)
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
