package auth

import (
	"crypto/md5"
	"encoding/hex"
	"strings"
)

// EmailToMD5 convert email to salted md5
func EmailToMD5(email string) string {
	email = strings.ToLower(strings.TrimSpace(email))
	hash := md5.Sum([]byte(email))
	return hex.EncodeToString(hash[:])
}
