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
	"github.com/eluleci/dock/config"
)

const (
	ResourceRegister = "/register"
	ResourceLogin = "/login"
)

var commandPermissionMap = map[string]map[string]bool{
	"get": {
		"get": true,
		"query": true,
	},
	"post": {
		"create": true,
	},
	"put": {
		"update": true,
	},
	"delete": {
		"delete": true,
	},
};

var facebookTokenVerificationEndpoint = "https://graph.facebook.com/debug_token"
var googleTokenVerificationEndpoint = "https://www.googleapis.com/oauth2/v3/tokeninfo?id_token="

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
	_, hasGoogle := requestWrapper.Message.Body["google"]

	if hasUsername || hasEmail {
		response.Body, err = createLocalAccount(requestWrapper, dbAdapter)
	} else if hasFacebook {
		response.Body, err = handleFacebookAuth(requestWrapper, dbAdapter, httpClient)
	}  else if hasGoogle {
		response.Body, err = handleGoogleAuth(requestWrapper, dbAdapter, httpClient)
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

var handleFacebookAuth = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter, HTTPClient *http.Client) (response map[string]interface{}, err *utils.Error) {

	facebookData, _ := requestWrapper.Message.Body["facebook"]
	facebookDataAsMap := facebookData.(map[string]interface{})

	userId, hasId := facebookDataAsMap["id"]
	accessToken, hasAccessToken := facebookDataAsMap["accessToken"]

	if !hasId || !hasAccessToken {
		err = &utils.Error{http.StatusBadRequest, "Facebook data must contain id and access token."}
		return
	}

	appFacebookAccessToken := config.SystemConfig.Facebook["appToken"]
	if appFacebookAccessToken == "" {
		err = &utils.Error{http.StatusInternalServerError, "Facebook information is not provided in server configuration."}
		return
	}

	urlBuilder := []string{facebookTokenVerificationEndpoint, "?access_token=", appFacebookAccessToken, "&input_token=", accessToken.(string)}
	verificationUrl := strings.Join(urlBuilder, "");

	tokenResponse, verificationErr := HTTPClient.Get(verificationUrl)
	if verificationErr != nil || tokenResponse.StatusCode != 200 {
		err = &utils.Error{http.StatusInternalServerError, "Verifying token failed."}
		return
	}

	var responseBody interface{}
	data, readErr := ioutil.ReadAll(tokenResponse.Body)
	if readErr != nil {
		err = &utils.Error{http.StatusInternalServerError, "Reading token response failed."}
		return
	}

	parseErr := json.Unmarshal(data, &responseBody)
	if parseErr != nil {
		err = &utils.Error{http.StatusInternalServerError, "Parsing token response failed."}
		return
	}
	responseBodyAsMap := responseBody.(map[string]interface{})

	tokenInfo, hasTokenInfo := responseBodyAsMap["data"]
	if !hasTokenInfo {
		err = &utils.Error{http.StatusInternalServerError, "Unexpected token response from platform."}
		return
	}

	tokenInfoAsMap := tokenInfo.(map[string]interface{})

	tokensAppId, hasAppId := tokenInfoAsMap["app_id"]
	tokensUserId, hasUserId := tokenInfoAsMap["user_id"]
	isValid, hasIsValid := tokenInfoAsMap["is_valid"]
	if !hasAppId || !hasUserId || !hasIsValid {
		err = &utils.Error{http.StatusInternalServerError, "Unexpected response from Facebook while validating."}
		return
	}

	if !strings.EqualFold(tokensAppId.(string), config.SystemConfig.Facebook["appId"]) {
		err = &utils.Error{http.StatusInternalServerError, "App id doesn't match to the token's app id."}
		return
	}

	if !strings.EqualFold(tokensUserId.(string), userId.(string)) {
		err = &utils.Error{http.StatusBadRequest, "User id doesn't match to the token's user id."}
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

var handleGoogleAuth = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter, HTTPClient *http.Client) (response map[string]interface{}, err *utils.Error) {

	googleData, _ := requestWrapper.Message.Body["google"]
	googleDataAsMap := googleData.(map[string]interface{})

	_, hasId := googleDataAsMap["id"]
	idToken, hasIdToken := googleDataAsMap["idToken"]

	if !hasId || !hasIdToken {
		err = &utils.Error{http.StatusBadRequest, "Google data must contain user id and id token."}
		return
	}

	googleClientId := config.SystemConfig.Google["clientId"]
	if googleClientId == "" {
		err = &utils.Error{http.StatusInternalServerError, "Google information is not provided in server configuration."}
		return
	}

	urlBuilder := []string{googleTokenVerificationEndpoint, idToken.(string)}
	verificationUrl := strings.Join(urlBuilder, "");

	tokenResponse, verificationErr := HTTPClient.Get(verificationUrl)
	if verificationErr != nil || tokenResponse.StatusCode != 200 {
		err = &utils.Error{http.StatusInternalServerError, "Verifying token failed. "}
		return
	}

	var responseBody interface{}
	data, readErr := ioutil.ReadAll(tokenResponse.Body)
	if readErr != nil {
		err = &utils.Error{http.StatusInternalServerError, "Reading token response failed."}
		return
	}

	parseErr := json.Unmarshal(data, &responseBody)
	if parseErr != nil {
		err = &utils.Error{http.StatusInternalServerError, "Parsing token response failed."}
		return
	}
	tokenInfoAsMap := responseBody.(map[string]interface{})

	tokensClientId, hasClientId := tokenInfoAsMap["aud"]
	if !hasClientId {
		err = &utils.Error{http.StatusInternalServerError, "Unexpected token response from platform."}
		return
	}

	if !strings.EqualFold(tokensClientId.(string), googleClientId) {
		err = &utils.Error{http.StatusInternalServerError, "Client id doesn't match to the token's client id."}
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

var HandleLogin = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (response messages.Message, err *utils.Error) {

	_, hasEmail := requestWrapper.Message.Body["email"]
	_, hasUsername := requestWrapper.Message.Body["username"]
	password, hasPassword := requestWrapper.Message.Body["password"]

	if !(hasEmail || hasUsername) || !hasPassword {
		err = &utils.Error{http.StatusBadRequest, "Login request must contain username or email, and password."}
		return
	}

	accountData, getAccountErr := getAccountData(requestWrapper, dbAdapter)
	if getAccountErr != nil {
		err = getAccountErr
		if getAccountErr.Code == http.StatusNotFound {
			err = &utils.Error{http.StatusUnauthorized, "Credentials don't match or account doesn't exist."}
		}
		return
	}
	existingPassword := accountData["password"].(string)

	passwordError := bcrypt.CompareHashAndPassword([]byte(existingPassword), []byte(password.(string)))
	if passwordError == nil {
		delete(accountData, "password")
		response.Body = accountData

		var accessToken string
		accessToken, err = generateToken(accountData["_id"].(bson.ObjectId), accountData)
		if err == nil {
			response.Body["accessToken"] = accessToken
			response.Status = http.StatusOK
		}
	} else {
		response.Status = http.StatusUnauthorized
	}
	return
}

var IsGranted = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (isGranted bool, err *utils.Error) {

	var permissions map[string]bool

	res := requestWrapper.Res
	if strings.EqualFold(res, ResourceLogin) || strings.EqualFold(res, ResourceRegister) {
		isGranted = true
		return
	}

	var roles []string
	roles, err = getRolesOfUser(requestWrapper)
	if err != nil {
		return
	}

	if strings.Count(requestWrapper.Res, "/") == 1 {
		permissions, err = getPermissionsOnResources(roles, requestWrapper)
	} else if strings.Count(requestWrapper.Res, "/") == 2 {
		permissions, err = getPermissionsOnObject(roles, requestWrapper, dbAdapter)
	} else {
		// TODO handle this resources
		//		fmt.Println("ERROR: auth.go.GetPermissions(): Count of the / is more than 2: " + requestWrapper.Res)
		err = &utils.Error{http.StatusBadRequest, ""}
	}

	for k, _ := range commandPermissionMap[strings.ToLower(requestWrapper.Message.Command)] {
		if permissions[k] {
			isGranted = true
			break
		}
	}

	return
}

func getRolesOfUser(requestWrapper messages.RequestWrapper) (roles []string, err *utils.Error) {
	// TODO get roles recursively. (inherited roles)

	dbAdapter := &adapters.MongoAdapter{adapters.MongoDB.C("users")}

	var userDataFromToken map[string]interface{}
	userDataFromToken, err = extractUserFromRequest(requestWrapper)

	if err != nil {
		return
	}

	if userDataFromToken != nil {
		userId := userDataFromToken["userId"].(string)

		var rw messages.RequestWrapper
		var m messages.Message
		m.Res = "/users/" + userId
		rw.Message = m

		var user map[string]interface{}
		user, err = adapters.HandleGetById(dbAdapter, rw)
		if err != nil {
			return
		}

		if user["_roles"] != nil {
			for _, r := range user["_roles"].([]interface{}) {
				roles = append(roles, "role:" + r.(string))
			}
		}
		roles = append(roles, "user:" + userId)
	}
	roles = append(roles, "*")

	return
}

func extractUserFromRequest(requestWrapper messages.RequestWrapper) (user map[string]interface{}, err *utils.Error) {

	authHeaders := requestWrapper.Message.Headers["Authorization"]
	if authHeaders != nil && len(authHeaders) > 0 {
		accessToken := authHeaders[0]
		user, err = verifyToken(accessToken)
	}
	return
}

func getPermissionsOnObject(roles []string, requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (permissions map[string]bool, err *utils.Error) {

	var userData map[string]interface{}
	userData, err = adapters.HandleGetById(dbAdapter, requestWrapper)
	if err != nil {
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

func getPermissionsOnResources(roles []string, requestWrapper messages.RequestWrapper) (permissions map[string]bool, err *utils.Error) {

	// TODO get class type permissions and return them
	permissions = map[string]bool{
		"create": true,
		"query": true,
	}

	return
}

func verifyToken(tokenString string) (userData map[string]interface{}, err *utils.Error) {

	token, tokenErr := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		return []byte("SIGN_IN_KEY"), nil
	})

	if tokenErr != nil {
		err = &utils.Error{http.StatusInternalServerError, "Parsing token failed."}
	}

	if !token.Valid {
		err = &utils.Error{http.StatusUnauthorized, "Token is not valid."}
	}

	userData = token.Claims["user"].(map[string]interface{})

	return
}

var getAccountData = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (accountData map[string]interface{}, err *utils.Error) {

	var whereParams = make(map[string]interface{})
	var queryKey, queryParam string

	if username, hasUsername := requestWrapper.Message.Body["username"]; hasUsername && username != "" {
		queryKey = "username"
		queryParam = username.(string)
	} else if email, hasEmail := requestWrapper.Message.Body["email"]; hasEmail && email != "" {
		queryKey = "email"
		queryParam = email.(string)
	} else if facebookData, hasFacebookData := requestWrapper.Message.Body["facebook"]; hasFacebookData {
		facebookDataAsMap := facebookData.(map[string]interface{})
		queryParam = facebookDataAsMap["id"].(string)
		queryKey = "facebook.id"
	} else if googleData, hasGoogleData := requestWrapper.Message.Body["google"]; hasGoogleData {
		googleDataAsMap := googleData.(map[string]interface{})
		queryParam = googleDataAsMap["id"].(string)
		queryKey = "google.id"
	}

	query := make(map[string]string)
	query["$eq"] = queryParam
	whereParams[queryKey] = query

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

var generateToken = func(userId bson.ObjectId, userData map[string]interface{}) (tokenString string, err *utils.Error) {

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

	var signErr error
	tokenString, signErr = token.SignedString([]byte("SIGN_IN_KEY"))
	if signErr != nil {
		err = &utils.Error{http.StatusInternalServerError, "Generating token failed."}
	}
	return
}

