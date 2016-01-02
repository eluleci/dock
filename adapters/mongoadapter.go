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
	"github.com/eluleci/dock/config"
	"bufio"
	"mime/multipart"
	"io"
	"errors"
	"encoding/base64"
	"net/url"
)

type MongoAdapter struct {
	Collection *mgo.Collection
}

var MongoDB *mgo.Database

var Connect = func(config config.Config) (err *utils.Error) {

	address, hasAddress := config.Mongo["address"]
	name, hasName := config.Mongo["name"]
	if !hasAddress || !hasName {
		err = &utils.Error{http.StatusInternalServerError, "Database 'address' and 'name' must be specified in dock-config.json."};
		return
	}

	session, mongoerr := mgo.Dial(address)
	if mongoerr != nil {
		err = &utils.Error{http.StatusInternalServerError, "Database connection failed."};
		return
	}

	// TODO: find a proper way to close the session
	// defer session.Close()

	MongoDB = session.DB(name)
	return

}

var HandlePost = func(m *MongoAdapter, requestWrapper messages.RequestWrapper) (response map[string]interface{}, hookBody map[string]interface{}, err *utils.Error) {

	if strings.EqualFold("/files", requestWrapper.Res) {

		if requestWrapper.Message.MultipartForm == nil {    // data is in the body

			objectId := bson.NewObjectId()
			now := time.Now()
			fileName := objectId.Hex()

			gridFile, mongoErr := MongoDB.GridFS("fs").Create(fileName)
			if mongoErr != nil {
				fmt.Println(mongoErr)
				err = &utils.Error{http.StatusInternalServerError, "Creating file failed."}
				return
			}
			gridFile.SetId(fileName)
			gridFile.SetName(fileName)
			gridFile.SetUploadDate(now)

			dec := base64.NewDecoder(base64.StdEncoding, requestWrapper.Message.ReqBodyRaw)
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
		} else {

			if len(requestWrapper.Message.MultipartForm.File) > 1 {
				err = &utils.Error{http.StatusBadRequest, "Only one file can be uploaded with one request."}
				return
			}

			for _, fileHeaders := range requestWrapper.Message.MultipartForm.File {
				for _, fileHeader := range fileHeaders {

					objectId := bson.NewObjectId()
					now := time.Now()

					file, _ := fileHeader.Open()

					// unescaping first because the name is escaped in a bad way.
					fileName, _ := url.QueryUnescape(fileHeader.Filename)
					fileName = strings.Replace(fileName, "/", "", -1)
					fileName = strings.Replace(fileName, " ", "", -1)
					fileName = strings.Replace(fileName, ":", "", -1)
					fileName = url.QueryEscape(fileName)
					fileName = objectId.Hex() + "-" + fileName
					gridFile, mongoErr := MongoDB.GridFS("fs").Create(fileName)
					if mongoErr != nil {
						err = &utils.Error{http.StatusInternalServerError, "Creating file failed."}
						return
					}

					gridFile.SetId(fileName)
					gridFile.SetUploadDate(now)
					gridFile.SetName(fileHeader.Filename)
					if writeErr := writeToGridFile(file, gridFile); writeErr != nil {
						err = &utils.Error{http.StatusInternalServerError, "Writing file failed."}
						return
					}

					response = make(map[string]interface{})
					response["_id"] = fileName
					response["createdAt"] = int32(now.Unix())
					hookBody = response
					return
				}
			}
		}

	} else {

		message := requestWrapper.Message

		objectId := bson.NewObjectId()
		createdAt := int32(time.Now().Unix())

		// additional fields
		message.Body["_id"] = objectId.Hex()
		message.Body["createdAt"] = createdAt
		message.Body["updatedAt"] = createdAt

		insertError := m.Collection.Insert(message.Body)
		if insertError != nil {
			err = &utils.Error{http.StatusInternalServerError, "Inserting item to database failed."};
			return
		}

		response = make(map[string]interface{})
		response["_id"] = objectId.Hex()
		response["createdAt"] = createdAt
		hookBody = message.Body
	}

	return
}

var HandleGetById = func(m *MongoAdapter, requestWrapper messages.RequestWrapper) (response map[string]interface{}, err *utils.Error) {

	if strings.Contains(requestWrapper.Res, "/files") {



	} else {
		message := requestWrapper.Message
		id := message.Res[strings.LastIndex(message.Res, "/") + 1:]
		response = make(map[string]interface{})

		getErr := m.Collection.FindId(id).One(&response)
		if getErr != nil {
			err = &utils.Error{http.StatusNotFound, "Item not found."};
			response = nil
			return
		}
	}
	return
}

var GetFile = func(id string) (response []byte, err *utils.Error) {

	file, mongoErr := MongoDB.GridFS("fs").OpenId(id)
	if mongoErr != nil {
		fmt.Println(mongoErr)
		err = &utils.Error{http.StatusNotFound, "File not found."};
		return
	}

	response = make([]byte, file.Size())
	_, printErr := file.Read(response)
	if printErr != nil {
		fmt.Println(printErr)
		err = &utils.Error{http.StatusInternalServerError, "Printing file failed."};
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
	findErr := m.Collection.FindId(id).One(&objectToUpdate)
	if findErr != nil {
		err = &utils.Error{http.StatusNotFound, "Item not found."};
		return
	}

	// updating the fields that request body contains
	for k, v := range message.Body {
		objectToUpdate[k] = v
	}

	updateErr := m.Collection.UpdateId(id, objectToUpdate)
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

	removeErr := m.Collection.RemoveId(id)
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

var writeToGridFile = func(file multipart.File, gridFile *mgo.GridFile) error {
	reader := bufio.NewReader(file)
	defer func() { file.Close() }()
	// make a buffer to keep chunks that are read
	buf := make([]byte, 1024)
	for {
		// read a chunk
		n, err := reader.Read(buf)
		if err != nil && err != io.EOF {
			return errors.New("Could not read the input file")
		}
		if n == 0 {
			break
		}
		// write a chunk
		if _, err := gridFile.Write(buf[:n]); err != nil {
			return errors.New("Could not write to GridFs for " + gridFile.Name())
		}
	}
	gridFile.Close()
	return nil
}

var writeBodyToGridFile = func(body io.ReadCloser, gridFile *mgo.GridFile) error {
	reader := bufio.NewReader(body)
	defer func() { body.Close() }()
	// make a buffer to keep chunks that are read
	buf := make([]byte, 1024)
	for {
		// read a chunk
		n, err := reader.Read(buf)
		if err != nil && err != io.EOF {
			return errors.New("Could not read the input file")
		}
		if n == 0 {
			break
		}
		// write a chunk
		if _, err := gridFile.Write(buf[:n]); err != nil {
			return errors.New("Could not write to GridFs for " + gridFile.Name())
		}
	}
	gridFile.Close()
	return nil
}