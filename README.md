# Dock

Web server has built in REST API that works with existing Mongo server.

[![wercker status](https://app.wercker.com/status/f65df5ac48e79fe90fa5bfcb9a7e17a6/s "wercker status")](https://app.wercker.com/project/bykey/f65df5ac48e79fe90fa5bfcb9a7e17a6)

## Usage

Dock, currently, doesn't have storage system in it. It is only a web interface for a RESTful API. So it requires a MongoDB instance up and running.

Dock requires a `dock-config.json` file to boot.

Assuming **Go** is installed, clone the repo and run **main.go** with `go run main.go` command. 

## Documentation

### Registration

#### Sign up with email

```
POST
/register
{
	"email": "",
	"password": ""
}
```

#### Sign up with username

```
POST
/register
{
	"username": "",
	"password": ""
}
```

#### Login with email
