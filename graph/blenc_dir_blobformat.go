package graph

import (
	"bytes"
	"io"
	"io/ioutil"
	"sort"
)

func blencDirBlobFormatSerialize(entries blencEntriesMap) (io.ReadCloser, error) {
	b := bytes.Buffer{}
	s := newBlencWriter(&b)

	if entries == nil {
		s.UInt(uint64(0))
	} else {
		s.UInt(uint64(len(entries)))

		for n, de := range entries {
			s.String(n)
			s.UInt(blencNodeType(de.node))
			s.String(de.bid)
			s.UInt(beKeyInfoTypeValue)
			s.String(de.key)

			// Metadata, always store in sorted (utf-8 bytewise) order
			s.UInt(uint64(len(de.metadata)))
			keys := []string{}
			for k := range de.metadata {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				s.String(k)
				s.String(de.metadata[k])
			}
		}
	}

	if s.err != nil {
		return nil, s.err
	}

	return ioutil.NopCloser(bytes.NewReader(b.Bytes())), nil
}

func blencDirBlobFormatDeserialize(rawReader io.Reader, ep *blencEP) (blencEntriesMap, error) {
	r := newBlencReader(rawReader)

	// Get number of entries and validate them
	entriesCnt := r.UInt()
	if entriesCnt > blencDirMaxEntries {
		return nil, ErrMalformedDirectoryBlob
	}
	entries := make(blencEntriesMap)

	for ; entriesCnt > 0; entriesCnt-- {

		// Get the name and validate it
		name := r.String(MaxEntryNameLength)
		if _, found := entries[name]; found || name == "" {
			return nil, ErrMalformedDirectoryBlob
		}

		// Create subnode instance
		nodeType := r.UInt()
		bid := r.String(blencMaxBlobNameLen)

		// KeyInfo is limited to a directly embedded key now
		keyInfoType := r.UInt()
		if keyInfoType != beKeyInfoTypeValue {
			return nil, ErrMalformedDirectoryBlob
		}
		key := r.String(blencMaxKeyLen)

		node := blencNewNode(nodeType, ep)
		if node == nil {
			return nil, ErrMalformedDirectoryBlob
		}

		nodeBase := toBlencNodeBase(node)
		nodeBase.bid = bid
		nodeBase.key = key

		entry := blencDirNodeEntry{
			bid:             bid,
			key:             key,
			node:            node,
			metadata:        MetadataMap{},
			unsavedEpochSet: blencEpochSetEmpty,
		}

		// prevMetaKey will be used to find out if
		// items are stored in sorted order and to
		// prevent empty keys
		prevMetaKey := ""
		metaCount := r.UInt()
		if metaCount > MaxMetadataKeysInNode {
			return nil, ErrMalformedDirectoryBlob
		}
		for i := metaCount; i > 0; i-- {
			key := r.String(MaxMetadataKeyLength)
			if key <= prevMetaKey {
				return nil, ErrMalformedDirectoryBlob
			}
			prevMetaKey = key // Save for next iteration
			entry.metadata[key] = r.String(MaxMetadataValueLength)
		}

		entries[name] = &entry
	}

	if r.err != nil {
		return nil, r.err
	}

	return entries, nil
}
