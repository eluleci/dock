package config

type Config struct {

	/* MongoDB configuration. Used for connecting to database. Available fields:
	 * address:	IP of the MongoDB server			(required)
	 * name:	Name of the database on the server	(required)
	 */
	Mongo        map[string]string `json:"mongo,omitempty"`

	/* Facebook configuration. Used for registration with Facebook account. Fields:
	 * appId:		Facebook app id	(required)
	 * appToken:	Facebook app token (required)
	 */
	Facebook     map[string]string `json:"facebook,omitempty"`

}

var SystemConfig Config
