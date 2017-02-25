package graph

import (
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"testing"
)

func dumpBe(n Node) {
	bn := toBase(n)
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
	case *beDirNode:
		for i := n.ListEntries(); i.Next(); {
			child, _, _ := i.GetEntry()
			dumpBe(child)
		}
	default:
	}
}

func TestBeSimpleSerialization(t *testing.T) {
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
		ep2, _ := ep2a.(*epBE)
		errCheck(t, err, nil)
		for _, p := range paths {
			ensureIsDir(t, ep2, p)
		}
	}
}
