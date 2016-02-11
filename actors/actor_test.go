package actors

import (
	"testing"
	"github.com/eluleci/dock/auth"
	"github.com/eluleci/dock/messages"
	"github.com/eluleci/dock/adapters"
	"github.com/eluleci/dock/utils"
	. "github.com/smartystreets/goconvey/convey"
	"net/http"
	"os"
	"strings"
	"gopkg.in/mgo.v2"
	"github.com/eluleci/dock/hooks"
	"mime/multipart"
)

var _handleRequest = func(a *Actor, requestWrapper messages.RequestWrapper) (response messages.Message) {
	return
}

var _handleGet = func(a *Actor, requestWrapper messages.RequestWrapper) (response messages.Message, err *utils.Error) {
	return
}

var _handlePost = func(a *Actor, requestWrapper messages.RequestWrapper, user interface{}) (response messages.Message, hookBody map[string]interface{}, err *utils.Error) {
	return
}

var _handlePut = func(a *Actor, requestWrapper messages.RequestWrapper) (response messages.Message, hookBody map[string]interface{}, err *utils.Error) {
	return
}

var _handleDelete = func(a *Actor, requestWrapper messages.RequestWrapper) (response messages.Message, err *utils.Error) {
	return
}

var _CreateActor = func(res string, level int, parentInbox chan messages.RequestWrapper) (a Actor) {
	return
}

func TestMain(m *testing.M) {
	saveRealFunctions()
	os.Exit(m.Run())
}

func saveRealFunctions() {
	_CreateActor = CreateActor
	_handleRequest = handleRequest
	_handleGet = handleGet
	_handlePost = handlePost
	_handlePut = handlePut
	_handleDelete = handleDelete

}

func resetFunctions() {
	CreateActor = _CreateActor
	handleRequest = _handleRequest
	handleGet = _handleGet
	handlePost = _handlePost
	handlePut = _handlePut
	handleDelete = _handleDelete
}

func TestInbox(t *testing.T) {

	Convey("Should call actor.handleRequest", t, func() {
		var called bool
		handleRequest = func(a *Actor, requestWrapper messages.RequestWrapper) (response messages.Message) {
			called = true
			return
		}

		var requestWrapper messages.RequestWrapper
		requestWrapper.Res = "/"
		responseChannel := make(chan messages.Message)
		requestWrapper.Listener = responseChannel

		var actor Actor
		actor.class = "someclass"
		actor.res = "/"
		actor.Inbox = make(chan messages.RequestWrapper)
		go actor.Run()
		actor.Inbox <- requestWrapper
		response := <-responseChannel

		So(response, ShouldNotBeNil)
		So(called, ShouldBeTrue)
	})

	Convey("Should forward message to a child actor", t, func() {
		parentRes := "/users"
		childRes := "/users/123"

		var calledOnParent bool
		var calledOnChild bool
		handleRequest = func(a *Actor, requestWrapper messages.RequestWrapper) (response messages.Message) {
			if strings.EqualFold(a.res, parentRes) {
				calledOnParent = true
			}
			if strings.EqualFold(a.res, childRes) {
				calledOnChild = true
			}
			return
		}

		var requestWrapper messages.RequestWrapper
		requestWrapper.Res = childRes
		responseChannel := make(chan messages.Message)
		requestWrapper.Listener = responseChannel

		CreateActor = func(res string, level int, parentInbox chan messages.RequestWrapper) (a Actor) {
			a.res = childRes
			a.level = 2
			a.Inbox = make(chan messages.RequestWrapper)
			return
		}

		var actor Actor
		actor.res = parentRes
		actor.level = 1
		actor.children = make(map[string]Actor)
		actor.Inbox = make(chan messages.RequestWrapper)
		go actor.Run()
		actor.Inbox <- requestWrapper
		response := <-responseChannel

		So(response, ShouldNotBeNil)
		So(calledOnParent, ShouldBeFalse)
		So(calledOnChild, ShouldBeTrue)
	})
}

func TestCreateActor(t *testing.T) {

	resetFunctions()
	Convey("Should create actor", t, func() {
		adapters.MongoDB = &mgo.Database{}
		actor := CreateActor("/", 0, nil)
		So(actor.res, ShouldEqual, "/");
		So(actor.actorType, ShouldEqual, ActorTypeRoot);
	})
	Convey("Should create actor for register", t, func() {
		adapters.MongoDB = &mgo.Database{}
		actor := CreateActor(ResourceRegister, 0, nil)
		So(actor.res, ShouldEqual, ResourceRegister);
		So(actor.class, ShouldEqual, ClassUsers);
	})
	Convey("Should create actor for login", t, func() {
		adapters.MongoDB = &mgo.Database{}
		actor := CreateActor(ResourceLogin, 0, nil)
		So(actor.res, ShouldEqual, ResourceLogin);
		So(actor.class, ShouldEqual, ClassUsers);
	})
	Convey("Should create actor for collection", t, func() {
		adapters.MongoDB = &mgo.Database{}
		actor := CreateActor("/comments", 1, nil)
		So(actor.class, ShouldEqual, "comments");
	})
	Convey("Should create actor for object", t, func() {
		adapters.MongoDB = &mgo.Database{}
		actor := CreateActor("/comments/123", 2, nil)
		So(actor.class, ShouldEqual, "comments");
	})
}

func TestHandleRequest(t *testing.T) {

	hooks.ExecuteTrigger = func(className, when, method string,
	parameters map[string][]string, body map[string]interface{}, multipart *multipart.Form,
	user interface{}) (responseBody map[string]interface{}, err *utils.Error) {
		return
	}

	isGrantedFuncThatReturnsTrue := func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (isGranted bool, user map[string]interface{}, err *utils.Error) {
		isGranted = true
		return
	}

	isGrantedFuncThatReturnsFalse := func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (isGranted bool, user map[string]interface{}, err *utils.Error) {
		isGranted = false
		return
	}

	resetFunctions()
	Convey("Should call auth.GetPermissions", t, func() {

		var called bool
		auth.IsGranted = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (isGranted bool, user map[string]interface{}, err *utils.Error) {
			called = true
			return
		}

		actor := &Actor{}
		actor.class = "someclass"
		handleRequest(actor, messages.RequestWrapper{})
		So(called, ShouldBeTrue)
	})

	Convey("Should return permission error", t, func() {

		auth.IsGranted = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (isGranted bool, user map[string]interface{}, err *utils.Error) {
			err = &utils.Error{http.StatusInternalServerError, ""}
			return
		}

		actor := &Actor{}
		actor.class = "someclass"
		response := handleRequest(actor, messages.RequestWrapper{})
		So(response.Status, ShouldEqual, http.StatusInternalServerError)
	})

	/////////////////////////
	// GET
	/////////////////////////
	resetFunctions()
	Convey("Should call handleGet", t, func() {

		auth.IsGranted = isGrantedFuncThatReturnsTrue

		var called bool
		handleGet = func(a *Actor, requestWrapper messages.RequestWrapper) (response messages.Message, err *utils.Error) {
			called = true
			return
		}

		var m messages.Message
		m.Command = "get"

		var rw messages.RequestWrapper
		rw.Message = m

		actor := &Actor{}
		actor.class = "someclass"
		handleRequest(actor, rw)
		So(called, ShouldBeTrue)
	})

	Convey("Should return Authorization error for GET", t, func() {

		auth.IsGranted = isGrantedFuncThatReturnsFalse

		var m messages.Message
		m.Command = "get"

		var rw messages.RequestWrapper
		rw.Message = m

		actor := &Actor{}
		actor.class = "someclass"
		response := handleRequest(actor, rw)
		So(response.Status, ShouldEqual, http.StatusUnauthorized)

	})

	/////////////////////////
	// POST
	/////////////////////////
	Convey("Should call handlePost", t, func() {

		auth.IsGranted = isGrantedFuncThatReturnsTrue

		var called bool
		handlePost = func(a *Actor, requestWrapper messages.RequestWrapper, user interface{}) (response messages.Message, hookBody map[string]interface{}, err *utils.Error) {
			called = true
			return
		}

		var m messages.Message
		m.Command = "post"

		var rw messages.RequestWrapper
		rw.Message = m

		actor := &Actor{}
		actor.class = "someclass"
		handleRequest(actor, rw)
		So(called, ShouldBeTrue)
	})

	Convey("Should return Authorization error for POST", t, func() {

		auth.IsGranted = isGrantedFuncThatReturnsFalse

		var m messages.Message
		m.Command = "post"

		var rw messages.RequestWrapper
		rw.Message = m

		actor := &Actor{}
		actor.class = "someclass"
		response := handleRequest(actor, rw)
		So(response.Status, ShouldEqual, http.StatusUnauthorized)

	})

	/////////////////////////
	// PUT
	/////////////////////////
	Convey("Should call handlePut", t, func() {

		auth.IsGranted = isGrantedFuncThatReturnsTrue

		var called bool
		handlePut = func(a *Actor, requestWrapper messages.RequestWrapper) (response messages.Message, hookBody map[string]interface{}, err *utils.Error) {
			called = true
			return
		}

		var m messages.Message
		m.Command = "put"
		var rw messages.RequestWrapper
		rw.Message = m

		actor := &Actor{}
		actor.class = "someclass"
		handleRequest(actor, rw)
		So(called, ShouldBeTrue)
	})

	Convey("Should return Authorization error for PUT", t, func() {

		auth.IsGranted = isGrantedFuncThatReturnsFalse

		var m messages.Message
		m.Command = "put"

		var rw messages.RequestWrapper
		rw.Message = m

		actor := &Actor{}
		actor.class = "someclass"
		response := handleRequest(actor, rw)
		So(response.Status, ShouldEqual, http.StatusUnauthorized)
	})

	/////////////////////////
	// DELETE
	/////////////////////////
	Convey("Should call handleDelete", t, func() {

		auth.IsGranted = isGrantedFuncThatReturnsTrue

		var called bool
		handleDelete = func(a *Actor, requestWrapper messages.RequestWrapper) (response messages.Message, err *utils.Error) {
			called = true
			return
		}

		var m messages.Message
		m.Command = "delete"
		var rw messages.RequestWrapper
		rw.Message = m

		actor := &Actor{}
		actor.class = "someclass"
		handleRequest(actor, rw)
		So(called, ShouldBeTrue)
	})

	Convey("Should call handleDelete and return error", t, func() {

		auth.IsGranted = isGrantedFuncThatReturnsTrue

		handleDelete = func(a *Actor, requestWrapper messages.RequestWrapper) (response messages.Message, err *utils.Error) {
			err = &utils.Error{http.StatusNotFound, "Item not found."};
			return
		}

		var m messages.Message
		m.Command = "delete"
		var rw messages.RequestWrapper
		rw.Message = m

		actor := &Actor{}
		actor.class = "someclass"
		response := handleRequest(actor, rw)
		So(response.Status, ShouldEqual, http.StatusNotFound)
	})

	Convey("Should return Authorization error for DELETE", t, func() {

		auth.IsGranted = isGrantedFuncThatReturnsFalse

		var m messages.Message
		m.Command = "delete"

		var rw messages.RequestWrapper
		rw.Message = m

		actor := &Actor{}
		actor.class = "someclass"
		response := handleRequest(actor, rw)
		So(response.Status, ShouldEqual, http.StatusUnauthorized)
	})
}

func TestHandleGet(t *testing.T) {

	resetFunctions()
	Convey("Should call auth.HandleLogin", t, func() {

		var called bool
		auth.HandleLogin = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (response messages.Message, err *utils.Error) {
			called = true
			return
		}

		var actor Actor
		actor.res = ResourceLogin
		_, _, err := handlePost(&actor, messages.RequestWrapper{}, nil)
		So(err, ShouldBeNil)
		So(called, ShouldBeTrue)

	})

	resetFunctions()
	Convey("Should call adapters.HandleGetById", t, func() {

		var called bool
		adapters.HandleGetById = func(m *adapters.MongoAdapter, requestWrapper messages.RequestWrapper) (response map[string]interface{}, err *utils.Error) {
			called = true
			return
		}

		var actor Actor
		actor.actorType = ActorTypeModel
		_, err := handleGet(&actor, messages.RequestWrapper{})
		So(err, ShouldBeNil)
		So(called, ShouldBeTrue)

	})

	resetFunctions()
	Convey("Should call adapters.HandleGet", t, func() {

		var called bool
		adapters.HandleGet = func(m *adapters.MongoAdapter, requestWrapper messages.RequestWrapper) (response map[string]interface{}, err *utils.Error) {
			called = true
			return
		}

		var actor Actor
		actor.actorType = ActorTypeCollection
		_, err := handleGet(&actor, messages.RequestWrapper{})
		So(err, ShouldBeNil)
		So(called, ShouldBeTrue)

	})

}

func TestHandlePost(t *testing.T) {

	resetFunctions()
	Convey("Should return method not allowed", t, func() {

		var actor Actor
		actor.res = ResourceTypeUsers
		response, _, _ := handlePost(&actor, messages.RequestWrapper{}, nil)
		So(response.Status, ShouldEqual, http.StatusMethodNotAllowed)

	})

	Convey("Should call auth.HandleSignUp", t, func() {

		var actor Actor
		actor.res = ResourceRegister

		var called bool
		auth.HandleSignUp = func(requestWrapper messages.RequestWrapper, dbAdapter *adapters.MongoAdapter) (response messages.Message, hookBody map[string]interface{}, err *utils.Error) {
			called = true
			return
		}

		_, _, err := handlePost(&actor, messages.RequestWrapper{}, nil)
		So(err, ShouldBeNil)
		So(called, ShouldBeTrue)

	})

	Convey("Should call auth.HandleSignUp", t, func() {

		var actor Actor
		actor.actorType = ActorTypeCollection

		var called bool
		adapters.HandlePost = func(m *adapters.MongoAdapter, requestWrapper messages.RequestWrapper) (response map[string]interface{}, hookBody map[string]interface{}, err *utils.Error) {
			called = true
			return
		}

		response, _, err := handlePost(&actor, messages.RequestWrapper{}, nil)
		So(err, ShouldBeNil)
		So(called, ShouldBeTrue)
		So(response.Status, ShouldEqual, http.StatusCreated)
	})

	Convey("Should return bad request", t, func() {

		var actor Actor
		actor.actorType = ActorTypeModel

		response, _, err := handlePost(&actor, messages.RequestWrapper{}, nil)
		So(err, ShouldBeNil)
		So(response.Status, ShouldEqual, http.StatusBadRequest)
	})
}

func TestHandlePut(t *testing.T) {

	resetFunctions()

	Convey("Should return bad request", t, func() {

		var actor Actor
		actor.actorType = ActorTypeCollection

		response, _, err := handlePut(&actor, messages.RequestWrapper{})
		So(err, ShouldBeNil)
		So(response.Status, ShouldEqual, http.StatusBadRequest)
	})

	Convey("Should call auth.HandleSignUp", t, func() {

		var actor Actor
		actor.actorType = ActorTypeModel

		var called bool
		adapters.HandlePut = func(m *adapters.MongoAdapter, requestWrapper messages.RequestWrapper) (response map[string]interface{}, hookBody map[string]interface{}, err *utils.Error) {
			called = true
			return
		}

		_, _, err := handlePut(&actor, messages.RequestWrapper{})
		So(err, ShouldBeNil)
		So(called, ShouldBeTrue)
	})
}

func TestHandleDelete(t *testing.T) {

	resetFunctions()
	Convey("Should return bad request", t, func() {

		var actor Actor
		actor.actorType = ActorTypeCollection

		response, err := handleDelete(&actor, messages.RequestWrapper{})
		So(err, ShouldBeNil)
		So(response.Status, ShouldEqual, http.StatusBadRequest)
	})
	Convey("Should call auth.HandleSignUp", t, func() {

		var actor Actor
		actor.actorType = ActorTypeModel

		var called bool
		adapters.HandleDelete = func(m *adapters.MongoAdapter, requestWrapper messages.RequestWrapper) (response map[string]interface{}, err *utils.Error) {
			called = true
			return
		}

		_, err := handleDelete(&actor, messages.RequestWrapper{})
		So(err, ShouldBeNil)
		So(called, ShouldBeTrue)
	})
}

func TestGetChildRes(t *testing.T) {

	Convey("Should return correct res of the child", t, func() {
		So(getChildRes("/users", "/"), ShouldEqual, "/users")
		So(getChildRes("/users/", "/"), ShouldEqual, "/users")
		So(getChildRes("/users/123", "/"), ShouldEqual, "/users")
		So(getChildRes("/users/123", "/users"), ShouldEqual, "/users/123")
		So(getChildRes("/users/123/", "/users/"), ShouldEqual, "/users/123")
	})
}

func TestRetrieveClassName(t *testing.T) {

	Convey("Should return correct class name", t, func() {
		So(retrieveClassName("/", 0), ShouldEqual, "")
		So(retrieveClassName("/users", 1), ShouldEqual, "users")
		So(retrieveClassName("/users/123", 2), ShouldEqual, "users")
		So(retrieveClassName("/users/123", 3), ShouldEqual, "")
	})
}