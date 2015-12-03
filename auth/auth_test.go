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
)

var _getAccountData = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (accountData map[string]interface{}) {
	return
}

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
		getAccountData = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (accountData map[string]interface{}) {
			called = true
			return
		}

		var message messages.Message
		message.Body = make(map[string]interface{})
		message.Body["password"] = "apasswordimpossibletofind"

		var requestWrapper messages.RequestWrapper
		requestWrapper.Message = message

		response, _ := HandleSignUp(requestWrapper, &adapters.MongoAdapter{})

		So(response.Status, ShouldEqual, http.StatusBadRequest)
		So(called, ShouldBeFalse)
	})

	Convey("Should return conflict", t, func() {

		getAccountData = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (accountData map[string]interface{}) {
			accountData = make(map[string]interface{})
			return
		}

		var called bool
		generateToken  = func(userId bson.ObjectId, userData map[string]interface{}) (tokenString string, err error) {
			called = true
			err = errors.New("error")
			return
		}

		var message messages.Message
		message.Body = make(map[string]interface{})
		message.Body["email"] = "email@domain.com"
		message.Body["password"] = "apasswordimpossibletofind"

		var requestWrapper messages.RequestWrapper
		requestWrapper.Message = message

		response, _ := HandleSignUp(requestWrapper, &adapters.MongoAdapter{})

		So(response.Status, ShouldEqual, http.StatusConflict)
		So(called, ShouldBeFalse)
	})

	Convey("Should return internal server error", t, func() {

		getAccountData = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (accountData map[string]interface{}) {
			return
		}

		generateToken  = func(userId bson.ObjectId, userData map[string]interface{}) (tokenString string, err error) {
			err = errors.New("error")
			return
		}

		adapters.HandlePost = func (m *adapters.MongoAdapter, requestWrapper messages.RequestWrapper) (response map[string]interface{}, err error) {
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
		getAccountData = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (accountData map[string]interface{}) {
			called = true
			return
		}

		adapters.HandlePost = func (m *adapters.MongoAdapter, requestWrapper messages.RequestWrapper) (response map[string]interface{}, err error) {
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
		getAccountData = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (accountData map[string]interface{}) {
			called = true
			return
		}

		adapters.HandlePost = func (m *adapters.MongoAdapter, requestWrapper messages.RequestWrapper) (response map[string]interface{}, err error) {
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

		getAccountData = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (accountData map[string]interface{}) {
			return
		}

		generateToken  = func(userId bson.ObjectId, userData map[string]interface{}) (tokenString string, err error) {
			tokenString = ""
			return
		}

		adapters.HandlePost = func (m *adapters.MongoAdapter, requestWrapper messages.RequestWrapper) (response map[string]interface{}, err error) {
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
}

