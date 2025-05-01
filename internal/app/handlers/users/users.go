package users

import (
	"net/http"

	"github.com/ryabkov82/gofermart/internal/app/utils/jwtauth"
)

type Request struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

func IssueNewToken(w http.ResponseWriter, userID int, login string, jwtKey []byte) error {
	token, err := jwtauth.GenerateNewToken(userID, login, jwtKey)
	if err != nil {
		return err
	}
	setTokenCookie(w, token)
	//log.Printf("Issued new JWT for user: %s", userID)
	return nil
}

// Устанавливает JWT в куки
func setTokenCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    token,
		HttpOnly: true,
		Path:     "/",
		//Secure:   true, // HTTPS-only
		SameSite: http.SameSiteStrictMode,
	})
}
