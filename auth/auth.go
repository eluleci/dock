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
	"strings"
	"fmt"
)

const (
	ResourceRegister = "/register"
	ResourceLogin = "/login"
)

var defaultPermissions = map[string]bool{
	"create": true,
	"query": true,
	"get": true,
	"update": true,
	"delete": true,

}

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


func GetPermissions(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (permissions map[string]bool, err utils.Error) {

	res := requestWrapper.Res
	if strings.EqualFold(res, ResourceLogin) || strings.EqualFold(res, ResourceRegister) {
		permissions = defaultPermissions
		return
	}

	roles, authErr := getRolesOfUser(requestWrapper)

	if authErr.Code != 0 {
		err = utils.Error{http.StatusInternalServerError, ""}
		return
	}

	var pErr error
	if strings.Count(requestWrapper.Res, "/") == 1 {
		permissions, pErr = getPermissionsOnResources(roles, requestWrapper)
	} else if strings.Count(requestWrapper.Res, "/") == 2 {
		permissions, pErr = getPermissionsOnObject(roles, requestWrapper, dbAdapter)
	} else {
		// TODO handle this resources
		fmt.Println("ERROR: auth.go.GetPermissions(): Count of the / is more than 2: " + requestWrapper.Res)
		err = utils.Error{http.StatusBadRequest, ""}
	}

	if pErr != nil {
		err = utils.Error{http.StatusInternalServerError, ""}
	}

	return
}

func getRolesOfUser(requestWrapper messages.RequestWrapper) (roles []string, err utils.Error) {

	dbAdapter := &adapters.MongoAdapter{adapters.MongoDB.C("users")}
	userDataFromToken, tokenErr := extractUserFromRequest(requestWrapper)

	if tokenErr.Code != 0 {
		err = tokenErr
		return
	}

	if userDataFromToken != nil {
		userId := userDataFromToken["userId"].(string)

		var rw messages.RequestWrapper
		var m messages.Message
		m.Res = "/users/" + userId
		rw.Message = m

		user, getErr := dbAdapter.HandleGetById(rw)
		if getErr != nil {
			err = utils.Error{800, ""}
			return
		} else if user["_roles"] != nil {
			for _, r := range user["_roles"].([]interface{}) {
				roles = append(roles, "role:" + r.(string))
			}
		}
		roles = append(roles, "user:" + userId)
	}
	roles = append(roles, "*")

	return
}


func extractUserFromRequest(requestWrapper messages.RequestWrapper) (user map[string]interface{}, err utils.Error) {

	authHeaders := requestWrapper.Message.Headers["Authorization"]
	if authHeaders != nil && len(authHeaders) > 0 {
		accessToken := authHeaders[0]
		user, err = verifyToken(accessToken)
	}
	return
}

func getPermissionsOnObject(roles []string, requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (permissions map[string]bool, err error) {

	userData, getErr := dbAdapter.HandleGetById(requestWrapper)

	if getErr != nil {
		err = getErr
		return
	}

	acl := userData["_acl"]
	if acl != nil {
		permissions = make(map[string]bool)

		for _, v := range roles {
			p := acl.(map[string]interface{})[v]
			if p != nil {
				for kAcl, _ := range p.(map[string]interface{}) {
					permissions[kAcl] = true
				}
			}
		}
	}else {
		permissions = map[string]bool{
			"get": true,
			"update": true,
			"delete": true,
		}
	}

	return
}

func getPermissionsOnResources(roles []string, requestWrapper messages.RequestWrapper) (permissions map[string]bool, err error) {

	// TODO get class type permissions and return them
	permissions = map[string]bool{
		"create": true,
		"query": true,
	}

	return
}


func verifyToken(tokenString string) (userData map[string]interface{}, err utils.Error) {

	token, tokenErr := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		return []byte("SIGN_IN_KEY"), nil
	})

	if tokenErr != nil {
		err = utils.Error{http.StatusInternalServerError, "Parsing token failed"}
	}

	if !token.Valid {
		err = utils.Error{http.StatusUnauthorized, "Token is not valid"}
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
	if len(username) > 0 {
		userData["username"] = username
	}
	if len(email) > 0 {
		userData["email"] = email
	}
	token.Claims["user"] = userData

	tokenString, err = token.SignedString([]byte("SIGN_IN_KEY"))
	return
}

