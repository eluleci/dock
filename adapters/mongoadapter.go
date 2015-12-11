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
	"fmt"
	"reflect"
)

type MongoAdapter struct {
	Collection *mgo.Collection
}

var MongoDB *mgo.Database

var HandlePost = func(m *MongoAdapter, requestWrapper messages.RequestWrapper) (response map[string]interface{}, err *utils.Error) {

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

var HandleGetById = func(m *MongoAdapter, requestWrapper messages.RequestWrapper) (response map[string]interface{}, err *utils.Error) {

	message := requestWrapper.Message
	id := message.Res[strings.LastIndex(message.Res, "/") + 1:]
	response = make(map[string]interface{})

	getErr := m.Collection.FindId(bson.ObjectIdHex(id)).One(&response)
	if getErr != nil {
		err = &utils.Error{http.StatusNotFound, "Item not found."};
		response = nil
		return
	}
	return
}

var HandleGet = func(m *MongoAdapter, requestWrapper messages.RequestWrapper) (response map[string]interface{}, err *utils.Error) {

	message := requestWrapper.Message

	response = make(map[string]interface{})

	if message.Parameters["aggregate"] != nil && message.Parameters["where"] != nil {
		err = &utils.Error{http.StatusBadRequest, "Where and aggregate parameters cannot be used at the same request."}
		return
	}

	var results []map[string]interface{}
	var getErr error

	whereParam, hasWhereParam, whereParamErr := extractJsonParameter(message, "where")
	aggregateParam, hasAggregateParam, aggregateParamErr := extractJsonParameter(message, "aggregate")
	sortParam, hasSortParam, sortParamErr := extractStringParameter(message, "sort")
	limitParam, _, limitParamErr := extractIntParameter(message, "limit")
	skipParam, _, skipParamErr := extractIntParameter(message, "skip")

	if aggregateParamErr != nil {err = aggregateParamErr}
	if whereParamErr != nil {err = whereParamErr}
	if sortParamErr != nil {err = sortParamErr}
	if limitParamErr != nil {err = limitParamErr}
	if skipParamErr != nil {err = skipParamErr}
	if err != nil {return}

	if hasWhereParam && hasAggregateParam {
		err = &utils.Error{http.StatusInternalServerError, "Aggregation cannot be used with where parameter."};
		return
	}

	if hasAggregateParam {
		getErr = m.Collection.Pipe(aggregateParam).All(&results)
	} else {
		query := m.Collection.Find(whereParam).Skip(skipParam).Limit(limitParam)
		if hasSortParam {
			query = query.Sort(sortParam)
		}
		getErr = query.All(&results)
	}

	if getErr != nil {
		err = &utils.Error{http.StatusInternalServerError, "Getting items failed."};
		fmt.Println(getErr)
		return
	}

	if results != nil {
		response["data"] = results
	} else {
		response["data"] = make([]map[string]interface{}, 0)
	}
	return
}

var HandlePut = func(m *MongoAdapter, requestWrapper messages.RequestWrapper) (response map[string]interface{}, err *utils.Error) {

	message := requestWrapper.Message
	if message.Body == nil {
		err = &utils.Error{http.StatusBadRequest, "Request body cannot be empty for update requests."}
		return
	}

	message.Body["updatedAt"] = int32(time.Now().Unix())
	id := message.Res[strings.LastIndex(message.Res, "/") + 1:]

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

var HandleDelete = func(m *MongoAdapter, requestWrapper messages.RequestWrapper) (response map[string]interface{}, err *utils.Error) {

	message := requestWrapper.Message
	id := message.Res[strings.LastIndex(message.Res, "/") + 1:]

	removeErr := m.Collection.RemoveId(bson.ObjectIdHex(id))
	if removeErr != nil {
		err = &utils.Error{http.StatusNotFound, "Item not found."};
		return
	}

	return
}

var extractJsonParameter = func(message messages.Message, key string) (value interface{}, hasParam bool, err *utils.Error) {

	var paramArray []string
	paramArray, hasParam = message.Parameters[key]

	if hasParam {
		parseErr := json.Unmarshal([]byte(paramArray[0]), &value)
		if parseErr != nil {
			fmt.Println(parseErr)
			err = &utils.Error{http.StatusBadRequest, "Parsing " + key + " parameter failed."}
		}
	}
	return
}

var extractStringParameter = func(message messages.Message, key string) (value string, hasParam bool, err *utils.Error) {

	var paramArray []string
	paramArray, hasParam = message.Parameters[key]

	if hasParam {
		var paramValue interface{}
		parseErr := json.Unmarshal([]byte(paramArray[0]), &paramValue)
		if parseErr != nil {
			fmt.Println(parseErr)
			err = &utils.Error{http.StatusBadRequest, "Parsing " + key + " parameter failed."}
		}

		fieldType := reflect.TypeOf(paramValue)
		fmt.Println(fieldType)

		if fieldType == nil || fieldType.Kind() != reflect.String {
			value = ""
			err = &utils.Error{http.StatusBadRequest, "The key '" + key + "' must be a valid string."}
			return
		}
		value = paramValue.(string)
	}
	return
}

var extractIntParameter = func(message messages.Message, key string) (value int, hasParam bool, err *utils.Error) {

	var paramArray []string
	paramArray, hasParam = message.Parameters[key]

	if hasParam {
		var paramValue interface{}
		parseErr := json.Unmarshal([]byte(paramArray[0]), &paramValue)
		if parseErr != nil {
			fmt.Println(parseErr)
			err = &utils.Error{http.StatusBadRequest, "Parsing " + key + " parameter failed."}
		}

		fieldType := reflect.TypeOf(paramValue)
		fmt.Println(fieldType)

		if fieldType == nil || fieldType.Kind() != reflect.Float64 {
			value = 0
			err = &utils.Error{http.StatusBadRequest, "The key '" + key + "' must be an integer."}
			return
		}
		value = int(paramValue.(float64))
	}
	return
}