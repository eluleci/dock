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

var HandleSignUp = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (response messages.Message, err error) {

	_, hasUsername := requestWrapper.Message.Body["username"]
	_, hasEmail := requestWrapper.Message.Body["email"]
	password, hasPassword := requestWrapper.Message.Body["password"]

	if !(hasEmail || hasUsername) || !hasPassword {
		response.Status = http.StatusBadRequest
		return
	}

	var existingAccount map[string]interface{}
	if hasUsername {
		existingAccount = getAccountData(requestWrapper, dbAdapter)
	} else if hasEmail {
		existingAccount = getAccountData(requestWrapper, dbAdapter)
	}

	if existingAccount != nil {
		response.Status = http.StatusConflict
		return
	}

	hashedPassword, hashErr := bcrypt.GenerateFromPassword([]byte(password.(string)), bcrypt.DefaultCost)
	if hashErr != nil {
		err = hashErr
		response.Status = http.StatusInternalServerError
		return
	}
	requestWrapper.Message.Body["password"] = string(hashedPassword)

	response.Body, err = adapters.HandlePost(dbAdapter, requestWrapper)
	fmt.Println(response.Body)
	accessToken, tokenErr := generateToken(response.Body["_id"].(bson.ObjectId), response.Body)
	if tokenErr == nil {
		response.Body["accessToken"] = accessToken
		response.Status = http.StatusCreated
	} else {
		response.Status = http.StatusInternalServerError
	}
	return
}

var HandleLogin = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (response messages.Message, err error) {

	emailArray, hasEmail := requestWrapper.Message.Parameters["email"]
	usernameArray, hasUsername := requestWrapper.Message.Parameters["username"]
	passwordArray, hasPassword := requestWrapper.Message.Parameters["password"]

	if (hasEmail || hasUsername) && hasPassword {
		password := passwordArray[0]

		requestWrapper.Message.Body = make(map[string]interface{})
		if len(usernameArray) > 0 {
			requestWrapper.Message.Body["username"] = usernameArray[0]
		}
		if len(emailArray) > 0 {
			requestWrapper.Message.Body["email"] = emailArray[0]
		}

		accountData := getAccountData(requestWrapper, dbAdapter)
		existingPassword := accountData["password"].(string)

		passwordError := bcrypt.CompareHashAndPassword([]byte(existingPassword), []byte(password))
		if passwordError == nil {
			delete(accountData, "password")
			response.Body = accountData

			accessToken, tokenErr := generateToken(accountData["_id"].(bson.ObjectId), accountData)
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

var checkAuthRequirements = func() {

}

var GetPermissions = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (permissions map[string]bool, err utils.Error) {

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
	// TODO get roles recursively. (inherited roles)

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

		user, getErr := adapters.HandleGetById(dbAdapter, rw)
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

	userData, getErr := adapters.HandleGetById(dbAdapter, requestWrapper)

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

var getAccountData = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (accountData map[string]interface{}) {

	var whereParams = make(map[string]interface{})

	if username, hasUsername := requestWrapper.Message.Body["username"]; hasUsername && username != "" {
		paramUsername := make(map[string]string)
		paramUsername["$eq"] = username.(string)
		whereParams["username"] = paramUsername
	}
	if email, hasEmail := requestWrapper.Message.Body["email"]; hasEmail && email != "" {
		paramEmail := make(map[string]string)
		paramEmail["$eq"] = email.(string)
		whereParams["email"] = paramEmail
	}

	whereParamsJson, err := json.Marshal(whereParams)
	if err != nil {
		return
	}
	requestWrapper.Message.Parameters["where"] = []string{string(whereParamsJson)}

	results, fetchErr := adapters.HandleGet(dbAdapter, requestWrapper)
	resultsAsMap := results["data"].([]map[string]interface{})
	if fetchErr != nil || len(resultsAsMap) == 0 {
		return
	}
	accountData = resultsAsMap[0]

	return
}

var generateToken = func(userId bson.ObjectId, userData map[string]interface{}) (tokenString string, err error) {

	token := jwt.New(jwt.SigningMethodHS256)

	userTokenData := make(map[string]interface{})
	userTokenData["userId"] = userId

	if username, hasUsername := userData["username"]; hasUsername && username != "" {
		userTokenData["username"] = username
	}
	if email, hasEmail := userData["email"]; hasEmail && email != "" {
		userTokenData["email"] = email
	}

	token.Claims["ver"] = "0.1"
	token.Claims["exp"] = time.Now().Add(time.Hour * 72).Unix()
	token.Claims["user"] = userTokenData

	tokenString, err = token.SignedString([]byte("SIGN_IN_KEY"))
	return
}

