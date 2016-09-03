package blenc

import (
	"testing"

	"github.com/cinode/go/datastore"
)

func allBE(f func(be BE, kg KeyDataGenerator)) {
	func() {
		allKG(func(kg KeyDataGenerator) {
			f(FromDatastore(datastore.InMemory()), kg)
		})
	}()
}

func TestNewBE(t *testing.T) {
	testData1 := []byte("data1")
	testData2 := []byte("data2")

	allBE(func(be BE, kg KeyDataGenerator) {
		d1n, d1k, err := be.Save(bReader(testData1, nil, nil, nil), kg)
		errPanic(err)

		d1n2, d1k2, err := be.Save(bReader(testData1, nil, nil, nil), kg)
		errPanic(err)

		if kg.IsDeterministic() {
			if d1n != d1n2 {
				t.Fatal("Saving identical blobs with deterministic KG produced different blob names")
			}
			if d1k != d1k2 {
				t.Fatal("Saving identical blobs with deterministic KG produced different keys")
			}
		} else {
			if d1n == d1n2 {
				t.Fatal("Saving identical blobs with non-deterministic KG produced identical blob names")
			}
			if d1k == d1k2 {
				t.Fatal("Saving identical blobs with non-deterministic KG produced identical keys")
			}
		}

		d2n, _, err := be.Save(bReader(testData2, nil, nil, nil), kg)
		errPanic(err)
		if d2n == d1n || d2n == d1n2 {
			t.Fatal("Same data blob name for different contents")
		}
	})
}
