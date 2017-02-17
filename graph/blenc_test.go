package graph

import (
	"fmt"
	"testing"
)

func TestBeSimpleSerialization(t *testing.T) {
	for _, paths := range [][][]string{
		//{{"a"}},
		{{"a", "b"}},
	} {
		fmt.Printf("-----------\n")
		ep := blencTest()
		for _, p := range paths {
			mkDir(t, ep, p)
		}
		// time.Sleep(time.Second * 10)
		fmt.Printf("Bid before sync: %s\n", ep.root.bid)
		fmt.Printf("Unsaved epoch set: %s\n", ep.root.unsavedGlobalEpochSet)
		errCheck(t, ep.sync(), nil)

		fmt.Printf("Bid after sync: %s\n", ep.root.bid)
		fmt.Printf("Unsaved epoch set: %s\n", ep.root.unsavedGlobalEpochSet)

		ep2a, err := FromBE(ep.be, ep.p)
		ep2, _ := ep2a.(*epBE)
		errCheck(t, err, nil)
		//dumpEP(ep2)
		for _, p := range paths {
			ensureIsDir(t, ep2, p)
		}
	}
}
