package proxy

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
)

const passwordEnvelope = "aes128gcm-v1:"
const passwordPurpose = "SGFG/settings/relay/password/v2"

// Machine/user identifiers are stable identifiers, not secrets. This provides
// local binding and casual at-rest protection, not a credential vault.
func derivePasswordKey(machineID, userSID string) []byte {
	sum := sha256.Sum256([]byte(passwordPurpose + "\x00" + machineID + "\x00" + userSID))
	return sum[:16]
}

func passwordGCM(key []byte) (cipher.AEAD, error) {
	if len(key) != 16 {
		return nil, errors.New("密码加密密钥长度无效")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func encryptPassword(password string, key []byte) (string, error) {
	gcm, err := passwordGCM(key)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nil, nonce, []byte(password), []byte(passwordPurpose))
	return passwordEnvelope + base64.RawStdEncoding.EncodeToString(append(nonce, sealed...)), nil
}

func decryptPassword(envelope string, key []byte) (string, error) {
	failure := errors.New("SS 中继密码无法解密（本机/账户不匹配或配置已损坏），请修复或移走设置文件后重新启动")
	if !strings.HasPrefix(envelope, passwordEnvelope) {
		return "", failure
	}
	gcm, err := passwordGCM(key)
	if err != nil {
		return "", failure
	}
	data, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(envelope, passwordEnvelope))
	if err != nil || len(data) < gcm.NonceSize()+gcm.Overhead() {
		return "", failure
	}
	plain, err := gcm.Open(nil, data[:gcm.NonceSize()], data[gcm.NonceSize():], []byte(passwordPurpose))
	if err != nil {
		return "", failure
	}
	return string(plain), nil
}
