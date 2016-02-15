package adapters

import (
	"io"
	"fmt"
	"time"
	"reflect"
	"net/http"
	"encoding/json"
	"encoding/base64"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"github.com/eluleci/dock/config"
	"github.com/eluleci/dock/utils"
)

type MongoAdapter struct {
	Collection *mgo.Collection
}

var MongoDB *mgo.Database
var Database string
var Session *mgo.Session

var Connect = func(config config.Config) (err *utils.Error) {

	addresses, hasAddress := config.Mongo["addresses"]
	if !hasAddress {
		err = &utils.Error{http.StatusInternalServerError, "Database 'addresses' must be specified in configuration file."};
		return
	}

	addressesAsStrings := make([]string, len(addresses.([]interface{})))
	for i, v := range addresses.([]interface{}) {addressesAsStrings[i] = v.(string)}

	database, hasDatabase := config.Mongo["database"]
	if !hasDatabase {
		err = &utils.Error{http.StatusInternalServerError, "Database name must be specified in 'database' in configuration file."};
		return
	}
	Database = database.(string)

	info := &mgo.DialInfo{
		Addrs:    addressesAsStrings,
		Database: Database,
	}

	authDatabase, hasAuthDatabase := config.Mongo["authDatabase"]
	if hasAuthDatabase {
		info.Database = authDatabase.(string)
	}

	username, hasUsername := config.Mongo["username"]
	if hasUsername {
		info.Username = username.(string)
	}

	password, hasPassword := config.Mongo["password"]
	if hasPassword {
		info.Password = password.(string)
	}

	var dialErr error
	Session, dialErr = mgo.DialWithInfo(info)
	if dialErr != nil {
		err = &utils.Error{http.StatusInternalServerError, "Database connection failed: " + dialErr.Error()};
		return
	}

	// TODO: find a proper way to close the session
	// defer session.Close()

	MongoDB = Session.DB(Database)
	return

}

var Create = func(collection string, data map[string]interface{}) (response map[string]interface{}, hookBody map[string]interface{}, err *utils.Error) {

	sessionCopy := Session.Copy()
	defer sessionCopy.Close()
	connection := sessionCopy.DB(Database).C(collection)

	// generate id and createdAt time
	id := bson.NewObjectId()
	createdAt := int32(time.Now().Unix())

	// additional fields
	data["_id"] = id.Hex()
	data["createdAt"] = createdAt
	data["updatedAt"] = createdAt

	insertError := connection.Insert(data)
	if insertError != nil {
		err = &utils.Error{http.StatusInternalServerError, "Inserting item to database failed."};
		return
	}

	response = map[string]interface{}{
		"_id": id.Hex(),
		"createdAt": createdAt,
	}
	hookBody = data

	return
}

var Get = func(collection string, id string) (response map[string]interface{}, err *utils.Error) {

	sessionCopy := Session.Copy()
	defer sessionCopy.Close()
	connection := sessionCopy.DB(Database).C(collection)

	response = make(map[string]interface{})

	getErr := connection.FindId(id).One(&response)
	if getErr != nil {
		err = &utils.Error{http.StatusNotFound, "Item not found."};
		response = nil
		return
	}
	return
}

var Query = func(collection string, parameters map[string][]string) (response map[string]interface{}, err *utils.Error) {

	sessionCopy := Session.Copy()
	defer sessionCopy.Close()
	connection := sessionCopy.DB(Database).C(collection)

	response = make(map[string]interface{})

	if parameters["aggregate"] != nil && parameters["where"] != nil {
		err = &utils.Error{http.StatusBadRequest, "Where and aggregate parameters cannot be used at the same request."}
		return
	}

	var results []map[string]interface{}
	var getErr error

	whereParam, hasWhereParam, whereParamErr := extractJsonParameter(parameters, "where")
	aggregateParam, hasAggregateParam, aggregateParamErr := extractJsonParameter(parameters, "aggregate")
	sortParam, hasSortParam, sortParamErr := extractStringParameter(parameters, "sort")
	limitParam, _, limitParamErr := extractIntParameter(parameters, "limit")
	skipParam, _, skipParamErr := extractIntParameter(parameters, "skip")

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
		getErr = connection.Pipe(aggregateParam).All(&results)
	} else {
		query := connection.Find(whereParam).Skip(skipParam).Limit(limitParam)
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

var Update = func(collection string, id string, data map[string]interface{}) (response map[string]interface{}, hookBody map[string]interface{}, err *utils.Error) {

	sessionCopy := Session.Copy()
	defer sessionCopy.Close()
	connection := sessionCopy.DB(Database).C(collection)

	if data == nil {
		err = &utils.Error{http.StatusBadRequest, "Request body cannot be empty for update requests."}
		return
	}

	data["updatedAt"] = int32(time.Now().Unix())

	objectToUpdate := make(map[string]interface{})
	findErr := connection.FindId(id).One(&objectToUpdate)
	if findErr != nil {
		err = &utils.Error{http.StatusNotFound, "Item not found."};
		return
	}

	// updating the fields that request body contains
	for k, v := range data {
		objectToUpdate[k] = v
	}

	updateErr := connection.UpdateId(id, objectToUpdate)
	if updateErr != nil {
		err = &utils.Error{http.StatusInternalServerError, "Update request to db failed."};
		return
	}

	response = map[string]interface{}{
		"updatedAt": data["updatedAt"],
	}
	hookBody = map[string]interface{}{
		"_id": id,
		"updatedAt": data["updatedAt"],
	}

	// add the updated fields to the hook body
	for k, v := range data {
		hookBody[k] = v
	}
	return
}

var Delete = func(collection string, id string) (response map[string]interface{}, err *utils.Error) {

	sessionCopy := Session.Copy()
	defer sessionCopy.Close()
	connection := sessionCopy.DB(Database).C(collection)

	removeErr := connection.RemoveId(id)
	if removeErr != nil {
		err = &utils.Error{http.StatusNotFound, "Item not found."};
	}
	return
}

var CreateFile = func(data io.ReadCloser) (response map[string]interface{}, hookBody map[string]interface{}, err *utils.Error) {

	sessionCopy := Session.Copy()
	defer sessionCopy.Close()

	objectId := bson.NewObjectId()
	now := time.Now()
	fileName := objectId.Hex()

	gridFile, mongoErr := sessionCopy.DB(Database).GridFS("fs").Create(fileName)
	if mongoErr != nil {
		fmt.Println(mongoErr)
		err = &utils.Error{http.StatusInternalServerError, "Creating file failed."}
		return
	}
	gridFile.SetId(fileName)
	gridFile.SetName(fileName)
	gridFile.SetUploadDate(now)

	dec := base64.NewDecoder(base64.StdEncoding, data)
	_, copyErr := io.Copy(gridFile, dec)
	if copyErr != nil {
		fmt.Println(copyErr)
		err = &utils.Error{http.StatusInternalServerError, "Writing file failed."}
		return
	}

	closeErr := gridFile.Close()
	if closeErr != nil {
		fmt.Println(closeErr)
		err = &utils.Error{http.StatusInternalServerError, "Closing file failed."}
		return
	}

	response = make(map[string]interface{})
	response["_id"] = fileName
	response["createdAt"] = int32(now.Unix())
	hookBody = response
	return

	return
}

var GetFile = func(id string) (response []byte, err *utils.Error) {

	sessionCopy := Session.Copy()
	defer sessionCopy.Close()

	file, mongoErr := sessionCopy.DB(Database).GridFS("fs").OpenId(id)
	if mongoErr != nil {
		err = &utils.Error{http.StatusNotFound, "File not found."};
		return
	}

	response = make([]byte, file.Size())
	_, printErr := file.Read(response)
	if printErr != nil {
		err = &utils.Error{http.StatusInternalServerError, "Printing file failed."};
	}
	return
}

var extractJsonParameter = func(parameters map[string][]string, key string) (value interface{}, hasParam bool, err *utils.Error) {

	var paramArray []string
	paramArray, hasParam = parameters[key]

	if hasParam {
		parseErr := json.Unmarshal([]byte(paramArray[0]), &value)
		if parseErr != nil {
			fmt.Println(parseErr)
			err = &utils.Error{http.StatusBadRequest, "Parsing " + key + " parameter failed."}
		}
	}
	return
}

var extractStringParameter = func(parameters map[string][]string, key string) (value string, hasParam bool, err *utils.Error) {

	var paramArray []string
	paramArray, hasParam = parameters[key]

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

var extractIntParameter = func(parameters map[string][]string, key string) (value int, hasParam bool, err *utils.Error) {

	var paramArray []string
	paramArray, hasParam = parameters[key]

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
