package blenc

type keyInfoStatic struct {
	t       byte
	key, iv []byte
}

func (k *keyInfoStatic) GetSymmetricKey() (byte, []byte, []byte, error) {
	return k.t, k.key, k.iv, nil
}

func NewStaticKeyInfo(t byte, key, iv []byte) KeyInfo {
	return &keyInfoStatic{
		t: t, key: key, iv: iv,
	}
}
