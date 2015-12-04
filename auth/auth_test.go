package auth

import (
	"testing"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/eluleci/dock/adapters"
	"github.com/eluleci/dock/messages"
	"os"
	"gopkg.in/mgo.v2/bson"
	"net/http"
	"errors"
	"github.com/eluleci/dock/utils"
	"net/http/httptest"
	"net/url"
	"fmt"
	"github.com/eluleci/dock/config"
)

var _getAccountData = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (accountData map[string]interface{}, err *utils.Error) {
	return
}

// keeping the original value of endpoint
var _facebookTokenVerificationEndpoint = facebookTokenVerificationEndpoint

func TestMain(m *testing.M) {
	saveRealFunctions()
	os.Exit(m.Run())
}

func saveRealFunctions() {
	_getAccountData = getAccountData
}

func resetFunctions() {
	getAccountData = _getAccountData
}

func TestHandleSignUp(t *testing.T) {

	Convey("Should return bad request", t, func() {

		var called bool
		getAccountData = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (accountData map[string]interface{}, err *utils.Error) {
			called = true
			return
		}

		var message messages.Message

		var requestWrapper messages.RequestWrapper
		requestWrapper.Message = message

		_, err := HandleSignUp(requestWrapper, &adapters.MongoAdapter{})

		So(err.Code, ShouldEqual, http.StatusBadRequest)
		So(called, ShouldBeFalse)
	})

	Convey("Should return bad request for password", t, func() {

		var called bool
		getAccountData = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (accountData map[string]interface{}, err *utils.Error) {
			called = true
			return
		}

		var message messages.Message
		message.Body = make(map[string]interface{})
		message.Body["username"] = "elgefe"

		var requestWrapper messages.RequestWrapper
		requestWrapper.Message = message

		_, err := HandleSignUp(requestWrapper, &adapters.MongoAdapter{})

		So(err.Code, ShouldEqual, http.StatusBadRequest)
		So(called, ShouldBeFalse)
	})

	Convey("Should return conflict", t, func() {

		getAccountData = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (accountData map[string]interface{}, err *utils.Error) {
			accountData = make(map[string]interface{})
			return
		}

		var called bool
		generateToken  = func(userId bson.ObjectId, userData map[string]interface{}) (tokenString string, err error) {
			called = true
			err = &utils.Error{http.StatusConflict, "Exists."}
			return
		}

		var message messages.Message
		message.Body = make(map[string]interface{})
		message.Body["email"] = "email@domain.com"
		message.Body["password"] = "apasswordimpossibletofind"

		var requestWrapper messages.RequestWrapper
		requestWrapper.Message = message

		_, err := HandleSignUp(requestWrapper, &adapters.MongoAdapter{})

		So(err.Code, ShouldEqual, http.StatusConflict)
		So(called, ShouldBeFalse)
	})

	Convey("Should return internal server error", t, func() {

		getAccountData = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (accountData map[string]interface{}, err *utils.Error) {
			return
		}

		generateToken  = func(userId bson.ObjectId, userData map[string]interface{}) (tokenString string, err error) {
			err = errors.New("error")
			return
		}

		adapters.HandlePost = func (m *adapters.MongoAdapter, requestWrapper messages.RequestWrapper) (response map[string]interface{}, err *utils.Error) {
			response = make(map[string]interface{})
			response["_id"] = bson.ObjectIdHex("564f1a28e63bce219e1cc745")
			return
		}

		var message messages.Message
		message.Body = make(map[string]interface{})
		message.Body["email"] = "email@domain.com"
		message.Body["password"] = "apasswordimpossibletofind"

		var requestWrapper messages.RequestWrapper
		requestWrapper.Message = message

		response, _ := HandleSignUp(requestWrapper, &adapters.MongoAdapter{})

		So(response.Status, ShouldEqual, http.StatusInternalServerError)
	})

	Convey("Should call auth.getAccountData with email", t, func() {

		var called bool
		getAccountData = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (accountData map[string]interface{}, err *utils.Error) {
			called = true
			return
		}

		adapters.HandlePost = func (m *adapters.MongoAdapter, requestWrapper messages.RequestWrapper) (response map[string]interface{}, err *utils.Error) {
			response = make(map[string]interface{})
			response["_id"] = bson.ObjectIdHex("564f1a28e63bce219e1cc745")
			return
		}

		var message messages.Message
		message.Body = make(map[string]interface{})
		message.Body["email"] = "email@domain.com"
		message.Body["password"] = "apasswordimpossibletofind"

		var requestWrapper messages.RequestWrapper
		requestWrapper.Message = message

		HandleSignUp(requestWrapper, &adapters.MongoAdapter{})

		So(called, ShouldBeTrue)
	})

	Convey("Should call auth.getAccountData with username", t, func() {

		var called bool
		getAccountData = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (accountData map[string]interface{}, err *utils.Error) {
			called = true
			return
		}

		adapters.HandlePost = func (m *adapters.MongoAdapter, requestWrapper messages.RequestWrapper) (response map[string]interface{}, err *utils.Error) {
			response = make(map[string]interface{})
			response["_id"] = bson.ObjectIdHex("564f1a28e63bce219e1cc745")
			return
		}

		var message messages.Message
		message.Body = make(map[string]interface{})
		message.Body["username"] = "lordoftherings"
		message.Body["password"] = "apasswordimpossibletofind"

		var requestWrapper messages.RequestWrapper
		requestWrapper.Message = message

		HandleSignUp(requestWrapper, &adapters.MongoAdapter{})

		So(called, ShouldBeTrue)
	})

	Convey("Should create account", t, func() {

		getAccountData = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (accountData map[string]interface{}, err *utils.Error) {
			return
		}

		generateToken  = func(userId bson.ObjectId, userData map[string]interface{}) (tokenString string, err error) {
			tokenString = ""
			return
		}

		adapters.HandlePost = func (m *adapters.MongoAdapter, requestWrapper messages.RequestWrapper) (response map[string]interface{}, err *utils.Error) {
			response = make(map[string]interface{})
			response["_id"] = bson.ObjectIdHex("564f1a28e63bce219e1cc745")
			return
		}

		var message messages.Message
		message.Body = make(map[string]interface{})
		message.Body["email"] = "email@domain.com"
		message.Body["password"] = "apasswordimpossibletofind"

		var requestWrapper messages.RequestWrapper
		requestWrapper.Message = message

		response, _ := HandleSignUp(requestWrapper, &adapters.MongoAdapter{})

		So(response.Status, ShouldEqual, http.StatusCreated)
	})

	config.SystemConfig = config.Config{}
	config.SystemConfig.Facebook = map[string]string{
		"appId": "someappid",
		"appToken": "someAppToken",
	}

	Convey("Should fail creating account with Facebook", t, func() {

		facebookData := make(map[string]interface{})
		facebookData["id"] = "10153102991889648"

		var message messages.Message
		message.Body = make(map[string]interface{})
		message.Body["facebook"] = facebookData

		var requestWrapper messages.RequestWrapper
		requestWrapper.Message = message

		_, err := HandleSignUp(requestWrapper, &adapters.MongoAdapter{})

		So(err.Code, ShouldEqual, http.StatusBadRequest)
	})

	Convey("Should fail connecting to Facebook", t, func() {

		// Test server that always responds with 200 code, and specific payload
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(500)
		}))
		defer server.Close()

		// Make a transport that reroutes all traffic to the example server
		transport := &http.Transport{
			Proxy: func(req *http.Request) (*url.URL, error) {
				return url.Parse(server.URL)
			},
		}

		// Make a http.Client with the transport
		httpClient = &http.Client{Transport: transport}
		facebookTokenVerificationEndpoint = "https://any.endpoint.not.correct"

		facebookData := make(map[string]interface{})
		facebookData["id"] = "10153102991889648"
		facebookData["accessToken"] = "CAAOPotl9EWoBAPeLlTcQWAEUjZB3SoJG2UCHh1cpf2Q5"

		var message messages.Message
		message.Body = make(map[string]interface{})
		message.Body["facebook"] = facebookData

		var requestWrapper messages.RequestWrapper
		requestWrapper.Message = message

		_, err := HandleSignUp(requestWrapper, &adapters.MongoAdapter{})

		So(err, ShouldNotBeNil)
		So(err.Code, ShouldEqual, http.StatusInternalServerError)
	})

	// reverting endpoint back to the original value
	facebookTokenVerificationEndpoint = _facebookTokenVerificationEndpoint

	Convey("Should fail parsing Facebook response", t, func() {

		// Test server that always responds with 200 code, and specific payload
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintln(w, `"data": {"app_id": "1002354526458218","application": "Vurze","expires_at": 1449154800,"is_valid": true,"scopes": ["public_profile"],"user_id": "10153102991889648"}}`)
		}))
		defer server.Close()

		// Make a transport that reroutes all traffic to the example server
		transport := &http.Transport{
			Proxy: func(req *http.Request) (*url.URL, error) {
				return url.Parse(server.URL)
			},
		}

		// Make a http.Client with the transport
		httpClient = &http.Client{Transport: transport}

		facebookData := make(map[string]interface{})
		facebookData["id"] = "10153102991889648"
		facebookData["accessToken"] = "CAAOPotl9EWoBAPeLlTcQWAEUjZB3SoJG2UCHh1cpf2Q5"

		var message messages.Message
		message.Body = make(map[string]interface{})
		message.Body["facebook"] = facebookData

		var requestWrapper messages.RequestWrapper
		requestWrapper.Message = message

		_, err := HandleSignUp(requestWrapper, &adapters.MongoAdapter{})

		So(err, ShouldNotBeNil)
		So(err.Code, ShouldEqual, http.StatusInternalServerError)
	})

	Convey("Should fail finding required fields in Facebook response ", t, func() {

		// Test server that always responds with 200 code, and specific payload
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintln(w, `{"data": {"app_id": "1002354526458218"}}`)
		}))
		defer server.Close()

		// Make a transport that reroutes all traffic to the example server
		transport := &http.Transport{
			Proxy: func(req *http.Request) (*url.URL, error) {
				return url.Parse(server.URL)
			},
		}

		// Make a http.Client with the transport
		httpClient = &http.Client{Transport: transport}

		facebookData := make(map[string]interface{})
		facebookData["id"] = "10153102991889648"
		facebookData["accessToken"] = "CAAOPotl9EWoBAPeLlTcQWAEUjZB3SoJG2UCHh1cpf2Q5"

		var message messages.Message
		message.Body = make(map[string]interface{})
		message.Body["facebook"] = facebookData

		var requestWrapper messages.RequestWrapper
		requestWrapper.Message = message

		_, err := HandleSignUp(requestWrapper, &adapters.MongoAdapter{})

		So(err, ShouldNotBeNil)
		So(err.Code, ShouldEqual, http.StatusInternalServerError)
	})

	Convey("Should return error if token doesn't match user", t, func() {

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintln(w, `{"data": {"app_id": "someappid", "user_id": "123123123123123123", "is_valid": true}}`)
		}))
		defer server.Close()

		// Make a transport that reroutes all traffic to the example server
		transport := &http.Transport{
			Proxy: func(req *http.Request) (*url.URL, error) {
				return url.Parse(server.URL)
			},
		}

		// Make a http.Client with the transport
		httpClient = &http.Client{Transport: transport}
		facebookTokenVerificationEndpoint = "http://graph.facebook.com/debug_token"

		facebookData := make(map[string]interface{})
		facebookData["id"] = "10153102991889648"
		facebookData["accessToken"] = "CAAOPotl9EWoBAPeLlTcQWAEUjZB3SoJG2UCHh1cpf2Q5"

		var message messages.Message
		message.Body = make(map[string]interface{})
		message.Body["facebook"] = facebookData

		var requestWrapper messages.RequestWrapper
		requestWrapper.Message = message

		_, err := HandleSignUp(requestWrapper, &adapters.MongoAdapter{})

		So(err, ShouldNotBeNil)
		So(err.Code, ShouldEqual, http.StatusBadRequest)
	})

	Convey("Should return error if token is not valid", t, func() {

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintln(w, `{"data": {"app_id": "someappid", "user_id": "123123123123123123", "is_valid": true}}`)
		}))
		defer server.Close()

		// Make a transport that reroutes all traffic to the example server
		transport := &http.Transport{
			Proxy: func(req *http.Request) (*url.URL, error) {
				return url.Parse(server.URL)
			},
		}

		// Make a http.Client with the transport
		httpClient = &http.Client{Transport: transport}
		facebookTokenVerificationEndpoint = "http://graph.facebook.com/debug_token"

		facebookData := make(map[string]interface{})
		facebookData["id"] = "10153102991889648"
		facebookData["accessToken"] = "CAAOPotl9EWoBAPeLlTcQWAEUjZB3SoJG2UCHh1cpf2Q5"

		var message messages.Message
		message.Body = make(map[string]interface{})
		message.Body["facebook"] = facebookData

		var requestWrapper messages.RequestWrapper
		requestWrapper.Message = message

		_, err := HandleSignUp(requestWrapper, &adapters.MongoAdapter{})

		So(err, ShouldNotBeNil)
		So(err.Code, ShouldEqual, http.StatusBadRequest)
	})

	Convey("Should return existing account", t, func() {

		getAccountData = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (accountData map[string]interface{}, err *utils.Error) {
			accountData = make(map[string]interface{})
			accountData["_id"] = bson.ObjectIdHex("564f1a28e63bce219e1cc745")
			return
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintln(w, `{"data": {"app_id": "someappid", "user_id": "10153102991889648", "is_valid": true}}`)
		}))
		defer server.Close()

		// Make a transport that reroutes all traffic to the example server
		transport := &http.Transport{
			Proxy: func(req *http.Request) (*url.URL, error) {
				return url.Parse(server.URL)
			},
		}

		// Make a http.Client with the transport
		httpClient = &http.Client{Transport: transport}
		facebookTokenVerificationEndpoint = "http://graph.facebook.com/debug_token"

		facebookData := make(map[string]interface{})
		facebookData["id"] = "10153102991889648"
		facebookData["accessToken"] = "CAAOPotl9EWoBAPeLlTcQWAEUjZB3SoJG2UCHh1cpf2Q5"

		var message messages.Message
		message.Body = make(map[string]interface{})
		message.Body["facebook"] = facebookData

		var requestWrapper messages.RequestWrapper
		requestWrapper.Message = message

		response, err := HandleSignUp(requestWrapper, &adapters.MongoAdapter{})

		So(err, ShouldBeNil)
		So(response, ShouldNotBeNil)
	})

	Convey("Should create account with Facebook", t, func() {

		getAccountData = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (accountData map[string]interface{}, err *utils.Error) {
			return
		}

		adapters.HandlePost = func (m *adapters.MongoAdapter, requestWrapper messages.RequestWrapper) (response map[string]interface{}, err *utils.Error) {
			response = make(map[string]interface{})
			response["_id"] = bson.ObjectIdHex("564f1a28e63bce219e1cc745")
			return
		}

		// Test server that always responds with 200 code, and specific payload
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintln(w, `{"data": {"app_id": "someappid","application": "Vurze","expires_at": 1449154800,"is_valid": true,"scopes": ["public_profile"],"user_id": "10153102991889648"}}`)
		}))
		defer server.Close()

		// Make a transport that reroutes all traffic to the example server
		transport := &http.Transport{
			Proxy: func(req *http.Request) (*url.URL, error) {
				return url.Parse(server.URL)
			},
		}
		_ = transport

		// Make a http.Client with the transport
		httpClient = &http.Client{Transport: transport}
		facebookTokenVerificationEndpoint = "http://graph.facebook.com/debug_token"

		facebookData := make(map[string]interface{})
		facebookData["id"] = "10153102991889648"
		facebookData["accessToken"] = "CAAOPotl9EWoBAPeLlTcQWAEUjZB3SoJG2UCHh1cpf2Q5"

		var message messages.Message
		message.Body = make(map[string]interface{})
		message.Body["facebook"] = facebookData

		var requestWrapper messages.RequestWrapper
		requestWrapper.Message = message

		response, err := HandleSignUp(requestWrapper, &adapters.MongoAdapter{})

		So(err, ShouldBeNil)
		So(response.Status, ShouldEqual, http.StatusCreated)
	})
}

func TestHandleLogin(t *testing.T) {

	Convey("Should return bad request", t, func() {

		var called bool
		getAccountData = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (accountData map[string]interface{}, err *utils.Error) {
			called = true
			return
		}

		var message messages.Message
		message.Body = make(map[string]interface{})

		var requestWrapper messages.RequestWrapper
		requestWrapper.Message = message

		response, _ := HandleLogin(requestWrapper, &adapters.MongoAdapter{})

		So(response.Status, ShouldEqual, http.StatusBadRequest)
		So(called, ShouldBeFalse)
	})

	Convey("Should login with email", t, func() {

		getAccountData = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (accountData map[string]interface{}, err *utils.Error) {
			accountData = make(map[string]interface{})
			// hased of 'zuhaha'
			accountData["password"] = "$2a$10$wqvcYHiRvoCy5ZUurNz9wuokDH1DyXjfd8k6Hk4DSJKui76gx1yrO"
			accountData["_id"] = bson.ObjectIdHex("564f1a28e63bce219e1cc745")
			return
		}

		parameters := make(map[string][]string)
		parameters["password"] = []string{"zuhaha"}
		parameters["email"] = []string{"email@domain.com"}

		var message messages.Message
		message.Parameters = parameters

		var requestWrapper messages.RequestWrapper
		requestWrapper.Message = message

		response, _ := HandleLogin(requestWrapper, &adapters.MongoAdapter{})
		So(response.Status, ShouldEqual, http.StatusOK)
	})

	Convey("Should login with username", t, func() {

		getAccountData = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (accountData map[string]interface{}, err *utils.Error) {
			accountData = make(map[string]interface{})
			// hased of 'zuhaha'
			accountData["password"] = "$2a$10$wqvcYHiRvoCy5ZUurNz9wuokDH1DyXjfd8k6Hk4DSJKui76gx1yrO"
			accountData["_id"] = bson.ObjectIdHex("564f1a28e63bce219e1cc745")
			return
		}

		parameters := make(map[string][]string)
		parameters["password"] = []string{"zuhaha"}
		parameters["username"] = []string{"yesitsme"}

		var message messages.Message
		message.Parameters = parameters

		var requestWrapper messages.RequestWrapper
		requestWrapper.Message = message

		response, _ := HandleLogin(requestWrapper, &adapters.MongoAdapter{})
		So(response.Status, ShouldEqual, http.StatusOK)
	})

	Convey("Should return forbidden (password) error", t, func() {

		getAccountData = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (accountData map[string]interface{}, err *utils.Error) {
			accountData = make(map[string]interface{})
			// hased of 'zuhaha'
			accountData["password"] = "$2a$10$wqvcYHiRvoCy5ZUurNz9wuokDH1DyXjfd8k6Hk4DSJKui76gx1yrO"
			accountData["_id"] = bson.ObjectIdHex("564f1a28e63bce219e1cc745")
			return
		}

		parameters := make(map[string][]string)
		parameters["password"] = []string{"notzuhaha"}
		parameters["username"] = []string{"yesitsme"}

		var message messages.Message
		message.Parameters = parameters

		var requestWrapper messages.RequestWrapper
		requestWrapper.Message = message

		response, _ := HandleLogin(requestWrapper, &adapters.MongoAdapter{})
		So(response.Status, ShouldEqual, http.StatusForbidden)
	})

	Convey("Should return internal server error", t, func() {

		getAccountData = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (accountData map[string]interface{}, err *utils.Error) {
			accountData = make(map[string]interface{})
			// hased of 'zuhaha'
			accountData["password"] = "$2a$10$wqvcYHiRvoCy5ZUurNz9wuokDH1DyXjfd8k6Hk4DSJKui76gx1yrO"
			accountData["_id"] = bson.ObjectIdHex("564f1a28e63bce219e1cc745")
			return
		}

		generateToken  = func(userId bson.ObjectId, userData map[string]interface{}) (tokenString string, err error) {
			err = errors.New("error")
			return
		}

		parameters := make(map[string][]string)
		parameters["password"] = []string{"zuhaha"}
		parameters["username"] = []string{"yesitsme"}

		var message messages.Message
		message.Parameters = parameters

		var requestWrapper messages.RequestWrapper
		requestWrapper.Message = message

		response, _ := HandleLogin(requestWrapper, &adapters.MongoAdapter{})
		So(response.Status, ShouldEqual, http.StatusInternalServerError)
	})

}

