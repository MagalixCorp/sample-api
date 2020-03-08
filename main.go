package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/gomodule/redigo/redis"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

var redisHost string
var redisPort string
var redisPassword string

//PostData a struct for holding the the data sent in the request
type PostData struct {
	Username string `json:"username"`
	Message  string `json:"message"`
}

//Configuration is a struct for holding the application configuration data read from a JSON file
type Configuration struct {
	RedisHost string
	RedisPort string
}

func setEnv() {
	//filename is the path to the json config file
	file, err := os.Open("config.json")
	if err != nil {
		log.Fatal(err)
	}
	decoder := json.NewDecoder(file)
	var configuration Configuration
	err = decoder.Decode(&configuration)
	if err != nil {
		log.Fatal(err)
	}
	if redisHost = os.Getenv("REDIS_HOST"); redisHost == "" {
		if redisHost = configuration.RedisHost; redisHost == "" {
			redisHost = "localhost"
		}
	}
	if redisPort = os.Getenv("REDIS_PORT"); redisPort == "" {
		if redisPort = configuration.RedisPort; redisPort == "" {
			redisPort = "6379"
		}
	}
	redisPassword = os.Getenv("REDIS_PASSWORD")
}

func newServer() http.Handler {
	r := mux.NewRouter().StrictSlash(true)
	r.Use(commonMiddleware)
	r.HandleFunc("/api", handlePost).Methods("POST")
	r.HandleFunc("/api", handleQuery).Methods("GET")
	return r
}

func main() {
	setEnv()
	var router = newServer()
	log.Println("Server starting on port 3000")
	// log.Fatal("Application is running", http.ListenAndServe(":3000", router))
	log.Fatal(http.ListenAndServe(":3000", handlers.CORS(handlers.AllowedHeaders([]string{"X-Requested-With", "Content-Type", "Authorization"}), handlers.AllowedMethods([]string{"GET", "POST", "PUT", "HEAD", "OPTIONS"}), handlers.AllowedOrigins([]string{"*"}))(router)))
}

func handlePost(w http.ResponseWriter, r *http.Request) {
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Println("Error while reading the request body:", err)
	}
	var newData PostData
	json.Unmarshal(b, &newData)
	username := newData.Username
	message := newData.Message
	if err != nil {
		log.Println("Error while marhsalling the data:", err)
	}
	pool := newPool(true)
	conn := pool.Get()
	defer conn.Close()
	err = set(conn, username, message)
	if err != nil {
		log.Println("Error while saving record to Redis:", err)
	}
	w.WriteHeader(204)
}

func handleQuery(w http.ResponseWriter, r *http.Request) {
	pool := newPool(false)
	conn := pool.Get()
	defer conn.Close()
	content, _ := get(conn)
	json.NewEncoder(w).Encode(content)
}

func respondWithError(w http.ResponseWriter, msg string, status int) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"message": msg})
}

func newPool(write bool) *redis.Pool {
	// We need to set the Redis connection settings for testing functions individually (not passing through main() function)
	setEnv()
	return &redis.Pool{
		// Maximum number of idle connections in the pool.
		MaxIdle: 80,
		// max number of connections
		MaxActive: 12000,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", redisHost+":"+redisPort)
			if err != nil {
				log.Println("Could not reach Redis", err)
			}
			_, err = c.Do("AUTH", redisPassword)
			if err != nil {
				log.Println("Could not authenticate to Redis", err)
			}
			return c, err
		},
	}
}
func set(c redis.Conn, key string, value string) error {
	_, err := c.Do("AUTH", redisPassword)
	if err != nil {
		log.Println("Could not authenticate to Redis", err)
	}
	_, err = c.Do("SET", key, value)
	if err != nil {
		return err
	}
	return nil
}

// get executes the redis GET command
func get(c redis.Conn) ([]PostData, error) {
	results := []PostData{}
	_, err := c.Do("AUTH", redisPassword)
	if err != nil {
		log.Println("Could not authenticate to Redis", err)
	}
	keys, err := redis.Strings(c.Do("KEYS", "*"))
	if err != nil {
		return results, err
	}
	for _, k := range keys {
		if v, err := redis.String(c.Do("GET", k)); err == nil {
			results = append(results, PostData{k, v})
		}
	}
	return results, nil
}

func commonMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}
