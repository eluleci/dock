package actors

import (
	"github.com/eluleci/dock/utils"
	"github.com/eluleci/dock/messages"
	"github.com/eluleci/dock/adapters"
	"github.com/eluleci/dock/auth"
	"encoding/json"
	"strings"
	"net/http"
	"github.com/eluleci/dock/modifier"
	"github.com/eluleci/dock/hooks"
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

type Actor struct {
	res         string
	level       int
	actorType   string
	class       string
	model       map[string]interface{}
	children    map[string]Actor
	Inbox       chan messages.RequestWrapper
	parentInbox chan messages.RequestWrapper
	adapter     *adapters.MongoAdapter
}

var RootActor Actor

var CreateActor = func(res string, level int, parentInbox chan messages.RequestWrapper) (a Actor) {

	var isFunctionActor bool
	var className string
	if strings.EqualFold(res, ResourceLogin) || strings.EqualFold(res, ResourceRegister) || strings.EqualFold(res, ResourceResetPassword) || strings.EqualFold(res, ResourceChangePassword) {
		className = ClassUsers
	} else {
		resParts := strings.Split(res, "/")
		resourceLevel := resParts[level]
		isFunctionActor = strings.HasPrefix(resourceLevel, "-")
		if len(resParts) > 1 {
			className = resParts[1]
		}
	}

	if isFunctionActor {
		a.actorType = ActorTypeFunctions
	} else if level == 0 {
		a.actorType = ActorTypeRoot
	} else if level == 1 {
		a.actorType = ActorTypeCollection
	} else if level == 2 {
		a.actorType = ActorTypeModel
	} else if level == 3 {
		a.actorType = ActorTypeAttribute
	}

	a.res = res
	a.level = level
	a.class = className
	a.children = make(map[string]Actor)
	a.Inbox = make(chan messages.RequestWrapper)
	a.parentInbox = parentInbox
	a.adapter = &adapters.MongoAdapter{adapters.MongoDB.C(className)}

	return
}

func retrieveClassName(res string, level int) (string) {
	if level == 0 {
		return ""
	} else if level == 1 {
		return res[1:]
	} else if level == 2 {
		return res[1:strings.LastIndex(res, "/")]
	}
	return ""

	// TODO: return class names of more complicated resources like: /Post/123/Author (return User)
}

func (a *Actor) Run() {
	defer func() {
		utils.Log("debug", a.res + ":  Stopped running.")
	}()

	utils.Log("debug", a.res + ": Started running")

	for {
		select {
		case requestWrapper := <-a.Inbox:

			if requestWrapper.Res == a.res {
				// if the resource of the message is this actor's resource

				messageString, _ := json.Marshal(requestWrapper.Message)
				utils.Log("debug", a.res + ": " + string(messageString))

				response := handleRequest(a, requestWrapper)
				a.checkAndSend(requestWrapper.Listener, response)
				utils.Log("debug", "")

				// TODO stop the actor if it belongs to an item and the item is deleted
				// TODO stop the actor if it belongs to an item and the item doesn't exist
				// TODO stop the actor if it belongs to an entity and 'get' returns an empty array (not sure though)

			} else {
				// if the resource belongs to a children actor
				childRes := getChildRes(requestWrapper.Res, a.res)

				actor, exists := a.children[childRes]

				if !exists {
					// if children doesn't exists, create a child actor for the res
					actor = CreateActor(childRes, a.level + 1, a.Inbox)
					go actor.Run()
					a.children[childRes] = actor
				}
				//   forward message to the children actor
				actor.Inbox <- requestWrapper
			}
		}
	}
}

var handleRequest = func(a *Actor, requestWrapper messages.RequestWrapper) (response messages.Message) {

	var isGranted bool
	var user map[string]interface{}
	var err *utils.Error
	var hookBody map[string]interface{}

	// TODO check for not allowed commands on resources. for ex: DELETE /topics, POST /users/123

	//	isActorTypeFunctions := strings.EqualFold(a.actorType, ActorTypeFunctions)

	isGranted, user, err = auth.IsGranted(requestWrapper, a.adapter)

	if isGranted && err == nil {
		response, err = executeTrigger(a, user, requestWrapper, "before")
		if response.Body != nil {
			// replace request body with the one that hook server returns
			requestWrapper.Message.Body = response.Body
		}
	}

	if err != nil {
		// skip below. status code is set at the end of the function
	} else if !isGranted {
		err = &utils.Error{http.StatusUnauthorized, "Unauthorized."}
	} else if (strings.EqualFold(a.actorType, ActorTypeFunctions)) {
		response, err = executeFunction(a, user, requestWrapper)
	} else if strings.EqualFold(requestWrapper.Message.Command, "get") {
		response, err = handleGet(a, requestWrapper)
	} else if strings.EqualFold(requestWrapper.Message.Command, "post") {
		response, hookBody, err = handlePost(a, requestWrapper, user)
	} else if strings.EqualFold(requestWrapper.Message.Command, "put") {
		response, hookBody, err = handlePut(a, requestWrapper)
	} else if strings.EqualFold(requestWrapper.Message.Command, "delete") {
		response, err = handleDelete(a, requestWrapper)
	}

	if err != nil {
		if response.Status == 0 {response.Status = err.Code}
		if response.Body == nil {response.Body = map[string]interface{}{"message":err.Message}}
	}

	// TODO: call hooks.ExecuteTrigger in goroutine
	var hookRequestWrapper = messages.RequestWrapper{}
	hookRequestWrapper.Message = requestWrapper.Message
	hookRequestWrapper.Message.Body = hookBody
	executeTrigger(a, user, hookRequestWrapper, "after")
	return
}

var executeTrigger = func(a *Actor, user interface{}, requestWrapper messages.RequestWrapper, when string) (response messages.Message, err *utils.Error) {

	response.Body, err = hooks.ExecuteTrigger(a.class, when,
		requestWrapper.Message.Command,
		requestWrapper.Message.Parameters,
		requestWrapper.Message.Body,
		requestWrapper.Message.MultipartForm,
		user)
	if err != nil {
		if response.Body == nil {
			response.Body = map[string]interface{}{"message": err.Message}
		}
		return
	}
	return
}

var executeFunction = func(a *Actor, user interface{}, requestWrapper messages.RequestWrapper) (response messages.Message, err *utils.Error) {

	response.Body, err = hooks.ExecuteFunction(a.res,
		requestWrapper.Message.Parameters,
		requestWrapper.Message.Body,
		user)

	if err != nil {
		if response.Body == nil {
			response.Body = map[string]interface{}{"message": err.Message}
		}
		return
	}
	return
}

var handleGet = func(a *Actor, requestWrapper messages.RequestWrapper) (response messages.Message, err *utils.Error) {

	isFileClass := strings.EqualFold(a.class, ClassFiles)
	isObjectTypeActor := strings.EqualFold(a.actorType, ActorTypeModel)
	isCollectionTypeActor := strings.EqualFold(a.actorType, ActorTypeCollection)

	if isObjectTypeActor {
		id := requestWrapper.Message.Res[strings.LastIndex(requestWrapper.Message.Res, "/") + 1:]
		if isFileClass {    // get file by id
			response.RawBody, err = adapters.GetFile(id)
		} else {            // get object by id
			response.Body, err = adapters.HandleGetById(a.adapter, requestWrapper)
		}
	} else if isCollectionTypeActor {                    // query objects
		response.Body, err = adapters.HandleGet(a.adapter, requestWrapper)
	}

	if err != nil {
		return
	}

	if requestWrapper.Message.Parameters["expand"] != nil {
		expandConfig := requestWrapper.Message.Parameters["expand"][0]
		if _, hasDataArray := response.Body["data"]; hasDataArray {
			response.Body, err = modifier.ExpandArray(response.Body, expandConfig)
		} else {
			response.Body, err = modifier.ExpandItem(response.Body, expandConfig)
		}
		if err != nil {
			return
		}
	}

	response.Body = filterFields(a, response.Body)
	return
}

var handlePost = func(a *Actor, requestWrapper messages.RequestWrapper, user interface{}) (response messages.Message, hookBody map[string]interface{}, err *utils.Error) {

	if strings.EqualFold(a.res, ResourceRegister) {                                // sign up request
		response, hookBody, err = auth.HandleSignUp(requestWrapper, a.adapter)
	} else if strings.EqualFold(a.res, ResourceLogin) {                            // login request
		response, err = auth.HandleLogin(requestWrapper, a.adapter)
	} else if strings.EqualFold(a.res, ResourceChangePassword) {                   // reset password
		response, err = auth.HandleChangePassword(requestWrapper, a.adapter, user)
	} else if strings.EqualFold(a.res, ResourceResetPassword) {                    // reset password
		response, err = auth.HandleResetPassword(requestWrapper, a.adapter)
	} else if strings.EqualFold(a.res, ResourceTypeUsers) {                        // post on users not allowed
		response.Status = http.StatusMethodNotAllowed
	} else if strings.EqualFold(a.actorType, ActorTypeCollection) {                // create object request
		response.Body, hookBody, err = adapters.HandlePost(a.adapter, requestWrapper)
		if err == nil {response.Status = http.StatusCreated}
	} else if strings.EqualFold(a.actorType, ActorTypeModel) {                    // post on objects are not allowed
		response.Status = http.StatusBadRequest
	}
	return
}

var handlePut = func(a *Actor, requestWrapper messages.RequestWrapper) (response messages.Message, hookBody map[string]interface{}, err *utils.Error) {

	if strings.EqualFold(a.actorType, ActorTypeCollection) {            // put on resources are not allowed
		response.Status = http.StatusBadRequest
	} else if strings.EqualFold(a.actorType, ActorTypeModel) {        // update object
		response.Body, hookBody, err = adapters.HandlePut(a.adapter, requestWrapper)
	}
	return
}

var handleDelete = func(a *Actor, requestWrapper messages.RequestWrapper) (response messages.Message, err *utils.Error) {

	if strings.EqualFold(a.actorType, ActorTypeCollection) {            // delete on resources are not allowed
		response.Status = http.StatusBadRequest
	} else if strings.EqualFold(a.actorType, ActorTypeModel) {        // delete object
		response.Body, err = adapters.HandleDelete(a.adapter, requestWrapper)
		if err == nil {
			response.Status = http.StatusNoContent
		}
	}
	return
}

func (a *Actor) checkAndSend(c chan messages.Message, m messages.Message) {
	defer func() {
		if r := recover(); r != nil {
			utils.Log("debug", a.res + "Trying to send on closed channel.")
		}
	}()
	c <- m
}

func getChildRes(res, parentRes string) (fullPath string) {
	res = strings.Trim(res, "/")
	parentRes = strings.Trim(parentRes, "/")
	currentResSize := len(parentRes)
	resSuffix := res[currentResSize:]
	trimmedSuffix := strings.Trim(resSuffix, "/")
	directChild := strings.Split(trimmedSuffix, "/")
	relativePath := directChild[0]
	if len(parentRes) > 0 {
		fullPath = "/" + parentRes + "/" + relativePath
	} else {
		fullPath = "/" + relativePath
	}
	return
}

func filterFields(a *Actor, object map[string]interface{}) map[string]interface{} {

	// filters 'password' fields of user objects
	// TODO make a generic filter engine that takes the filter config from the configuration file such as:
	//
	//	{
	//		filterOptions: {
	//			"/users": ["password"],
	//			"/comments": ["score"]
	//		},
	//		triggersUrl: "api2.miwi.com"
	//	}
	//
	if strings.EqualFold(a.res, ResourceTypeUsers) {
		users := object["data"]
		for _, user := range users.([]map[string]interface{}) {
			delete(user, "password")
		}
	} else {
		if strings.EqualFold(a.class, ClassUsers) {
			delete(object, "password")
		}
	}

	return object
}