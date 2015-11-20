package auth

import (
	"github.com/eluleci/dock/messages"
	"net/http"
	"golang.org/x/crypto/bcrypt"
	"github.com/eluleci/dock/adapters"
	"encoding/json"
	"time"
	"github.com/dgrijalva/jwt-go"
	"gopkg.in/mgo.v2/bson"
	"github.com/eluleci/dock/utils"
)

func HandleSignUp(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (response messages.Message, err error) {

	usernameArray, hasUsername := requestWrapper.Message.Body["username"]
	emailArray, hasEmail := requestWrapper.Message.Body["email"]
	password, hasPassword := requestWrapper.Message.Body["password"]

	var username, email string
	if hasUsername {
		username = usernameArray.(string)
	}
	if hasEmail {
		email = emailArray.(string)
	}
	accountData := getAccountData(requestWrapper, dbAdapter, username, email)

	if accountData != nil {
		response.Status = http.StatusConflict
		return
	}

	if (hasEmail || hasUsername) && hasPassword {
		hashedPassword, hashErr := bcrypt.GenerateFromPassword([]byte(password.(string)), bcrypt.DefaultCost)
		if hashErr != nil {
			err = hashErr
			return
		}
		requestWrapper.Message.Body["password"] = string(hashedPassword)
		response.Body, err = dbAdapter.HandlePost(requestWrapper)
		response.Status = http.StatusCreated
	} else {
		response.Status = http.StatusBadRequest
	}
	// TODO generate Access Token
	return
}

func HandleLogin(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (response messages.Message, err error) {

	emailArray, hasEmail := requestWrapper.Message.Parameters["email"]
	usernameArray, hasUsername := requestWrapper.Message.Parameters["username"]
	passwordArray, hasPassword := requestWrapper.Message.Parameters["password"]

	if (hasEmail || hasUsername) && hasPassword {
		password := passwordArray[0]

		var username, email string
		if len(usernameArray) > 0 {
			username = usernameArray[0]
		}
		if len(emailArray) > 0 {
			email = emailArray[0]
		}

		accountData := getAccountData(requestWrapper, dbAdapter, username, email)
		existingPassword := accountData["password"].(string)

		passwordError := bcrypt.CompareHashAndPassword([]byte(existingPassword), []byte(password))
		if passwordError == nil {
			delete(accountData, "password")
			response.Body = accountData

			accessToken, tokenErr := generateToken(accountData["_id"].(bson.ObjectId), username, email)
			if tokenErr == nil {
				response.Body["accessToken"] = accessToken
				response.Status = http.StatusOK
			} else {
				response.Status = http.StatusInternalServerError
			}
		} else {
			response.Status = http.StatusForbidden
		}
	} else {
		response.Status = http.StatusBadRequest
	}
	return
}


func VerifyToken(tokenString string) (userData map[string]interface{}, err error) {

	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		return []byte("SIGN_IN_KEY"), nil
	})

	if err != nil {
		err = &utils.Error{http.StatusInternalServerError, "Parsing token failed"}
	}

	if !token.Valid {
		err = &utils.Error{http.StatusUnauthorized, "Token is not valid"}
	}

	userData = token.Claims["user"].(map[string]interface{})

	return
}

func getAccountData(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter, username, email string) (accountData map[string]interface{}) {

	var whereParams = make(map[string]interface{})
	if username != "" {
		paramUsername := make(map[string]string)
		paramUsername["$eq"] = username
		whereParams["username"] = paramUsername
	}
	if email != "" {
		paramEmail := make(map[string]string)
		paramEmail["$eq"] = email
		whereParams["email"] = paramEmail
	}
	whereParamsJson, err := json.Marshal(whereParams)
	if err != nil {
		return
	}

	requestWrapper.Message.Parameters["where"] = []string{string(whereParamsJson)}
	if err != nil {
		return
	}

	results, fetchErr := dbAdapter.HandleGet(requestWrapper)
	resultsAsMap := results["data"].([]map[string]interface{})
	if fetchErr != nil || len(resultsAsMap) == 0 {
		return
	}
	accountData = resultsAsMap[0]

	return
}

func generateToken(userId bson.ObjectId, username, email string) (tokenString string, err error) {

	token := jwt.New(jwt.SigningMethodHS256)

	token.Claims["ver"] = "0.1"
	token.Claims["exp"] = time.Now().Add(time.Hour * 72).Unix()

	userData := make(map[string]interface{})
	userData["userId"] = userId
	userData["username"] = username
	userData["email"] = email
	token.Claims["user"] = userData

	tokenString, err = token.SignedString([]byte("SIGN_IN_KEY"))
	return
}