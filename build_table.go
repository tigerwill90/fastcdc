package fastcdc

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"fmt"
	"math"
	"strings"
)

// Generate cleverly make "random" numbers by ciphering all zeros using a key and
// nonce (a.k.a. initialization vector) of all zeroes. This is effectively
// noise, but it is predictable noise, so the results are always the same.
func Generate() string {
	maxValue := uint32(math.Pow(2, 31))
	table := make([]byte, 1024)
	key := make([]byte, 32)
	nonce := make([]byte, 16)

	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err)
	}

	stream := cipher.NewCTR(block, nonce)
	cipher := make([]byte, 1024)
	stream.XORKeyStream(cipher, table)

	sb := strings.Builder{}
	sb.WriteString("[256]uint{\n   ")

	it := 0
	for i := 0; i < len(table); i += 4 {
		num := binary.BigEndian.Uint32(cipher[i:]) % maxValue
		if num < maxValue {
			if it%6 == 0 && it != 0 {
				sb.WriteString("\n   ")
			}
			sb.WriteString(fmt.Sprintf("%d, ", num))
		} else {
			panic(fmt.Sprintf("unexpected number: %d", num))
		}
		it++
	}

	sb.WriteString("\n}")

	return sb.String()
}
