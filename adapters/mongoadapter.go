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

func (m *MongoAdapter) HandlePost(requestWrapper messages.RequestWrapper) (response map[string]interface{}, err error) {

	message := requestWrapper.Message

	objectId := bson.NewObjectId()
	createdAt := int32(time.Now().Unix())

	// additional fields
	message.Body["_id"] = objectId
	message.Body["createdAt"] = createdAt
	message.Body["updatedAt"] = createdAt

	err = m.Collection.Insert(message.Body)
	if err != nil {
		err = &utils.Error{http.StatusInternalServerError, "Inserting item failed."};
		return
	}

	response = make(map[string]interface{})
	response["objectId"] = objectId
	response["createdAt"] = createdAt

	return
}

func (m *MongoAdapter) HandleGetById(requestWrapper messages.RequestWrapper) (response map[string]interface{}, err error) {

	message := requestWrapper.Message
	id := message.Res[strings.LastIndex(message.Res, "/")+1:]
	response = make(map[string]interface{})

	err = m.Collection.FindId(bson.ObjectIdHex(id)).One(&response)
	if err != nil {
		utils.Log("fatal", "Getting item with id failed")
		err = &utils.Error{http.StatusNotFound, "Item not found."};
		return
	}
	return
}

func (m *MongoAdapter) HandleGet(requestWrapper messages.RequestWrapper) (response map[string]interface{}, err error) {

	message := requestWrapper.Message

	response = make(map[string]interface{})

	var results []map[string]interface {}

	var whereParams map[string]interface {}
	json.Unmarshal([]byte(message.Parameters["where"][0]), &whereParams)

	err = m.Collection.Find(whereParams).All(&results)
	if err != nil {
		utils.Log("fatal", "Querying items failed")
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

func (m *MongoAdapter) HandlePut(requestWrapper messages.RequestWrapper) (response map[string]interface{}, err error) {

	message := requestWrapper.Message
	message.Body["updatedAt"] = int32(time.Now().Unix())
	id := message.Res[strings.LastIndex(message.Res, "/")+1:]

	objectToUpdate := make(map[string]interface{})
	err = m.Collection.FindId(bson.ObjectIdHex(id)).One(&objectToUpdate)
	if err != nil {
		utils.Log("fatal", "Getting item with id failed")
		err = &utils.Error{http.StatusNotFound, "Item not found."};
		return
	}

	// updating the fields that request body contains
	for k, v := range message.Body {
		objectToUpdate[k] = v
	}

	err = m.Collection.UpdateId(bson.ObjectIdHex(id), objectToUpdate)
	if err != nil {
		utils.Log("fatal", "Updating item failed")
		err = &utils.Error{http.StatusNotFound, "Item not found."};
		return
	}

	response = make(map[string]interface{})
	response["updatedAt"] = message.Body["updatedAt"]

	return
}

func (m *MongoAdapter) HandleDelete(requestWrapper messages.RequestWrapper) (response map[string]interface{}, err error) {

	message := requestWrapper.Message
	id := message.Res[strings.LastIndex(message.Res, "/")+1:]

	err = m.Collection.RemoveId(bson.ObjectIdHex(id))
	if err != nil {
		utils.Log("fatal", "Deleting item failed")
		err = &utils.Error{http.StatusNotFound, "Item not found."};
		return
	}

	return
}
