package actors

import (
	"testing"
	"github.com/eluleci/dock/auth"
	"github.com/eluleci/dock/messages"
	"github.com/eluleci/dock/adapters"
	"github.com/eluleci/dock/utils"
	. "github.com/smartystreets/goconvey/convey"
	"net/http"
)


var originalHandleGet = func (a *Actor, requestWrapper messages.RequestWrapper) (response messages.Message, err error) {
	return
}

func TestHandleRequest(t *testing.T) {

	Convey("Should call auth.GetPermissions", t, func() {

		var getPermissionsCalled bool
		auth.GetPermissions = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (permissions map[string]bool, err utils.Error) {
			getPermissionsCalled = true
			return
		}

		handleRequest(&Actor{}, messages.RequestWrapper{})
		So(getPermissionsCalled, ShouldBeTrue)

	})

	Convey("Should return permission error", t, func() {

		auth.GetPermissions = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (permissions map[string]bool, err utils.Error) {
			err = utils.Error{http.StatusInternalServerError, ""}
			return
		}

		response, _ := handleRequest(&Actor{}, messages.RequestWrapper{})
		So(response.Status, ShouldEqual, http.StatusInternalServerError)

	})

	/////////////////////////
	// GET
	/////////////////////////
	Convey("Should call handleGet", t, func() {

		auth.GetPermissions = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (permissions map[string]bool, err utils.Error) {
			permissions = map[string]bool{"create": true,"query": true,"get": true,"update": true,"delete": true,}
			return
		}

		var called bool
		originalHandleGet = handleGet
		handleGet = func (a *Actor, requestWrapper messages.RequestWrapper) (response messages.Message, err error) {

			called = true
			return
		}

		var m messages.Message
		m.Command = "get"

		var rw messages.RequestWrapper
		rw.Message = m

		_, err := handleRequest(&Actor{}, rw)
		So(err, ShouldBeNil)
		So(called, ShouldBeTrue)

	})

	Convey("Should return Authorization error for GET", t, func() {

		auth.GetPermissions = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (permissions map[string]bool, err utils.Error) {
			permissions = map[string]bool{"create": true,"update": true,"delete": true,}
			return
		}

		var m messages.Message
		m.Command = "get"

		var rw messages.RequestWrapper
		rw.Message = m

		_, err := handleRequest(&Actor{}, rw)
		So(err, ShouldNotBeNil)
		So((err.(*utils.Error)).Code, ShouldEqual, http.StatusUnauthorized)

	})

	/////////////////////////
	// POST
	/////////////////////////
	Convey("Should call handlePost", t, func() {

		auth.GetPermissions = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (permissions map[string]bool, err utils.Error) {
			permissions = map[string]bool{"create": true,"query": true,"get": true,"update": true,"delete": true,}
			return
		}

		var called bool
		handlePost = func (a *Actor, requestWrapper messages.RequestWrapper) (response messages.Message, err error) {
			called = true
			return
		}

		var m messages.Message
		m.Command = "post"

		var rw messages.RequestWrapper
		rw.Message = m

		_, err := handleRequest(&Actor{}, rw)
		So(err, ShouldBeNil)
		So(called, ShouldBeTrue)

	})

	Convey("Should return Authorization error for POST", t, func() {

		auth.GetPermissions = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (permissions map[string]bool, err utils.Error) {
			permissions = map[string]bool{"query": true,"get": true,"update": true,"delete": true,}
			return
		}

		var m messages.Message
		m.Command = "post"

		var rw messages.RequestWrapper
		rw.Message = m

		_, err := handleRequest(&Actor{}, rw)
		So(err, ShouldNotBeNil)
		So((err.(*utils.Error)).Code, ShouldEqual, http.StatusUnauthorized)

	})

	/////////////////////////
	// PUT
	/////////////////////////
	Convey("Should call handlePut", t, func() {

		auth.GetPermissions = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (permissions map[string]bool, err utils.Error) {
			permissions = map[string]bool{"create": true,"query": true,"get": true,"update": true,"delete": true,}
			return
		}

		var called bool
		handlePut = func (a *Actor, requestWrapper messages.RequestWrapper) (response messages.Message, err error) {
			called = true
			return
		}

		var m messages.Message
		m.Command = "put"
		var rw messages.RequestWrapper
		rw.Message = m

		_, err := handleRequest(&Actor{}, rw)
		So(err, ShouldBeNil)
		So(called, ShouldBeTrue)

	})

	Convey("Should return Authorization error for PUT", t, func() {

		auth.GetPermissions = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (permissions map[string]bool, err utils.Error) {
			permissions = map[string]bool{"create": true,"query": true,"get": true,"delete": true,}
			return
		}

		var m messages.Message
		m.Command = "put"

		var rw messages.RequestWrapper
		rw.Message = m

		_, err := handleRequest(&Actor{}, rw)
		So(err, ShouldNotBeNil)
		So((err.(*utils.Error)).Code, ShouldEqual, http.StatusUnauthorized)

	})

	/////////////////////////
	// DELETE
	/////////////////////////
	Convey("Should call handleDelete", t, func() {

		auth.GetPermissions = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (permissions map[string]bool, err utils.Error) {
			permissions = map[string]bool{"create": true,"query": true,"get": true,"update": true,"delete": true,}
			return
		}

		var called bool
		handleDelete = func (a *Actor, requestWrapper messages.RequestWrapper) (response messages.Message, err error) {
			called = true
			return
		}

		var m messages.Message
		m.Command = "delete"
		var rw messages.RequestWrapper
		rw.Message = m

		_, err := handleRequest(&Actor{}, rw)
		So(err, ShouldBeNil)
		So(called, ShouldBeTrue)

	})

	Convey("Should return Authorization error for DELETE", t, func() {

		auth.GetPermissions = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (permissions map[string]bool, err utils.Error) {
			permissions = map[string]bool{"create": true,"query": true,"get": true,"update": true,}
			return
		}

		var m messages.Message
		m.Command = "delete"

		var rw messages.RequestWrapper
		rw.Message = m

		_, err := handleRequest(&Actor{}, rw)
		So(err, ShouldNotBeNil)
		So((err.(*utils.Error)).Code, ShouldEqual, http.StatusUnauthorized)

	})
}

func TestHandleGet(t *testing.T) {

	resetFunctions()
	Convey("Should call auth.HandleLogin", t, func() {

		var called bool
		auth.HandleLogin = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (response messages.Message, err error) {
			called = true
			return
		}

		var actor Actor
		actor.res = ResourceLogin
		_, err := handleGet(&actor, messages.RequestWrapper{})
		So(err, ShouldBeNil)
		So(called, ShouldBeTrue)

	})

	resetFunctions()
	Convey("Should call adapters.HandleGetById", t, func() {

		var called bool
		adapters.HandleGetById = func (m *adapters.MongoAdapter, requestWrapper messages.RequestWrapper) (response map[string]interface{}, err error) {
			called = true
			return
		}

		var actor Actor
		actor.actorType = ActorTypeObject
		_, err := handleGet(&actor, messages.RequestWrapper{})
		So(err, ShouldBeNil)
		So(called, ShouldBeTrue)

	})

	resetFunctions()
	Convey("Should call adapters.HandleGet", t, func() {

		var called bool
		adapters.HandleGet = func (m *adapters.MongoAdapter, requestWrapper messages.RequestWrapper) (response map[string]interface{}, err error) {
			called = true
			return
		}

		var actor Actor
		actor.actorType = ActorTypeResource
		_, err := handleGet(&actor, messages.RequestWrapper{})
		So(err, ShouldBeNil)
		So(called, ShouldBeTrue)

	})

}

func resetFunctions() {
	handleGet = originalHandleGet
}