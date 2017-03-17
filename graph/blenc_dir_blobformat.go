package graph

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"sort"
)

var (
	errBlencToManyDirEntries      = errors.New("To many directory entries")
	errBlencEntryNameToLong       = errors.New("Entry name to long")
	errBlencIncorectEntryType     = errors.New("Incorrect entry type")
	errBlencEmptyEntryName        = errors.New("Empty entry name")
	errBlencIncorectKeyInfoType   = errors.New("Incorrect key info type")
	errBlencBlobNameToLong        = errors.New("Blob name to long")
	errBlencBlobKeyToLong         = errors.New("Blob key to long")
	errBlencDuplicatedEntry       = errors.New("Duplicated entry name")
	errBlencEmptyMetadataKey      = errors.New("Empty metadata key")
	errBlencDuplicatedMetadataKey = errors.New("Duplicated metadata key")
	errBlencUnorderedMetadataKeys = errors.New("Metadata keys are not correctly ordered")
)

func blencDirBlobFormatSerialize(entries blencEntriesMap) (io.ReadCloser, error) {
	b := bytes.Buffer{}
	s := newBlencWriter(&b)

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
		return nil, r.setErr(errBlencToManyDirEntries)
	}
	entries := make(blencEntriesMap)

	for ; entriesCnt > 0; entriesCnt-- {

		// Get the name and validate it
		name := r.String(MaxEntryNameLength, errBlencEntryNameToLong)
		if name == "" {
			return nil, r.setErr(errBlencEmptyEntryName)
		}
		if _, found := entries[name]; found {
			return nil, r.setErr(errBlencDuplicatedEntry)
		}

		// Create subnode instance
		nodeType := r.UInt()
		bid := r.String(blencMaxBlobNameLen, errBlencBlobNameToLong)

		// KeyInfo is limited to a directly embedded key now
		keyInfoType := r.UInt()
		if keyInfoType != beKeyInfoTypeValue {
			return nil, r.setErr(errBlencIncorectKeyInfoType)
		}
		key := r.String(blencMaxKeyLen, errBlencBlobKeyToLong)

		node := blencNewNode(nodeType, ep)
		if node == nil {
			return nil, r.setErr(errBlencIncorectEntryType)
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
			return nil, r.setErr(ErrTooManyMetadataKeys)
		}
		for i := metaCount; i > 0; i-- {
			key := r.String(MaxMetadataKeyLength, ErrInvalidMetadataKey)
			if key <= prevMetaKey {
				if key == "" {
					return nil, r.setErr(errBlencEmptyMetadataKey)
				}
				if key == prevMetaKey {
					return nil, r.setErr(errBlencDuplicatedMetadataKey)
				}
				return nil, r.setErr(errBlencUnorderedMetadataKeys)
			}
			prevMetaKey = key // Save for next iteration
			entry.metadata[key] = r.String(MaxMetadataValueLength, ErrInvalidMetadataValue)
		}

		entries[name] = &entry
	}

	if r.err != nil {
		return nil, r.err
	}

	return entries, nil
}
