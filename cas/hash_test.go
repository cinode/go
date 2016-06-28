package cas

import "testing"

func TestHasher(t *testing.T) {

	testData := []struct {
		n string
		d string
	}{
		{n: "sGKot5hBsd81kMupNCXHaqbhv3huEbxAFMLnpcX2hniwn", d: ""},
		{n: "sBjj4AWTNrjQVHqgWbP2XaxXz4DYH1WZMyERHxsad7b2w", d: "test"},
	}

	for _, d := range testData {

		h := newHasher()
		h.Write([]byte(d.d))
		if h.Name() != d.n {
			t.Errorf("Invalid hashed name")
		}
	}
}
