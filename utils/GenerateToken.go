package utils

import (
	"gosplitbill/model"
	"os"
	"strconv"

	"github.com/golang-jwt/jwt"
)

func GenerateTokenJWT(user *model.User) (string, error) {
	expHours, _ := strconv.Atoi((os.Getenv("JWT_EXPIRE_HOURS")))

	if expHours == 0 {
		expHours = 24
	}

	claims := jwt.MapClaims{
		"user_id": user.ID,
		"email": user.Email,
		"username": user.UserName,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString([]byte (os.Getenv("JWT_SECRET")))

}


func GenerateTokenChangePassword(user *model.User) (string, error) {
		expHours, _ := strconv.Atoi((os.Getenv("JWT_EXPIRE_HOURS")))

	if expHours == 0 {
		expHours = 24
	}

	claims := jwt.MapClaims{
		"email": user.Email,
		"user_id": user.ID,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString([]byte (os.Getenv("JWT_SECRET")))
}

