package cas

import "testing"

func TestHasher(t *testing.T) {

	testData := []struct {
		n string
		d string
	}{
		{n: "ZZ8FaUwURAkWvzbnRhTt2pWSJCYZMAELqPk9USTUJgC4", d: ""},
		{n: "Uy3RfJCyen9FrvTvpZCpnBLWJiBbkidTTHNcpo1PdYHD", d: "test"},
	}

	for _, d := range testData {

		h := newHasher()
		h.Write([]byte(d.d))
		n := h.Name()
		if n != d.n {
			t.Errorf("Invalid hashed name, got %s, expected %s", n, d.n)
		}
	}
}
