package cloudkey

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt"
)

var sampleSecretKey = []byte("SecretYouShouldHide")

// CreateToken :
func CreateToken(uid int, account, username string) (string, error) {
	now := time.Now()
	// 加上 24 小時
	future := now.Add(24 * time.Hour)
	// 將時間轉換為 Unix 時間戳記
	timestamp := future.Unix()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"uid":      uid,
		"account":  account,
		"username": username,
		"exp":      timestamp,
	})

	return token.SignedString(sampleSecretKey)
}

// ValidateToken :
func ValidateToken(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Don't forget to validate the alg is what you expect:
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		// hmacSampleSecret is a []byte containing your secret, e.g. []byte("my_secret_key")
		return sampleSecretKey, nil
	})

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return claims, nil
	} else {
		return jwt.MapClaims{}, err
	}
}
