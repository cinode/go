package graph

import "testing"

func TestMetadataMapChange(t *testing.T) {
	for name, d := range map[string]struct {
		orig   MetadataMap
		change *MetadataChange
		result MetadataMap
	}{
		"nilChaneOnEmpty": {
			MetadataMap{},
			nil,
			MetadataMap{},
		},
		"nilChangeOnMapWithEntries": {
			MetadataMap{"k1": "v1", "k2": "v2"},
			nil,
			MetadataMap{},
		},
		"ClearAndNewKeyOnMapWithEntries": {
			MetadataMap{"k1": "v1", "k2": "v2"},
			&MetadataChange{
				Set: MetadataMap{"k3": "v3", "k4": "v4"},
			},
			MetadataMap{"k3": "v3", "k4": "v4"},
		},
		"DontClearAndNewKeyOnMapWithEntries": {
			MetadataMap{"k1": "v1", "k2": "v2"},
			&MetadataChange{
				KeepOld: true,
				Set:     MetadataMap{"k3": "v3", "k4": "v4"},
			},
			MetadataMap{"k1": "v1", "k2": "v2", "k3": "v3", "k4": "v4"},
		},
		"DontClearAndClearEntriesAndNewKeyOnMapWithEntries": {
			MetadataMap{"k1": "v1", "k2": "v2"},
			&MetadataChange{
				KeepOld: true,
				Clear:   []string{"k2", "k3"},
				Set:     MetadataMap{"k3": "v3", "k4": "v4"},
			},
			MetadataMap{"k1": "v1", "k3": "v3", "k4": "v4"},
		},
	} {
		applied, err := d.change.apply(d.orig)
		errCheck(t, err, nil)
		for k, v := range applied {
			v2, ok := d.result[k]
			if !ok {
				t.Fatalf("Failed test %s: extra key '%s' with value '%s' found",
					name, k, v,
				)
			}
			if v != v2 {
				t.Fatalf("Failed test %s: key '%s' has incorrect value '%s', should be '%s'",
					name, k, v, v2,
				)
			}
		}
		for k := range d.result {
			if _, ok := applied[k]; !ok {
				t.Fatalf("Failed test %s: key '%s' missing",
					name, k,
				)
			}
		}
	}
}
