// Copyright 2024 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

package storage

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"

	"github.com/cockroachdb/errors"
	"golang.org/x/crypto/scrypt"
)

const (
	scryptN      = 32768
	scryptR      = 8
	scryptP      = 1
	scryptKeyLen = 32 // AES-256
	saltLen      = 16
	// encryptedPrefix is prepended to encrypted blobs so we can distinguish
	// encrypted from plaintext CA keys.
	encryptedPrefix = "RATEL-ENCRYPTED\n"
)

// EncryptWithPassphrase encrypts data using AES-256-GCM with a key derived
// from the passphrase via scrypt. Returns: prefix + salt(16) + nonce(12) + ciphertext.
func EncryptWithPassphrase(plaintext, passphrase []byte) ([]byte, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return nil, errors.Wrap(err, "generating salt")
	}

	key, err := scrypt.Key(passphrase, salt, scryptN, scryptR, scryptP, scryptKeyLen)
	if err != nil {
		return nil, errors.Wrap(err, "deriving key")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, errors.Wrap(err, "creating cipher")
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, errors.Wrap(err, "creating GCM")
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, errors.Wrap(err, "generating nonce")
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	// prefix + salt + nonce + ciphertext
	out := make([]byte, 0, len(encryptedPrefix)+saltLen+len(nonce)+len(ciphertext))
	out = append(out, []byte(encryptedPrefix)...)
	out = append(out, salt...)
	out = append(out, nonce...)
	out = append(out, ciphertext...)
	return out, nil
}

// DecryptWithPassphrase decrypts data that was encrypted by EncryptWithPassphrase.
func DecryptWithPassphrase(encrypted, passphrase []byte) ([]byte, error) {
	prefix := []byte(encryptedPrefix)
	if len(encrypted) < len(prefix) {
		return nil, errors.New("data too short")
	}
	encrypted = encrypted[len(prefix):]

	if len(encrypted) < saltLen+12 { // salt + minimum nonce
		return nil, errors.New("encrypted data too short")
	}

	salt := encrypted[:saltLen]
	encrypted = encrypted[saltLen:]

	key, err := scrypt.Key(passphrase, salt, scryptN, scryptR, scryptP, scryptKeyLen)
	if err != nil {
		return nil, errors.Wrap(err, "deriving key")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, errors.Wrap(err, "creating cipher")
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, errors.Wrap(err, "creating GCM")
	}

	nonceSize := gcm.NonceSize()
	if len(encrypted) < nonceSize {
		return nil, errors.New("encrypted data too short for nonce")
	}

	nonce := encrypted[:nonceSize]
	ciphertext := encrypted[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, errors.Wrap(err, "decryption failed (wrong passphrase?)")
	}
	return plaintext, nil
}

// IsEncrypted returns true if the data starts with the encryption prefix.
func IsEncrypted(data []byte) bool {
	prefix := []byte(encryptedPrefix)
	if len(data) < len(prefix) {
		return false
	}
	for i, b := range prefix {
		if data[i] != b {
			return false
		}
	}
	return true
}
