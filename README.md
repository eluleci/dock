# Dock

Dock is a backend as a service which works with an existing MongoDB server. 

[![wercker status](https://app.wercker.com/status/f65df5ac48e79fe90fa5bfcb9a7e17a6/s "wercker status")](https://app.wercker.com/project/bykey/f65df5ac48e79fe90fa5bfcb9a7e17a6)

## Configuration
Configurations, such as database, are provided with **dock-config.json** file and that file is required while booting up. Example:

```
{
  "mongo": {
    "address":  "localhost",
    "name":     "dock-db"
  }
}
```


## Running the server

### With Docker

There is a Docker image that contains the server at **eluleci/dock** in [Docker Hub](docker pull yourusername/docker-whale) so you can pull and run the image with this command.

`docker run --publish 8080:1707 --name Dock eluleci/dock`

It will exit with a config error. You must provide a **dock-config.json** file to the container you just built. Locate the configuration file and run

`docker cp dock-config.json CONTAINER_ID:/go/dock-config.json`

The configuration file in your host device will be copied to the container. Then run

`docker start CONTAINER_ID` 

to start the server.

Check the server is running more then 10 seconds. Server tries to connect to the database many times and quits after some time with an error message. If the container stops working you can check the logs to see the reason.


### Building from source

Clone the repo, place the configuration file in the root folder and run **main.go**. If error occurs during boot up then the process will exit with a descriptive error message.

## Documentation

### Objects

#### Create object

There is no need for defining the classes or object fields for creating objects. You can directly make a POST request to the class endpoint that you want the create the object.

```
WARNING: There are some system classes such as 'users' and 'roles'. So creating objects under reserved resources are not allowed.
```

Example:

**Request**

```
POST /topics
{
	"title": "This is a topic title",
	"tags": ["topic", "subject"]
}
```

**Response**

```
201 Created
{
	"_id": "564f1a28e63bce219e1cc745",
	"createdAt": 987239623
}
```

#### Get object

**Request**

```
GET /topics/564f1a28e63bce219e1cc745
```

**Response**

```
200 OK
{
	"_id": "564f1a28e63bce219e1cc745",
	"createdAt": 987239623,
	"updatedAt": 987239623,
	"title": "This is a topic title",
	"tags": ["topic", "subject"]
}
```

#### Query objects

Listing all the objects.

**Request**

```
GET /topics
```

Querying objects requires a url parameter called **where** and it should be encoded json. That parameter will be used to query the MongoDB so you can check the [Mongo documentation](https://docs.mongodb.org/manual/tutorial/query-documents/) to see how to construct the json.

**Request**

```
GET /topics?where={"createdAt":{"$gte":987239623}}
```

#### Update object

Only the provided fields will be updated. The other fields will remain same.

**Request**

```
PUT /topics/564f1a28e63bce219e1cc745
{
	"title": "This is another topic title"
}
```

**Response**

```
200 OK
{
	"updatedAt": 987239623
}
```

#### Delete object

**Request**

```
DELETE /topics/564f1a28e63bce219e1cc745
```

**Response**

```
200 OK
```

### Registration

#### Sign up with email

**Request**

```
POST /register
{
	"email": "johny@bravo.com",
	"password": "ihaveamazinghair"
}
```

**Response**

```
{
  "_id": "5660236795fc151444e53f69",
  "accessToken": "eyJhbGciOi.eyJleHAiOjE0NDk0MDAyOTU.Xa1tUvYgI_YqdA",
  "createdAt": 1449141095
}
```

#### Sign up with username

**Request**

```
POST /register
{
	"username": "johnybravo",
	"password": "ihaveamazinghair"
}
```

**Response**

```
{
  "_id": "5660236795fc151444e53f69",
  "accessToken": "eyJhbGciOi.eyJleHAiOjE0NDk0MDAyOTU.Xa1tUvYgI_YqdA",
  "createdAt": 1449141095
}
```

#### Sign up with Facebook

**Request**

```
POST /register
{
  "facebook": {
    "id": "10153102991889648",
    "accessToken": "CAAOPotl9EWoBAPeLlTcQWAEUjZB3SoJG2UCHh1cpf2Q5Ow3jWLfu8zndhH82Jaax8JFlijClzy0mos4f7ZCUotJxPr7ZAcUdJUk2bKGTqx6kbrq2FuMQgnZC8vXDGZCDTrWY5k5vWfGTw8zqBmti607L4soEQt8xnZCdgZCj7k45ElIDZCufbWxdiB1Q9hRFZA1ATTYeiZCE3NZBKbmQCJEHL5r"
  }
}
```

**Response**

```
{
  "_id": "5660236795fc151444e53f69",
  "accessToken": "eyJhbGciOi.eyJleHAiOjE0NDk0MDAyOTU.Xa1tUvYgI_YqdA",
  "createdAt": 1449141095,
  "isNewUser": true
}
```

### Login

#### Login with email

**Request**

```
GET /login?email=johny@bravo.com&password=ihaveamazinghair
```

**Response**

```
{
  "_id": "566022ba95fc1514392fe69c",
  "accessToken": "eyJhbGciOiJI.eyJleHAiOjE0NDk0MTIzOTYsInVzZXIiOnsidXNlc.Juubl8V_xRC9y1srp",
  "createdAt": 1449140922,
  "updatedAt": 1449140922,
  "email": "johny@bravo.com"
}
```

#### Login with username

```
GET /login?username=johnybravo&password=ihaveamazinghair
```

```
{
  "_id": "566022ba95fc1514392fe69c",
  "accessToken": "eyJhbGciOiJI.eyJleHAiOjE0NDk0MTIzOTYsInVzZXIiOnsidXNlc.Juubl8V_xRC9y1srp",
  "createdAt": 1449140922,
  "updatedAt": 1449140922,
  "username": "johnybravo"
}
```

#### Login with Facebook

**Request**

```
POST /register
{
  "facebook": {
    "id": "10153102991889648",
    "accessToken": "CAAOPotl9EWoBAPeLlTcQWAEUjZB3SoJG2UCHh1cpf2Q5Ow3jWLfu8zndhH82Jaax8JFlijClzy0mos4f7ZCUotJxPr7ZAcUdJUk2bKGTqx6kbrq2FuMQgnZC8vXDGZCDTrWY5k5vWfGTw8zqBmti607L4soEQt8xnZCdgZCj7k45ElIDZCufbWxdiB1Q9hRFZA1ATTYeiZCE3NZBKbmQCJEHL5r"
  }
}
```

**Response**

```
{
  "_id": "5660236795fc151444e53f69",
  "accessToken": "eyJhbGciOi.eyJleHAiOjE0NDk0MDAyOTU.Xa1tUvYgI_YqdA",
  "createdAt": 1449141095,
  "updatedAt": 1449141095,
  "isNewUser": false
}
```
