package auth

import (
	"github.com/eluleci/dock/messages"
	"net/http"
	"golang.org/x/crypto/bcrypt"
	"github.com/eluleci/dock/adapters"
	"encoding/json"
	"time"
	"github.com/dgrijalva/jwt-go"
	"github.com/eluleci/dock/utils"
	"strings"
	"io/ioutil"
	"github.com/eluleci/dock/config"
	"math/rand"
	"net/smtp"
	"fmt"
)

const (
	ActorTypeRoot = "root"
	ActorTypeCollection = "collection"
	ActorTypeModel = "model"
	ActorTypeAttribute = "attribute"
	ActorTypeFunctions = "functions"
	ClassUsers = "users"
	ClassFiles = "files"
	ResourceTypeUsers = "/users"
	ResourceTypeFiles = "/files"
	ResourceRegister = "/register"
	ResourceLogin = "/login"
	ResourceResetPassword = "/resetpassword"
	ResourceChangePassword = "/changepassword"
)

// used for password generation
var fruits = []string{"apples", "appricots", "avocados", "bananas", "cherries", "coconuts", "cranberries", "damsons",
	"dates", "durian", "grapes", "guavas", "jambuls", "jujubes", "kiwis", "lemons", "limes", "mangos", "melons",
	"olives", "oranes", "mandarines", "papayas", "peaches", "pears", "plums", "pineapples", "pumpkins", "pomelos",
	"raspberries", "satsumas", "strawberries", "tomatoes"}

// used for password generation
var quantities = []string{"two", "three", "four", "five", "six", "seven", "eight", "nine", "ten"}

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

var HandleSignUp = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (response messages.Message, hookBody map[string]interface{}, err *utils.Error) {

	_, hasUsername := requestWrapper.Message.Body["username"]
	_, hasEmail := requestWrapper.Message.Body["email"]
	_, hasFacebook := requestWrapper.Message.Body["facebook"]
	_, hasGoogle := requestWrapper.Message.Body["google"]

	if hasUsername || hasEmail {
		response.Body, hookBody, err = createLocalAccount(requestWrapper, dbAdapter)
	} else if hasFacebook {
		response.Body, hookBody, err = handleFacebookAuth(requestWrapper, dbAdapter, httpClient)
	}  else if hasGoogle {
		response.Body, hookBody, err = handleGoogleAuth(requestWrapper, dbAdapter, httpClient)
	} else {
		err = &utils.Error{http.StatusBadRequest, "No suitable registration data found."}
		return
	}

	if err != nil {
		return
	}

	accessToken, tokenErr := generateToken(response.Body["_id"].(string), response.Body)
	if tokenErr == nil {
		response.Body["accessToken"] = accessToken
		response.Status = http.StatusCreated
	} else {
		response.Status = http.StatusInternalServerError
	}
	return
}

var createLocalAccount = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (response map[string]interface{}, hookBody map[string]interface{}, err *utils.Error) {

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

	response, hookBody, err = adapters.HandlePost(ClassUsers ,dbAdapter, requestWrapper)
	return
}

var handleFacebookAuth = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter, HTTPClient *http.Client) (response map[string]interface{}, hookBody map[string]interface{}, err *utils.Error) {

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
		response, hookBody, err = adapters.HandlePost(ClassUsers, dbAdapter, requestWrapper)
		response["isNewUser"] = true
	} else {
		response = existingAccount
		response["isNewUser"] = false
		// TODO update existing token with the new token. (optionally check which expires later)
	}
	return
}

var handleGoogleAuth = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter, HTTPClient *http.Client) (response map[string]interface{}, hookBody map[string]interface{}, err *utils.Error) {

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
		response, hookBody, err = adapters.HandlePost(ClassUsers, dbAdapter, requestWrapper)
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
		accessToken, err = generateToken(accountData["_id"].(string), accountData)
		if err == nil {
			response.Body["accessToken"] = accessToken
			response.Status = http.StatusOK
		}
	} else {
		response.Status = http.StatusUnauthorized
	}
	return
}

var HandleChangePassword = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter, user interface{}) (response messages.Message, err *utils.Error) {

	userAsMap := user.(map[string]interface{})

	if len(userAsMap) == 0 {
		err = &utils.Error{http.StatusUnauthorized, "Access token must be provided for change password request."}
		return
	}

	password, hasPassword := requestWrapper.Message.Body["password"]
	if !hasPassword {
		err = &utils.Error{http.StatusBadRequest, "Password must be provided in the body with field 'password'."}
		return
	}

	newPassword, hasNewPassword := requestWrapper.Message.Body["newPassword"]
	if !hasNewPassword {
		err = &utils.Error{http.StatusBadRequest, "New password must be provided in the body with field 'newPassword'."}
		return
	}

	existingPassword := userAsMap["password"].(string)

	passwordError := bcrypt.CompareHashAndPassword([]byte(existingPassword), []byte(password.(string)))
	if passwordError != nil {
		err = &utils.Error{http.StatusUnauthorized, "Existing password is not correct."}
		return
	}

	hashedPassword, hashErr := bcrypt.GenerateFromPassword([]byte(newPassword.(string)), bcrypt.DefaultCost)
	if hashErr != nil {
		err = &utils.Error{http.StatusInternalServerError, "Hashing new password failed."}
		return
	}

	updatePasswordRW := messages.RequestWrapper{}
	updatePasswordM := messages.Message{}
	updatePasswordM.Res = ResourceTypeUsers + "/" + userAsMap["_id"].(string)
	updatePasswordM.Body = map[string]interface{}{
		"password": string(hashedPassword),
	}
	updatePasswordRW.Message = updatePasswordM

	response.Body, _, err = adapters.HandlePut(dbAdapter, updatePasswordRW)

	if err != nil {
		return
	}

	return
}

var HandleResetPassword = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (response messages.Message, err *utils.Error) {

	resetPasswordConfig := config.SystemConfig.ResetPassword
	if resetPasswordConfig == nil {
		err = &utils.Error{http.StatusInternalServerError, "Email reset configuration is not set in configuration file."}
	}

	senderEmail, hasSenderEmail := config.SystemConfig.ResetPassword["senderEmail"]
	senderEmailPassword, hasSenderEmailPassword := config.SystemConfig.ResetPassword["senderEmailPassword"]
	smtpServer, hasSmtpServer := config.SystemConfig.ResetPassword["smtpServer"]
	smtpPort, hasSmtpPort := config.SystemConfig.ResetPassword["smtpPort"]
	mailSubject, hasMailSubject := config.SystemConfig.ResetPassword["mailSubject"]
	mailContentTemplate, hasMailContent := config.SystemConfig.ResetPassword["mailContentTemplate"]

	if !hasSmtpServer || !hasSmtpPort || !hasSenderEmail || !hasSenderEmailPassword || !hasMailSubject || !hasMailContent {
		err = &utils.Error{http.StatusInternalServerError, "Email reset configuration is not correct."}
		return
	}

	recipientEmail, hasRecipientEmail := requestWrapper.Message.Body["email"]
	if !hasRecipientEmail {
		err = &utils.Error{http.StatusBadRequest, "Email must be provided in the body."}
		return
	}

	accountData, err := getAccountData(requestWrapper, dbAdapter)
	if err != nil {
		return
	}

	// generating random password like: "twoapplesandfiveoranges" or "threekiwisandsevenbananas"
	passwordFirstHalf := quantities[rand.Intn(len(quantities))] + fruits[rand.Intn(len(fruits))]
	passwordSecondHalf := quantities[rand.Intn(len(quantities))] + fruits[rand.Intn(len(fruits))]
	generatedPassword := passwordFirstHalf + "and" + passwordSecondHalf
	hashedPassword, hashErr := bcrypt.GenerateFromPassword([]byte(generatedPassword), bcrypt.DefaultCost)
	if hashErr != nil {
		err = &utils.Error{http.StatusInternalServerError, "Hashing new password failed."}
		return
	}

	updatePasswordRW := messages.RequestWrapper{}
	updatePasswordM := messages.Message{}
	updatePasswordM.Res = ResourceTypeUsers + "/" + accountData["_id"].(string)
	updatePasswordM.Body = map[string]interface{}{
		"password": string(hashedPassword),
	}
	updatePasswordRW.Message = updatePasswordM

	response.Body, _, err = adapters.HandlePut(dbAdapter, updatePasswordRW)

	if err != nil {
		return
	}

	err = sendNewPasswordEmail(smtpServer, smtpPort, senderEmail, senderEmailPassword, mailSubject, mailContentTemplate, recipientEmail.(string), generatedPassword)
	return
}

var sendNewPasswordEmail = func(smtpServer, smtpPost, senderEmail, senderEmailPassword, subject, contentTemplate, recipientEmail, newPassword string) (err *utils.Error) {

	auth := smtp.PlainAuth("", senderEmail, senderEmailPassword, smtpServer)

	generatedContent := fmt.Sprintf(contentTemplate, newPassword)
	to := []string{recipientEmail}
	msg := []byte(
	"From: " + senderEmail + "\r\n" +
	"To: " + recipientEmail + "\r\n" +
	"Subject: " + subject + "\r\n" +
	"MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n" +
	"\r\n" + generatedContent + "\r\n")
	sendMailErr := smtp.SendMail(smtpServer + ":" + smtpPost, auth, senderEmail, to, msg)

	if sendMailErr != nil {
		fmt.Println(sendMailErr)
		err = &utils.Error{http.StatusInternalServerError, "Sending email failed."}
	}
	return
}

var IsGranted = func(collection string, requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (isGranted bool, user map[string]interface{}, err *utils.Error) {

	var permissions map[string]bool

	var roles []string
	user, err = getUser(requestWrapper)
	if err != nil {
		return
	}

	roles, err = getRolesOfUser(user)
	if err != nil {
		return
	}

	res := requestWrapper.Res
	if strings.EqualFold(res, ResourceLogin) || strings.EqualFold(res, ResourceRegister) {
		isGranted = true
		return
	}

	// if res contains ':' then this is a function uri. remove the function name and get the permissions on real uri
	if strings.Index(res, "-") > 0 {
		requestWrapper.Res = requestWrapper.Res[:strings.Index(requestWrapper.Res, "-") - 1]
	}

	if strings.Count(requestWrapper.Res, "/") == 1 {
		permissions, err = getPermissionsOnResources(roles, requestWrapper)
	} else if strings.Count(requestWrapper.Res, "/") == 2 {
		id := requestWrapper.Res[strings.LastIndex(requestWrapper.Res, "/") + 1:]
		permissions, err = getPermissionsOnObject(collection, id, roles)
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

func getUser(requestWrapper messages.RequestWrapper) (user map[string]interface{}, err *utils.Error) {

	var userDataFromToken map[string]interface{}
	userDataFromToken, err = extractUserFromRequest(requestWrapper)

	if err != nil {
		return
	}

	if userDataFromToken != nil {
		userId := userDataFromToken["userId"].(string)
		user, err = adapters.HandleGetById(ClassUsers, userId)
		if err != nil {
			return
		}
	}

	return
}

func getRolesOfUser(user map[string]interface{}) (roles []string, err *utils.Error) {

	// TODO: get roles recursively

	if user != nil && user["_roles"] != nil {
		for _, r := range user["_roles"].([]interface{}) {
			roles = append(roles, "role:" + r.(string))
		}
		roles = append(roles, "user:" + user["_id"].(string))
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

func getPermissionsOnObject(collection string, id string, roles []string) (permissions map[string]bool, err *utils.Error) {

	var model map[string]interface{}
	model, err = adapters.HandleGetById(collection, id)
	if err != nil {
		return
	}

	acl := model["_acl"]
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

var generateToken = func(userId string, userData map[string]interface{}) (tokenString string, err *utils.Error) {

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

