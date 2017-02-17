package graph

import (
	"bytes"
	"io"
	"io/ioutil"
)

func beDirBlobFormatSerialize(entries beEntriesMap) (io.ReadCloser, error) {
	b := bytes.Buffer{}
	s := newBeWriter(&b)

	if entries == nil {
		s.UInt(uint64(0))
	} else {
		s.UInt(uint64(len(entries)))

		for n, de := range entries {
			s.String(n)
			s.UInt(beNodeType(de.node))
			s.String(de.bid)
			s.UInt(beKeyInfoTypeValue)
			s.String(de.key)

			// TODO: Metadata
			s.UInt(0)
			/*
			   s.UInt(uint64(len(de.Metadata)))
			   for k, v := range de.Metadata {
			       s.String(k)
			       s.String(v)
			   }
			*/
		}
	}

	if s.err != nil {
		return nil, s.err
	}

	return ioutil.NopCloser(bytes.NewReader(b.Bytes())), nil
}

func beDirBlobFormatDeserialize(rawReader io.Reader, ep *epBE) (beEntriesMap, error) {
	r := newBeReader(rawReader)

	// Get number of entries and validate them
	entriesCnt := r.UInt()
	if entriesCnt > beDirMaxEntries {
		return nil, ErrMalformedDirectoryBlob
	}
	entries := make(beEntriesMap)

	for ; entriesCnt > 0; entriesCnt-- {

		// Get the name and validate it
		name := r.String(MaxEntryNameLength)
		if _, found := entries[name]; found || name == "" {
			return nil, ErrMalformedDirectoryBlob
		}

		// Create subnode instance
		nodeType := r.UInt()
		bid := r.String(beMaxBlobNameLen)

		// KeyInfo is limited to a directly embedded key now
		keyInfoType := r.UInt()
		if keyInfoType != beKeyInfoTypeValue {
			return nil, ErrMalformedDirectoryBlob
		}
		key := r.String(beMaxKeyLen)

		node := beNewNode(nodeType, ep)
		if node == nil {
			return nil, ErrMalformedDirectoryBlob
		}

		entry := beDirNodeEntry{
			bid:             bid,
			key:             key,
			node:            node,
			unsavedEpochSet: beEpochSetEmpty,
		}
		metaCount := r.UInt()
		if metaCount > 0 {
			return nil, ErrMalformedDirectoryBlob
		}
		/*
		   if metaCount > maxMetaEntries {
		       return ErrMalformedDirectoryBlob
		   }
		   for ; metaCount > 0; metaCount-- {
		       key := r.String(maxMetaKeyLen)
		       if _, exists := entry.Metadata[key]; exists || key == "" {
		           return ErrMalformedDirectoryBlob
		       }
		       entry.Metadata[key] = r.String(maxMetaValueLen)
		   }
		*/

		entries[name] = &entry
	}

	if r.err != nil {
		return nil, r.err
	}

	return entries, nil
}
