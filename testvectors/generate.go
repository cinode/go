/*
Copyright © 2023 Bartłomiej Święcki (byo)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// The generator application creates test vectors for dynamic data
//
// Those vectors contain both valid and invalid datasets.
//
// Vectors are created independently from the main code to ensure
// a bug in the main implementation does not result in false positives
// or negatives caused by propagation of the bug into the test
// vectors generation. Similarly, whenever the generation method
// is incorrect, it is more probable to find it with an independent
// implementation.
package main

import (
	"bytes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/chacha20"
)

func main() {
	generateTestVectorsForDynamicLinks()
}

type TestCase struct {
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	Details          string   `json:"-"`
	DetailsLines     []string `json:"details,omitempty"`
	WhenAdded        string   `json:"added_at"`
	BlobName         []byte   `json:"blob_name"`
	EncryptionKey    []byte   `json:"encryption_key"`
	UpdateDataset    []byte   `json:"update_dataset"`
	DecryptedDataset []byte   `json:"decrypted_dataset"`
	ValidPublicly    bool     `json:"valid_publicly"`
	ValidPrivately   bool     `json:"valid_privately"`
	GoErrorContains  string   `json:"go_error_contains,omitempty"`
}

type gp struct {
	reservedByte   *byte
	privKey        *ed25519.PrivateKey
	nonce          *uint64
	contentVersion *uint64

	signatureHashPrefix        *byte
	signatureBlobName          func([]byte) []byte
	signatureContentVersion    func(uint64) uint64
	signatureEncryptedLinkData func([]byte) []byte
	signaturePostProcess       func([]byte) []byte

	keyGenSignatureDataPrefix *byte
	keyGenSignatureBlobName   func([]byte) []byte
	keyGenSignature           func([]byte) []byte
	keyGenHashPrefix          *byte
	keyGenHashEncryptionAlg   *byte
	keyGenHashBlobType        *byte

	keyType  *byte
	keyBytes func([]byte) []byte

	keyValidationBlockSignatureDataPrefix *byte
	keyValidationBlockSignatureBlobName   func([]byte) []byte
	keyValidationBlockPrefix              *byte
	keyValidationSignature                func([]byte) []byte
	keyValidationBlockLength              func(byte) byte

	ivGenHashPrefix     *byte
	ivGenEncryptionAlg  *byte
	ivGenBlobType       *byte
	ivGenBlobNameLength func(byte) byte
	ivGenBlobName       func([]byte) []byte
	ivGenContentVersion func(ver uint64) uint64
	ivGenLinkData       func([]byte) []byte
	ivCorrupt           func([]byte) []byte

	linkData *[]byte

	truncateAt int
}

func def[T any](b **T, v T) {
	if *b == nil {
		*b = &v
	}
}

func uint64p(v uint64) *uint64 { return &v }
func bytep(b byte) *byte       { return &b }

func defF[T any](b *func(T) T) {
	cp := func(val any) any {
		if val, ok := val.([]byte); ok {
			return append([]byte{}, val...)
		}
		return val
	}

	if *b == nil {
		*b = func(t T) T { return t }
	} else {
		orig := *b
		*b = func(t T) T {
			return orig(cp(t).(T))
		}
	}
}

func genAll(gp gp) ([]byte, []byte, []byte, []byte) {

	// Set default parameters
	def(&gp.reservedByte, 0)
	def(&gp.privKey, ed25519.NewKeyFromSeed(seed1[:ed25519.SeedSize]))
	def(&gp.nonce, binary.BigEndian.Uint64(seed2[:]))
	def(&gp.contentVersion, binary.BigEndian.Uint64(seed2[8:]))
	def(&gp.signatureHashPrefix, 0)
	defF(&gp.signatureBlobName)
	defF(&gp.signatureContentVersion)
	defF(&gp.signatureEncryptedLinkData)
	defF(&gp.signaturePostProcess)
	def(&gp.keyGenSignatureDataPrefix, 0x01)
	defF(&gp.keyGenSignatureBlobName)
	defF(&gp.keyGenSignature)
	def(&gp.keyGenHashPrefix, 0x01)        // Hasher for key
	def(&gp.keyGenHashEncryptionAlg, 0x00) // Hasher for chacha20
	def(&gp.keyGenHashBlobType, 0x02)      // Dynamic link blob type
	def(&gp.keyType, 0x00)                 // chacha20 key type
	defF(&gp.keyBytes)
	def(&gp.keyValidationBlockSignatureDataPrefix, 0x01)
	defF(&gp.keyValidationBlockSignatureBlobName)
	def(&gp.keyValidationBlockPrefix, 0x00)
	defF(&gp.keyValidationSignature)
	defF(&gp.keyValidationBlockLength)
	def(&gp.ivGenHashPrefix, 0x02)    // Hasher for iv
	def(&gp.ivGenEncryptionAlg, 0x00) // Hasher for chacha20
	def(&gp.ivGenBlobType, 0x02)      // Dynamic link blob type
	defF(&gp.ivGenBlobNameLength)
	defF(&gp.ivGenBlobName)
	defF(&gp.ivGenContentVersion)
	defF(&gp.ivGenLinkData)
	defF(&gp.ivCorrupt)

	def(&gp.linkData, seed4[:])

	// Start link data creation - it begins with unchanging link data
	buff := []byte{}
	buff = append(buff, *gp.reservedByte)
	buff = append(buff, gp.privKey.Public().(ed25519.PublicKey)...)
	buff = append(buff, 0, 0, 0, 0, 0, 0, 0, 0)
	binary.BigEndian.PutUint64(buff[len(buff)-8:], *gp.nonce)

	hash := sha256.Sum256(buff)

	blobName := [sha256.Size + 1]byte{0x02}
	copy(blobName[1:], hash[:])

	for i := 1; i <= sha256.Size; i++ {
		blobName[0] ^= blobName[i]
	}

	keygenToSignBytes := append(
		[]byte{*gp.keyGenSignatureDataPrefix},
		gp.keyGenSignatureBlobName(blobName[:])...,
	)

	keyGenHasher := sha256.New()
	keyGenHasher.Write([]byte{
		*gp.keyGenHashPrefix,
		*gp.keyGenHashEncryptionAlg,
		*gp.keyGenHashBlobType,
	})
	keyGenHasher.Write(gp.keyGenSignature(ed25519.Sign(*gp.privKey, keygenToSignBytes)))

	key := append(
		[]byte{*gp.keyType},
		gp.keyBytes(keyGenHasher.Sum(nil)[:chacha20.KeySize])...,
	)

	keyValidationBlockKeygenToSignBytes := append(
		[]byte{*gp.keyValidationBlockSignatureDataPrefix},
		gp.keyValidationBlockSignatureBlobName(blobName[:])...,
	)

	keyValidationBlock := append(
		[]byte{*gp.keyValidationBlockPrefix},
		gp.keyValidationSignature(ed25519.Sign(*gp.privKey, keyValidationBlockKeygenToSignBytes))...,
	)

	unencryptedDataBuff := append([]byte{
		gp.keyValidationBlockLength(
			byte(len(keyValidationBlock)),
		),
	},
		keyValidationBlock...,
	)
	unencryptedDataBuff = append(unencryptedDataBuff, *gp.linkData...)

	ivGenHasher := sha256.New()
	ivGenHasher.Write([]byte{
		*gp.ivGenHashPrefix,
		*gp.ivGenEncryptionAlg,
		*gp.ivGenBlobType,
		gp.ivGenBlobNameLength(byte(len(blobName))),
	})
	ivGenHasher.Write(gp.ivGenBlobName(blobName[:]))

	var verBuff [8]byte
	binary.BigEndian.PutUint64(verBuff[:], gp.ivGenContentVersion(*gp.contentVersion))
	ivGenHasher.Write(verBuff[:])
	ivGenHasher.Write(gp.ivGenLinkData(unencryptedDataBuff))

	iv := gp.ivCorrupt(ivGenHasher.Sum(nil)[:chacha20.NonceSizeX])

	ccc, err := chacha20.NewUnauthenticatedCipher(key[1:], iv)
	if err != nil {
		panic(err)
	}
	encryptedLinkDataBuff := bytes.NewBuffer(nil)
	cipher.StreamWriter{
		S: ccc,
		W: encryptedLinkDataBuff,
	}.Write(unencryptedDataBuff)

	encryptedLinkData := encryptedLinkDataBuff.Bytes()

	toSignHasher := sha256.New()
	toSignHasher.Write([]byte{*gp.signatureHashPrefix})
	bn := gp.signatureBlobName(blobName[:])
	toSignHasher.Write([]byte{byte(len(bn))})
	toSignHasher.Write(bn)
	binary.BigEndian.PutUint64(verBuff[:], gp.signatureContentVersion(*gp.contentVersion))
	toSignHasher.Write(verBuff[:])
	toSignHasher.Write(gp.signatureEncryptedLinkData(encryptedLinkData))

	signature := ed25519.Sign(*gp.privKey, toSignHasher.Sum(nil))
	signature = gp.signaturePostProcess(signature)

	// Add changing data to the link buffer
	buff = append(buff, 0, 0, 0, 0, 0, 0, 0, 0)
	binary.BigEndian.PutUint64(buff[len(buff)-8:], *gp.contentVersion)
	buff = append(buff, signature...)
	buff = append(buff, byte(len(iv)))
	buff = append(buff, iv...)
	buff = append(buff, encryptedLinkData...)

	if gp.truncateAt > 0 {
		buff = buff[:gp.truncateAt]
	}

	return buff, blobName[:], key, *gp.linkData
}

func genLink(gp gp) []byte {
	link, _, _, _ := genAll(gp)
	return link
}

func blobName(gp gp) []byte {
	_, blobName, _, _ := genAll(gp)
	return blobName
}

func key(gp gp) []byte {
	_, _, key, _ := genAll(gp)
	return key
}

func decrypted(gp gp) []byte {
	_, _, _, decrypted := genAll(gp)
	return decrypted
}

var (
	seed1 = sha256.Sum256([]byte("cinode test vectors data seed - 1"))
	seed2 = sha256.Sum256([]byte("cinode test vectors data seed - 2"))
	seed3 = sha256.Sum256([]byte("cinode test vectors data seed - 3"))
	seed4 = sha256.Sum256([]byte("cinode test vectors data seed - 4"))

	privKey2 = ed25519.NewKeyFromSeed(seed3[:ed25519.SeedSize])
)

func writeLinkData(tc TestCase) error {
	fName := tc.Name + ".json"
	err := os.MkdirAll(filepath.Dir(fName), 0777)
	if err != nil {
		return err
	}

	l := strings.Split(tc.Details, "\n")
	for i := range l {
		l[i] = strings.TrimSpace(l[i])
	}
	for len(l) > 0 && l[0] == "" {
		l = l[1:]
	}
	for len(l) > 0 && l[len(l)-1] == "" {
		l = l[:len(l)-1]
	}
	tc.DetailsLines = l

	jsonData, err := json.MarshalIndent(&tc, "", "   ")
	if err != nil {
		return err
	}

	err = os.WriteFile(fName, jsonData, 0666)
	if err != nil {
		return err
	}

	return nil
}

func generateTestVectorsForDynamicLinks() {

	// Completely valid links

	for i := 0; i < 10; i++ {
		linkData := []byte(fmt.Sprintf("Link data %02d", i))
		gp := gp{
			linkData: &linkData,
		}
		if i == 0 {
			// Have the first blob with default dataset
			gp.linkData = nil
		}
		writeLinkData(TestCase{
			Description:      fmt.Sprintf("Correct link - %02d", i),
			Name:             fmt.Sprintf("dynamic/correct/%03d_correct_link", i),
			WhenAdded:        "2023-01-20",
			UpdateDataset:    genLink(gp),
			BlobName:         blobName(gp),
			EncryptionKey:    key(gp),
			DecryptedDataset: decrypted(gp),
			ValidPublicly:    true,
			ValidPrivately:   true,
		})
	}

	// Incorrect blobs on the public layer, those should be rejected by the network
	// automatically, any failure to reject those can be exploited

	writeLinkData(TestCase{
		Details: `
			Empty dataset is an invalid blob.
		`,
		Description:     "Empty dataset",
		Name:            "dynamic/attacks/public/001_empty",
		WhenAdded:       "2023-01-20",
		UpdateDataset:   []byte{},
		BlobName:        blobName(gp{}),
		EncryptionKey:   key(gp{}),
		GoErrorContains: "data truncated while reading reserved byte",
	})

	writeLinkData(TestCase{
		Details: `
			The first byte in the dataset is reserved for future protocol
			modifications without breaking backwards compatibility.
			Currently the reserved byte must be zero, any byte other than
			that must be rejected.
		`,
		Description: "Invalid reserved byte",
		Name:        "dynamic/attacks/public/002_reserved_byte",
		WhenAdded:   "2023-01-20",
		UpdateDataset: genLink(gp{
			reservedByte: bytep(0xFF),
		}),
		BlobName:        blobName(gp{}),
		EncryptionKey:   key(gp{}),
		GoErrorContains: "invalid value of the reserved byte",
	})

	writeLinkData(TestCase{
		Description: "Truncated public key",
		Name:        "dynamic/attacks/public/003_pubkey_truncated",
		WhenAdded:   "2023-01-20",
		UpdateDataset: genLink(gp{
			truncateAt: 1 + ed25519.PublicKeySize/2,
		}),
		BlobName:        blobName(gp{}),
		EncryptionKey:   key(gp{}),
		GoErrorContains: "data truncated while reading public key",
	})

	writeLinkData(TestCase{
		Description: "Truncated nonce",
		Name:        "dynamic/attacks/public/004_nonce_truncated",
		WhenAdded:   "2023-01-20",
		UpdateDataset: genLink(gp{
			truncateAt: 1 + ed25519.PublicKeySize + 4,
		}),
		BlobName:        blobName(gp{}),
		EncryptionKey:   key(gp{}),
		GoErrorContains: "data truncated while reading nonce",
	})

	writeLinkData(TestCase{
		Details: `
			Blob name is built based on unchanging data in the dynamic link
			this test blob ensures that the reserved byte value must remain the same
			to keep the same blob name
		`,
		Description:   "Blob name mismatch for reserved byte",
		Name:          "dynamic/attacks/public/005_blob_mismatch_reserved_byte",
		WhenAdded:     "2023-01-20",
		UpdateDataset: genLink(gp{}),
		BlobName: blobName(gp{
			reservedByte: bytep(0xFF),
		}),
		EncryptionKey:   key(gp{}),
		GoErrorContains: "blob name mismatch",
	})

	writeLinkData(TestCase{
		Details: `
			Blob name is built based on unchanging data in the dynamic link
			this test blob ensures that the public key must remain the same
			to keep the same blob name
		`,
		Description:   "Blob name mismatch for key",
		Name:          "dynamic/attacks/public/006_blob_mismatch_priv_key",
		WhenAdded:     "2023-01-20",
		UpdateDataset: genLink(gp{}),
		BlobName: blobName(gp{
			privKey: &privKey2,
		}),
		EncryptionKey:   key(gp{}),
		GoErrorContains: "blob name mismatch",
	})

	writeLinkData(TestCase{
		Details: `
			Blob name is built based on unchanging data in the dynamic link
			this test blob ensures that the link nonce must remain the same
			to keep the same blob name
		`,
		Description:   "Blob name mismatch for nonce",
		Name:          "dynamic/attacks/public/007_blob_mismatch_nonce",
		WhenAdded:     "2023-01-20",
		UpdateDataset: genLink(gp{}),
		BlobName: blobName(gp{
			nonce: uint64p(12345),
		}),
		EncryptionKey:   key(gp{}),
		GoErrorContains: "blob name mismatch",
	})

	writeLinkData(TestCase{
		Description: "Truncated content version",
		Name:        "dynamic/attacks/public/008_truncated_content_version",
		WhenAdded:   "2023-01-20",
		UpdateDataset: genLink(gp{
			truncateAt: 1 + ed25519.PublicKeySize + 8 + 4,
		}),
		BlobName:        blobName(gp{}),
		EncryptionKey:   key(gp{}),
		GoErrorContains: "data truncated while reading content version",
	})

	writeLinkData(TestCase{
		Description: "Truncated signature",
		Name:        "dynamic/attacks/public/009_truncated_signature",
		WhenAdded:   "2023-01-20",
		UpdateDataset: genLink(gp{
			truncateAt: 1 + ed25519.PublicKeySize + 8 + 8 + ed25519.SignatureSize/2,
		}),
		BlobName:        blobName(gp{}),
		EncryptionKey:   key(gp{}),
		GoErrorContains: "data truncated while reading signature",
	})

	writeLinkData(TestCase{
		Description: "Truncated iv",
		Name:        "dynamic/attacks/public/010_truncated_iv",
		WhenAdded:   "2023-01-20",
		UpdateDataset: genLink(gp{
			truncateAt: 1 + ed25519.PublicKeySize + 8 + 8 + ed25519.SignatureSize + chacha20.NonceSizeX/2,
		}),
		BlobName:        blobName(gp{}),
		EncryptionKey:   key(gp{}),
		GoErrorContains: "data truncated while reading iv",
	})

	writeLinkData(TestCase{
		Details: `
			Dynamic link signature is protecting both unchanging and changing blob
			data. Also to enable signatures for different independent data sources,
			those are using additional prefixes to avoid signature collisions that could
			be exploited by an attacker (e.g. by causing reveal of signature for
			data in one source that has the same byte sequence as in other source).
			This test ensures that the signature won't match if the data source prefix
			is different.
		`,
		Description: "Signature mismatch - prefix byte",
		Name:        "dynamic/attacks/public/011_signature_mismatch_prefix",
		WhenAdded:   "2023-01-20",
		UpdateDataset: genLink(gp{
			signatureHashPrefix: bytep(0xFF),
		}),
		BlobName:        blobName(gp{}),
		EncryptionKey:   key(gp{}),
		GoErrorContains: "signature mismatch",
	})

	writeLinkData(TestCase{
		Details: `
			Dynamic link signature is protecting both unchanging and changing blob
			data. The same public/private key pair can be used for different blob names.
			To ensure that the signature for different blob names is different,
			the blob name itself is included in the signed dataset.

			This prevents an attack where targeted person uses the same private key
			but different nonces to sign data of different security level. If the same
			signature would be created for the same dataset in different blob names,
			attacker could use this to trick the victim to sign some less important
			information and reuse it to replace other blob name that is of a high
			importance.
		`,
		Description: "Signature mismatch - blob name",
		Name:        "dynamic/attacks/public/012_signature_mismatch_blob_name",
		WhenAdded:   "2023-01-20",
		UpdateDataset: genLink(gp{
			signatureBlobName: func(b []byte) []byte {
				return blobName(gp{privKey: &privKey2})
			},
		}),
		BlobName:        blobName(gp{}),
		EncryptionKey:   key(gp{}),
		GoErrorContains: "signature mismatch",
	})

	writeLinkData(TestCase{
		Details: `
			Dynamic link signature is protecting both unchanging and changing blob
			data. To enable forward progress, the link with higher content version
			does replace the link with the lower one. The signature thus has to
			include the content version so that the attacker can not reuse an older
			version of the link to replace newer one by bumping the content version.
		`,
		Description: "Signature mismatch - content version",
		Name:        "dynamic/attacks/public/013_signature_mismatch_content_version",
		WhenAdded:   "2023-01-20",
		UpdateDataset: genLink(gp{
			signatureContentVersion: func(u uint64) uint64 { return u + 1 },
		}),
		BlobName:        blobName(gp{}),
		EncryptionKey:   key(gp{}),
		GoErrorContains: "signature mismatch",
	})

	writeLinkData(TestCase{
		Details: `
			Dynamic link signature is protecting both unchanging and changing blob
			data. The signature must protect the main encrypted link data.
			The signature has to be calculated over the encrypted data so that the
			network can detect invalid signatures without revealing the unencrypted
			information.
		`,
		Description: "Signature mismatch - encrypted link data",
		Name:        "dynamic/attacks/public/014_signature_mismatch_encrypted_link_data",
		WhenAdded:   "2023-01-20",
		UpdateDataset: genLink(gp{
			signatureEncryptedLinkData: func(b []byte) []byte {
				// Modify single bit in the encrypted link data
				b[len(b)/2] ^= 0x40
				return b
			},
		}),
		BlobName:        blobName(gp{}),
		EncryptionKey:   key(gp{}),
		GoErrorContains: "signature mismatch",
	})

	writeLinkData(TestCase{
		Description: "Signature mismatch - corrupted signature bytes",
		Name:        "dynamic/attacks/public/015_signature_mismatch_signature_bytes",
		WhenAdded:   "2023-01-20",
		UpdateDataset: genLink(gp{
			signaturePostProcess: func(b []byte) []byte {
				// Change single bit in the middle of the signature
				b[len(b)/2] ^= 0x80
				return b
			},
		}),
		BlobName:        blobName(gp{}),
		EncryptionKey:   key(gp{}),
		GoErrorContains: "signature mismatch",
	})

	// Public data is valid but private data is incorrect,
	// those blobs will only fail when trying to read the data but will be successfully
	// propagated through the network when validating public dataset only.
	//
	// If validation fails at the private level, this means that the creator of the data
	// is compromised. This could happen for various reasons:
	//  * Creator uses incorrect software producing invalid blogs
	//  * Creator was hacked and someone is disrupting the link creation process
	//  * Creator is a bad actor and tries to disrupt the network in some way

	writeLinkData(TestCase{
		Details: `
			Data link encryption key is deterministically calculated from link's configuration.
			It must be calculate based on unchanging link dataset and private key.

			When generating the encryption key, private key is used to calculate a signature
			of blob's unchanging data. That signature must be different to the signature
			used to sign the encrypted data itself thus a different prefix is used for each
			signing schemes.

			This test ensures that an invalid prefix determining the kind of data being signed
			results in encryption key validation failure.
		`,
		Description: "Invalid encryption key - keygen signature data prefix",
		Name:        "dynamic/attacks/private/001_keygen_signature_prefix",
		WhenAdded:   "2023-01-20",
		UpdateDataset: genLink(gp{
			keyValidationBlockSignatureDataPrefix: bytep(0x77),
		}),
		BlobName:        blobName(gp{}),
		EncryptionKey:   key(gp{}),
		ValidPublicly:   true,
		GoErrorContains: "invalid key validation block signature",
	})

	writeLinkData(TestCase{
		Details: `
			Data link encryption key is deterministically calculated from link's configuration.
			It must be calculate based on unchanging link dataset and private key.

			This test ensures that an invalid blob name used in key generation signature will
			result in rejection of the encryption key.
		`,
		Description: "Invalid encryption key - keygen signature blob name",
		Name:        "dynamic/attacks/private/002_keygen_signature_blob_name",
		WhenAdded:   "2023-01-21",
		UpdateDataset: genLink(gp{
			keyValidationBlockSignatureBlobName: func(b []byte) []byte {
				b[len(b)/2] ^= 0x20
				return b
			},
		}),
		BlobName:        blobName(gp{}),
		EncryptionKey:   key(gp{}),
		ValidPublicly:   true,
		GoErrorContains: "invalid key validation block signature",
	})

	writeLinkData(TestCase{
		Details: `
			Data link encryption key is deterministically calculated from link's configuration.
			It must be calculate based on unchanging link dataset and private key.

			This test ensures that any corruption in the base signature will result in rejection
			of the encryption key.
		`,
		Description: "Invalid encryption key - keygen signature corruption",
		Name:        "dynamic/attacks/private/003_keygen_signature_corruption",
		WhenAdded:   "2023-01-21",
		UpdateDataset: genLink(gp{
			keyValidationBlockSignatureBlobName: func(b []byte) []byte {
				b[len(b)/2] ^= 0x20
				return b
			},
		}),
		BlobName:        blobName(gp{}),
		EncryptionKey:   key(gp{}),
		ValidPublicly:   true,
		GoErrorContains: "invalid key validation block signature",
	})

	writeLinkData(TestCase{
		Details: `
			Data link encryption key is calculated by using a hash function.
			To avoid potential security issue where the same hash function with
			the same input data is reused in different context revealing such hash publicly,
			each kind of data uses different prefix for the data being hashed mitigating the risk.

			This test ensures that invalid hashed data prefix results with an invalid key that is
			rejected while trying to read the plaintext link data.
		`,
		Description: "Invalid encryption key - keygen hash prefix",
		Name:        "dynamic/attacks/private/004_keygen_hash_prefix",
		WhenAdded:   "2023-01-21",
		UpdateDataset: genLink(gp{
			keyGenHashPrefix: bytep(0xFE),
		}),
		BlobName: blobName(gp{}),
		EncryptionKey: key(gp{
			keyGenHashPrefix: bytep(0xFE),
		}),
		ValidPublicly:   true,
		GoErrorContains: "key mismatch",
	})

	writeLinkData(TestCase{
		Details: `
			Data link encryption key is calculated by using a hash function.
			To avoid potential security issue where the same hash function with
			the same input data is reused for different encryption algorithms,
			there's an encoded encryption algorithm info before the final data to be hashed.

			This test ensures that invalid encryption algorithm information used in the hashed
			data results with an invalid key that is rejected while trying to read the plaintext link data.
		`,
		Description: "Invalid encryption key - keygen hash encryption alg",
		Name:        "dynamic/attacks/private/005_keygen_hash_encryption_alg",
		WhenAdded:   "2023-01-21",
		UpdateDataset: genLink(gp{
			keyGenHashEncryptionAlg: bytep(0xFD),
		}),
		BlobName: blobName(gp{}),
		EncryptionKey: key(gp{
			keyGenHashEncryptionAlg: bytep(0xFD),
		}),
		ValidPublicly:   true,
		GoErrorContains: "key mismatch",
	})

	writeLinkData(TestCase{
		Details: `
			Data link encryption key is calculated by using a hash function.
			To avoid potential security issue where the same hash function with
			the same input data is reused for different blob type, there's an encoded
			blob type information before the final data to be hashed.

			This test ensures that invalid blob type used in the hashed data results
			with an invalid key that is rejected while trying to read the plaintext link data.
		`,
		Description: "Invalid encryption key - keygen hash blob type",
		Name:        "dynamic/attacks/private/006_keygen_hash_blob_type",
		WhenAdded:   "2023-01-21",
		UpdateDataset: genLink(gp{
			keyGenHashBlobType: bytep(0xFC),
		}),
		BlobName: blobName(gp{}),
		EncryptionKey: key(gp{
			keyGenHashBlobType: bytep(0xFC),
		}),
		ValidPublicly:   true,
		GoErrorContains: "key mismatch",
	})

	writeLinkData(TestCase{
		Details: `
			Encryption key contains encryption algorithm type information encoded before
			key bytes. This test ensures that invalid encryption algorithm is rejected.
		`,
		Description: "Invalid encryption key - keygen type",
		Name:        "dynamic/attacks/private/007_key_type",
		WhenAdded:   "2023-01-21",
		UpdateDataset: genLink(gp{
			keyType: bytep(0xFB),
		}),
		BlobName: blobName(gp{}),
		EncryptionKey: key(gp{
			keyType: bytep(0xFB),
		}),
		ValidPublicly:   true,
		GoErrorContains: "wrong key type",
	})

	writeLinkData(TestCase{
		Details: `
			Encryption key contains encryption algorithm type information encoded before
			key bytes. This test ensures that invalid encryption algorithm is rejected.
		`,
		Description: "Invalid encryption key - keygen corruption",
		Name:        "dynamic/attacks/private/008_key_corruption",
		WhenAdded:   "2023-01-21",
		UpdateDataset: genLink(gp{
			keyBytes: func(b []byte) []byte {
				b[len(b)/2] ^= 0x10
				return b
			},
		}),
		BlobName:      blobName(gp{}),
		EncryptionKey: key(gp{}),
		ValidPublicly: true,
		// Encryption key is invalid, we most likely get garbage there,
		// since key validation block size is at the beginning, it will contain the wrong value
		GoErrorContains: "block size",
	})

	writeLinkData(TestCase{
		Details: `
			For client validation purposes, unencrypted data contains key validation block
			next to the link information itself. That data contains prefix so that it can be
			modified in the future supporting different validation block formats.

			This test checks if invalid prefix value makes the blob invalid.
		`,
		Description: "Invalid key validation block - prefix",
		Name:        "dynamic/attacks/private/009_key_validation_block_prefix",
		WhenAdded:   "2023-01-21",
		UpdateDataset: genLink(gp{
			keyValidationBlockPrefix: bytep(0xFA),
		}),
		BlobName:        blobName(gp{}),
		EncryptionKey:   key(gp{}),
		ValidPublicly:   true,
		GoErrorContains: "invalid key validation block reserved byte",
	})

	writeLinkData(TestCase{
		Details: `
			For client validation purposes, unencrypted data contains key validation block
			next to the link information itself. That validation block contains the signature
			used converted later to an encryption key with a hash function.
		
			This test checks if an invalid signature stored in key validation block
			will end up with the link being rejected.
		`,
		Description: "Invalid key validation block - signature",
		Name:        "dynamic/attacks/private/010_key_validation_block_signature",
		WhenAdded:   "2023-01-21",
		UpdateDataset: genLink(gp{
			keyValidationSignature: func(b []byte) []byte {
				b[len(b)/2] ^= 0x08
				return b
			},
		}),
		BlobName:        blobName(gp{}),
		EncryptionKey:   key(gp{}),
		ValidPublicly:   true,
		GoErrorContains: "invalid key validation block signature",
	})

	writeLinkData(TestCase{
		Details: `
			For client validation purposes, unencrypted data contains key validation block
			next to the link information itself. The data is length prefixed and must be of
			a specific length to be accepted.
		
			This test checks whether key validation block smaller than the desired value
			will result in rejection of the encryption key.
		`,
		Description: "Invalid key validation block - length smaller",
		Name:        "dynamic/attacks/private/011_key_validation_block_length_smaller",
		WhenAdded:   "2023-01-21",
		UpdateDataset: genLink(gp{
			keyValidationBlockLength: func(b byte) byte {
				return b - 1
			},
		}),
		BlobName:        blobName(gp{}),
		EncryptionKey:   key(gp{}),
		ValidPublicly:   true,
		GoErrorContains: "invalid key validation block signature",
	})

	writeLinkData(TestCase{
		Details: `
			For client validation purposes, unencrypted data contains key validation block
			next to the link information itself. The data is length prefixed and must be of
			a specific length to be accepted.
		
			This test checks whether key validation block larger than the desired value
			will result in rejection of the encryption key.
		`,
		Description: "Invalid key validation block - length larger",
		Name:        "dynamic/attacks/private/012_key_validation_block_length_larger",
		WhenAdded:   "2023-01-21",
		UpdateDataset: genLink(gp{
			keyValidationBlockLength: func(b byte) byte {
				return b + 1
			},
		}),
		BlobName:        blobName(gp{}),
		EncryptionKey:   key(gp{}),
		ValidPublicly:   true,
		GoErrorContains: "invalid key validation block signature",
	})

	writeLinkData(TestCase{
		Details: `
			Data link encryption iv is calculated by using a hash function.
			To avoid potential security issue where the same hash function with
			the same input data is reused in different context revealing such hash publicly,
			each kind of data uses different prefix for the data being hashed mitigating the risk.

			This test ensures that invalid hashed data prefix results with an invalid iv that is
			rejected while trying to read the plaintext link data.
		`,
		Description: "Invalid encryption iv - iv gen hash prefix",
		Name:        "dynamic/attacks/private/013_iv_has_gen_h_prefix",
		WhenAdded:   "2023-01-21",
		UpdateDataset: genLink(gp{
			ivGenHashPrefix: bytep(0xFE),
		}),
		BlobName:        blobName(gp{}),
		EncryptionKey:   key(gp{}),
		ValidPublicly:   true,
		GoErrorContains: "iv mismatch",
	})

	writeLinkData(TestCase{
		Details: `
			Data link encryption iv is calculated by using a hash function.
			To avoid potential security issue where the same hash function with
			the same input data is reused for different encryption algorithms,
			there's an encoded encryption algorithm info before the final data to be hashed.

			This test ensures that invalid encryption algorithm information used in the hashed
			data results with an invalid iv that is rejected while trying to read the plaintext link data.
		`,
		Description: "Invalid encryption iv - iv gen hash encryption alg",
		Name:        "dynamic/attacks/private/014_keygen_hash_encryption_alg",
		WhenAdded:   "2023-01-21",
		UpdateDataset: genLink(gp{
			ivGenEncryptionAlg: bytep(0xFD),
		}),
		BlobName:        blobName(gp{}),
		EncryptionKey:   key(gp{}),
		ValidPublicly:   true,
		GoErrorContains: "iv mismatch",
	})

	writeLinkData(TestCase{
		Details: `
			Data link encryption iv is calculated by using a hash function.
			To avoid potential security issue where the same hash function with
			the same input data is reused for different blob type, there's an encoded
			blob type information before the final data to be hashed.

			This test ensures that invalid blob type used in the hashed data results
			with an invalid iv that is rejected while trying to read the plaintext link data.
		`,
		Description: "Invalid encryption iv - iv gen hash blob type",
		Name:        "dynamic/attacks/private/015_iv_gen_hash_blob_type",
		WhenAdded:   "2023-01-21",
		UpdateDataset: genLink(gp{
			ivGenBlobType: bytep(0xFC),
		}),
		BlobName:        blobName(gp{}),
		EncryptionKey:   key(gp{}),
		ValidPublicly:   true,
		GoErrorContains: "iv mismatch",
	})

	writeLinkData(TestCase{
		Details: `
			Data link encryption iv is calculated by using a hash function.
			The iv value is calculated from both changing and unchanging data.

			This test ensures that invalid blob name length used in the hashed data results
			with an invalid iv that is rejected while trying to read the plaintext link data.
		`,
		Description: "Invalid encryption iv - iv gen hash blob name length",
		Name:        "dynamic/attacks/private/016_iv_gen_hash_blob_name_length",
		WhenAdded:   "2023-01-21",
		UpdateDataset: genLink(gp{
			ivGenBlobNameLength: func(b byte) byte { return b + 1 },
		}),
		BlobName:        blobName(gp{}),
		EncryptionKey:   key(gp{}),
		ValidPublicly:   true,
		GoErrorContains: "iv mismatch",
	})

	writeLinkData(TestCase{
		Details: `
			Data link encryption iv is calculated by using a hash function.
			The iv value is calculated from both changing and unchanging data.

			This test ensures that invalid blob name used in the hashed data results
			with an invalid iv that is rejected while trying to read the plaintext link data.
		`,
		Description: "Invalid encryption iv - iv gen hash blob name",
		Name:        "dynamic/attacks/private/017_iv_gen_hash_blob_name",
		WhenAdded:   "2023-01-21",
		UpdateDataset: genLink(gp{
			ivGenBlobName: func(b []byte) []byte {
				b[len(b)/2] ^= 0x04
				return b
			},
		}),
		BlobName:        blobName(gp{}),
		EncryptionKey:   key(gp{}),
		ValidPublicly:   true,
		GoErrorContains: "iv mismatch",
	})

	writeLinkData(TestCase{
		Details: `
			Data link encryption iv is calculated by using a hash function.
			The iv value is calculated from both changing and unchanging data.

			This test ensures that invalid content version used in the hashed data results
			with an invalid iv that is rejected while trying to read the plaintext link data.
		`,
		Description: "Invalid encryption iv - iv gen hash content version",
		Name:        "dynamic/attacks/private/018_iv_gen_hash_content_version",
		WhenAdded:   "2023-01-21",
		UpdateDataset: genLink(gp{
			ivGenContentVersion: func(ver uint64) uint64 { return ver + 1 },
		}),
		BlobName:        blobName(gp{}),
		EncryptionKey:   key(gp{}),
		ValidPublicly:   true,
		GoErrorContains: "iv mismatch",
	})

	writeLinkData(TestCase{
		Details: `
			Data link encryption iv is calculated by using a hash function.
			The iv value is calculated from both changing and unchanging data.

			This test ensures that invalid link data used in the hashed data results
			with an invalid iv that is rejected while trying to read the plaintext link data.
		`,
		Description: "Invalid encryption iv - iv gen hash link data",
		Name:        "dynamic/attacks/private/019_iv_gen_hash_link_data",
		WhenAdded:   "2023-01-21",
		UpdateDataset: genLink(gp{
			ivGenLinkData: func(b []byte) []byte {
				b[len(b)/2] ^= 0x01
				return b
			},
		}),
		BlobName:        blobName(gp{}),
		EncryptionKey:   key(gp{}),
		ValidPublicly:   true,
		GoErrorContains: "iv mismatch",
	})

	writeLinkData(TestCase{
		Details: `
			Data link encryption iv is calculated by using a hash function.
			The iv value is calculated from both changing and unchanging data.

			This test ensures that corrupted iv is rejected while trying to read the plaintext link data.
		`,
		Description: "Invalid encryption iv - corrupted iv",
		Name:        "dynamic/attacks/private/020_corrupted_iv",
		WhenAdded:   "2023-01-21",
		UpdateDataset: genLink(gp{
			ivCorrupt: func(b []byte) []byte {
				b[len(b)/2] ^= 0x77
				return b
			},
		}),
		BlobName:        blobName(gp{}),
		EncryptionKey:   key(gp{}),
		ValidPublicly:   true,
		GoErrorContains: "iv mismatch",
	})
}
