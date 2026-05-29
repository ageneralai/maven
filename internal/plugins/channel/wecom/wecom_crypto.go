package wecom

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/ageneralai/maven/internal/kernel/channels/webhook"
)

func wecomDecrypt(encodingAESKey, expectedReceiveID, encrypted string) (string, string, error) {
	aesKey, err := decodeWeComAESKey(encodingAESKey)
	if err != nil {
		return "", "", fmt.Errorf("decode aes key: %w", err)
	}
	raw, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", "", fmt.Errorf("base64 decode encrypted data: %w", err)
	}
	if len(raw) == 0 || len(raw)%aes.BlockSize != 0 {
		return "", "", fmt.Errorf("invalid encrypted block size")
	}
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return "", "", fmt.Errorf("new aes cipher: %w", err)
	}
	plain := make([]byte, len(raw))
	iv := aesKey[:aes.BlockSize]
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(plain, raw)
	plain, err = pkcs7Unpad(plain, 32)
	if err != nil {
		return "", "", fmt.Errorf("pkcs7 unpad: %w", err)
	}
	if len(plain) < 20 {
		return "", "", fmt.Errorf("plaintext too short")
	}
	msgLen := int(binary.BigEndian.Uint32(plain[16:20]))
	if msgLen < 0 || 20+msgLen > len(plain) {
		return "", "", fmt.Errorf("invalid msg length")
	}
	msg := plain[20 : 20+msgLen]
	receiveID := string(plain[20+msgLen:])
	exp := strings.TrimSpace(expectedReceiveID)
	if exp != "" && receiveID != exp {
		return "", "", fmt.Errorf("receive id mismatch")
	}
	return string(msg), receiveID, nil
}

func wecomEncrypt(encodingAESKey, receiveID, plaintext string) (string, error) {
	aesKey, err := decodeWeComAESKey(encodingAESKey)
	if err != nil {
		return "", fmt.Errorf("decode aes key: %w", err)
	}
	random16 := make([]byte, 16)
	if _, err := rand.Read(random16); err != nil {
		return "", fmt.Errorf("generate random bytes: %w", err)
	}
	msg := []byte(plaintext)
	msgLen := make([]byte, 4)
	binary.BigEndian.PutUint32(msgLen, uint32(len(msg)))
	raw := make([]byte, 0, len(random16)+4+len(msg)+len(receiveID))
	raw = append(raw, random16...)
	raw = append(raw, msgLen...)
	raw = append(raw, msg...)
	raw = append(raw, []byte(receiveID)...)
	padded := pkcs7Pad(raw, 32)
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return "", fmt.Errorf("new aes cipher: %w", err)
	}
	cipherData := make([]byte, len(padded))
	iv := aesKey[:aes.BlockSize]
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(cipherData, padded)
	return base64.StdEncoding.EncodeToString(cipherData), nil
}

func (w *WeComChannel) signature(timestamp, nonce, data string) string {
	return webhook.Signature(w.cfg.Token, timestamp, nonce, data)
}

func (w *WeComChannel) decrypt(encrypted string) (string, string, error) {
	return wecomDecrypt(w.cfg.EncodingAESKey, w.receiveID, encrypted)
}

func (w *WeComChannel) encrypt(plaintext, receiveID string) (string, error) {
	return wecomEncrypt(w.cfg.EncodingAESKey, receiveID, plaintext)
}

func decodeWeComAESKey(encodingAESKey string) ([]byte, error) {
	trimmed := strings.TrimSpace(encodingAESKey)
	if trimmed == "" {
		return nil, fmt.Errorf("empty encodingAESKey")
	}
	withPadding := trimmed
	if !strings.HasSuffix(withPadding, "=") {
		withPadding += "="
	}
	aesKey, err := base64.StdEncoding.DecodeString(withPadding)
	if err != nil {
		return nil, err
	}
	if len(aesKey) != 32 {
		return nil, fmt.Errorf("invalid aes key length: %d", len(aesKey))
	}
	return aesKey, nil
}

func pkcs7Pad(src []byte, blockSize int) []byte {
	padding := blockSize - len(src)%blockSize
	if padding == 0 {
		padding = blockSize
	}
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(src, padText...)
}

func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 || len(data)%blockSize != 0 {
		return nil, fmt.Errorf("invalid padded data length")
	}
	padLen := int(data[len(data)-1])
	if padLen == 0 || padLen > blockSize || padLen > len(data) {
		return nil, fmt.Errorf("invalid padding length")
	}
	for i := len(data) - padLen; i < len(data); i++ {
		if int(data[i]) != padLen {
			return nil, fmt.Errorf("invalid padding bytes")
		}
	}
	return data[:len(data)-padLen], nil
}
