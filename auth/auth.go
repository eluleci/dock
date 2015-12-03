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
	"io/ioutil"
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

var httpClient = http.DefaultClient

var HandleSignUp = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (response messages.Message, err *utils.Error) {

	_, hasUsername := requestWrapper.Message.Body["username"]
	_, hasEmail := requestWrapper.Message.Body["email"]
	_, hasFacebook := requestWrapper.Message.Body["facebook"]

	if hasUsername || hasEmail {
		response.Body, err = createLocalAccount(requestWrapper, dbAdapter)
	} else if hasFacebook {
		response.Body, err = createSocialAccount(requestWrapper, dbAdapter, httpClient)
	} else {
		err = &utils.Error{http.StatusBadRequest, "No suitable registration data found."}
		return
	}

	if err != nil {
		return
	}

	accessToken, tokenErr := generateToken(response.Body["_id"].(bson.ObjectId), response.Body)
	if tokenErr == nil {
		response.Body["accessToken"] = accessToken
		response.Status = http.StatusCreated
	} else {
		response.Status = http.StatusInternalServerError
	}
	return
}

var createLocalAccount = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (response map[string]interface{}, err *utils.Error) {

	_, hasUsername := requestWrapper.Message.Body["username"]
	_, hasEmail := requestWrapper.Message.Body["email"]
	password, hasPassword := requestWrapper.Message.Body["password"]

	if !(hasEmail || hasUsername) || !hasPassword {
		err = &utils.Error{http.StatusBadRequest, "Username or email, and password must be provided."}
		return
	}

	existingAccount, _ := getAccountData(requestWrapper, dbAdapter)
	if existingAccount != nil {
		err = &utils.Error{http.StatusConflict, "User with same email-username already exists."}
		return
	}

	hashedPassword, hashErr := bcrypt.GenerateFromPassword([]byte(password.(string)), bcrypt.DefaultCost)
	if hashErr != nil {
		err = &utils.Error{http.StatusInternalServerError, "Hashing password failed."}
		return
	}
	requestWrapper.Message.Body["password"] = string(hashedPassword)

	response, err = adapters.HandlePost(dbAdapter, requestWrapper)
	return
}

var verificationEndpoint = "https://graph.facebook.com/debug_token"

var createSocialAccount = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter, HTTPClient *http.Client) (response map[string]interface{}, err *utils.Error) {

	facebookData, _ := requestWrapper.Message.Body["facebook"]
	facebookDataAsMap := facebookData.(map[string]interface{})

	userId, hasId := facebookDataAsMap["id"]
	accessToken, hasAccessToken := facebookDataAsMap["accessToken"]

	if !hasId || !hasAccessToken {
		err = &utils.Error{http.StatusBadRequest, "Facebook data must contain id and access token."}
		return
	}

	appFacebookAccessToken := "1002354526458218|j5aGV36GyRfmK0D-nE3eu3vtg1s"

	urlBuilder := []string{verificationEndpoint, "?access_token=", appFacebookAccessToken, "&input_token=", accessToken.(string)}
	verificationUrl := strings.Join(urlBuilder, "");

	tokenResponse, verificationErr := HTTPClient.Get(verificationUrl)
	if verificationErr != nil || tokenResponse.StatusCode != 200 {
		err = &utils.Error{http.StatusInternalServerError, "Reaching Facebook API failed. "}
		return
	}

	var responseBody interface{}
	data, readErr := ioutil.ReadAll(tokenResponse.Body)
	if readErr != nil {
		err = &utils.Error{http.StatusInternalServerError, "Reading Facebook response failed."}
		return
	}

	parseErr := json.Unmarshal(data, &responseBody)
	if parseErr != nil {
		err = &utils.Error{http.StatusInternalServerError, "Parsing Facebook response failed."}
		return
	}

	responseBodyAsMap := responseBody.(map[string]interface{})
	tokenInfo, hasTokenInfo := responseBodyAsMap["data"]
	if !hasTokenInfo {
		err = &utils.Error{http.StatusInternalServerError, "Unexpected response from Facebook while validating."}
		return
	}

	tokenInfoAsMap := tokenInfo.(map[string]interface{})

	tokensUserId, hasUserId := tokenInfoAsMap["user_id"]
	isValid, hasIsValid := tokenInfoAsMap["is_valid"]
	if !hasUserId || !hasIsValid {
		err = &utils.Error{http.StatusInternalServerError, "Unexpected response from Facebook while validating."}
		return
	}

	if !strings.EqualFold(tokensUserId.(string), userId.(string)) {
		err = &utils.Error{http.StatusBadRequest, "User id doesn't match the token."}
		return
	}

	if !isValid.(bool) {
		err = &utils.Error{http.StatusBadRequest, "Token is not valid."}
		return
	}

	existingAccount, _ := getAccountData(requestWrapper, dbAdapter)

	if existingAccount == nil {
		response, err = adapters.HandlePost(dbAdapter, requestWrapper)
		response["isNewUser"] = true
	} else {
		response = existingAccount
		response["isNewUser"] = false
		// TODO update existing token with the new token. (optionally check which expires later)
	}
	return
}

var HandleLogin = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (response messages.Message, err error) {

	emailArray, hasEmail := requestWrapper.Message.Parameters["email"]
	usernameArray, hasUsername := requestWrapper.Message.Parameters["username"]
	passwordArray, hasPassword := requestWrapper.Message.Parameters["password"]

	if !(hasEmail || hasUsername) || !hasPassword {
		response.Status = http.StatusBadRequest
		return
	}
	password := passwordArray[0]

	// adding username or email to request wrapper to get account data
	requestWrapper.Message.Body = make(map[string]interface{})
	if hasUsername {
		requestWrapper.Message.Body["username"] = usernameArray[0]
	} else if hasEmail {
		requestWrapper.Message.Body["email"] = emailArray[0]
	}

	accountData, getAccountErr := getAccountData(requestWrapper, dbAdapter)
	if getAccountErr != nil {
		if getAccountErr.Code == http.StatusNotFound {
			err = &utils.Error{http.StatusUnauthorized, "Credentials don't match or account doesn't exist."}
		} else {
			err = getAccountErr
		}
		return
	}
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
	return
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
		//		fmt.Println("ERROR: auth.go.GetPermissions(): Count of the / is more than 2: " + requestWrapper.Res)
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

var getAccountData = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (accountData map[string]interface{}, err *utils.Error) {

	var whereParams = make(map[string]interface{})

	if username, hasUsername := requestWrapper.Message.Body["username"]; hasUsername && username != "" {
		paramUsername := make(map[string]string)
		paramUsername["$eq"] = username.(string)
		whereParams["username"] = paramUsername
	} else if email, hasEmail := requestWrapper.Message.Body["email"]; hasEmail && email != "" {
		paramEmail := make(map[string]string)
		paramEmail["$eq"] = email.(string)
		whereParams["email"] = paramEmail
	} else if facebookData, hasFacebookData := requestWrapper.Message.Body["facebook"]; hasFacebookData {
		facebookDataAsMap := facebookData.(map[string]interface{})
		userId := facebookDataAsMap["id"]
		paramFacebookId := make(map[string]string)
		paramFacebookId["$eq"] = userId.(string)
		whereParams["facebook.id"] = paramFacebookId
	}

	whereParamsJson, jsonErr := json.Marshal(whereParams)
	if jsonErr != nil {
		err = &utils.Error{http.StatusInternalServerError, "Creating user request failed."}
		return
	}
	requestWrapper.Message.Parameters["where"] = []string{string(whereParamsJson)}

	results, fetchErr := adapters.HandleGet(dbAdapter, requestWrapper)
	resultsAsMap := results["data"].([]map[string]interface{})
	if fetchErr != nil || len(resultsAsMap) == 0 {
		err = &utils.Error{http.StatusNotFound, "Item not found."}
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

