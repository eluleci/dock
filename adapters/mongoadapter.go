package adapters

import (
	"github.com/eluleci/dock/messages"
	"github.com/eluleci/dock/utils"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"encoding/json"
	"time"
	"strings"
	"net/http"
)

type MongoAdapter struct {
	Collection    *mgo.Collection
}

var MongoDB *mgo.Database

var HandlePost = func (m *MongoAdapter, requestWrapper messages.RequestWrapper) (response map[string]interface{}, err *utils.Error) {

	message := requestWrapper.Message

	objectId := bson.NewObjectId()
	createdAt := int32(time.Now().Unix())

	// additional fields
	message.Body["_id"] = objectId
	message.Body["createdAt"] = createdAt
	message.Body["updatedAt"] = createdAt

	insertError := m.Collection.Insert(message.Body)
	if insertError != nil {
		err = &utils.Error{http.StatusInternalServerError, "Inserting item to database failed."};
		return
	}

	response = make(map[string]interface{})
	response["_id"] = objectId
	response["createdAt"] = createdAt

	return
}

var HandleGetById = func (m *MongoAdapter, requestWrapper messages.RequestWrapper) (response map[string]interface{}, err *utils.Error) {

	message := requestWrapper.Message
	id := message.Res[strings.LastIndex(message.Res, "/")+1:]
	response = make(map[string]interface{})

	getErr := m.Collection.FindId(bson.ObjectIdHex(id)).One(&response)
	if getErr != nil {
		err = &utils.Error{http.StatusNotFound, "Item not found."};
		response = nil
		return
	}
	return
}

var HandleGet = func (m *MongoAdapter, requestWrapper messages.RequestWrapper) (response map[string]interface{}, err *utils.Error) {

	message := requestWrapper.Message

	response = make(map[string]interface{})

	var results []map[string]interface {}

	var whereParams map[string]interface{}
	if message.Parameters["where"] != nil {
		json.Unmarshal([]byte(message.Parameters["where"][0]), &whereParams)
	}

	findErr := m.Collection.Find(whereParams).All(&results)
	if findErr != nil {
		err = &utils.Error{http.StatusInternalServerError, "Querying items failed."};
		return
	}

	if results != nil {
		response["data"] = results
	} else {
		response["data"] = make([]map[string]interface {}, 0)
	}
	return
}

var HandlePut = func (m *MongoAdapter, requestWrapper messages.RequestWrapper) (response map[string]interface{}, err *utils.Error) {

	message := requestWrapper.Message
	message.Body["updatedAt"] = int32(time.Now().Unix())
	id := message.Res[strings.LastIndex(message.Res, "/")+1:]

	objectToUpdate := make(map[string]interface{})
	findErr := m.Collection.FindId(bson.ObjectIdHex(id)).One(&objectToUpdate)
	if findErr != nil {
		err = &utils.Error{http.StatusNotFound, "Item not found."};
		return
	}

	// updating the fields that request body contains
	for k, v := range message.Body {
		objectToUpdate[k] = v
	}

	updateErr := m.Collection.UpdateId(bson.ObjectIdHex(id), objectToUpdate)
	if updateErr != nil {
		err = &utils.Error{http.StatusInternalServerError, "Update request to db failed."};
		return
	}

	response = make(map[string]interface{})
	response["updatedAt"] = message.Body["updatedAt"]

	return
}

var HandleDelete = func (m *MongoAdapter, requestWrapper messages.RequestWrapper) (response map[string]interface{}, err *utils.Error) {

	message := requestWrapper.Message
	id := message.Res[strings.LastIndex(message.Res, "/")+1:]

	removeErr := m.Collection.RemoveId(bson.ObjectIdHex(id))
	if removeErr != nil {
		err = &utils.Error{http.StatusNotFound, "Item not found."};
		return
	}

	return
}
