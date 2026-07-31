package netease

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"
	"net/url"
	"strings"
)

const linuxApiKeyHex = "7246674226682325323F5E6544673A51"

const (
	weApiNonce      = "0CoJUm6Qyw8W8jud"
	weApiIv         = "0102030405060708"
	weApiPubModulus = "00e0b509f6259df8642dbc35662901477df22677ec152b5ff68ace615bb7b725152b3ab17a876aea8a5aa76d2e417629ec4ee341f56135fccf695280104e0312ecbda92557c93870114af6c9d05c4f7f0c3685b7a46bee255932575cce10b424d813cfe4875d3e82047b97ddef52741d546b8e289dc6935b3ece0462db0a22b8e7"
	weApiPubKey     = "010001"
)

func pkcs7Padding(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	return append(data, bytes.Repeat([]byte{byte(padding)}, padding)...)
}

func randomString(size int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, size)
	rand.Read(b)
	for i, v := range b {
		b[i] = letters[int(v)%len(letters)]
	}
	return string(b)
}

func reverseString(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

func aesEncryptECB(data, key []byte) []byte {
	block, _ := aes.NewCipher(key)
	data = pkcs7Padding(data, block.BlockSize())
	out := make([]byte, len(data))
	for bs, be := 0, block.BlockSize(); bs < len(data); bs, be = bs+block.BlockSize(), be+block.BlockSize() {
		block.Encrypt(out[bs:be], data[bs:be])
	}
	return out
}

func aesEncryptCBC(text, key, iv string) string {
	src := pkcs7Padding([]byte(text), aes.BlockSize)
	block, _ := aes.NewCipher([]byte(key))
	out := make([]byte, len(src))
	cipher.NewCBCEncrypter(block, []byte(iv)).CryptBlocks(out, src)
	return base64.StdEncoding.EncodeToString(out)
}

func rsaEncrypt(text, pubKey, modulus string) string {
	hexText := hex.EncodeToString([]byte(reverseString(text)))
	biText, _ := new(big.Int).SetString(hexText, 16)
	biPub, _ := new(big.Int).SetString(pubKey, 16)
	biMod, _ := new(big.Int).SetString(modulus, 16)
	return fmt.Sprintf("%0256x", new(big.Int).Exp(biText, biPub, biMod))
}

func EncryptLinux(data string) string {
	key, _ := hex.DecodeString(linuxApiKeyHex)
	return strings.ToUpper(hex.EncodeToString(aesEncryptECB([]byte(data), key)))
}

func EncryptWeApi(text string) (string, string) {
	secKey := randomString(16)
	params := aesEncryptCBC(aesEncryptCBC(text, weApiNonce, weApiIv), secKey, weApiIv)
	encSecKey := rsaEncrypt(secKey, weApiPubKey, weApiPubModulus)
	return params, encSecKey
}

func encryptEApi(urlPath, payload string) string {
	u, _ := url.Parse(urlPath)
	if u.Path != "" {
		urlPath = u.Path
	}
	urlPath = strings.ReplaceAll(urlPath, "/eapi/", "/api/")
	digest := md5Hex(fmt.Sprintf("nobody%suse%smd5forencrypt", urlPath, payload))
	data := fmt.Sprintf("%s-36cd479b6b5-%s-36cd479b6b5-%s", urlPath, payload, digest)
	return hex.EncodeToString(aesEncryptECB([]byte(data), []byte("e82ckenh8dichen8")))
}

func md5Hex(s string) string {
	h := md5.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}
