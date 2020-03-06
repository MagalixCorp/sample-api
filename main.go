package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/gomodule/redigo/redis"
	"github.com/gorilla/mux"
)

var redisHostWrite string
var redisHostRead string
var redisPort string
var redisPassword string

const redisKey = "msg-key"

//PostData a struct for holding the the data sent in the request
type PostData struct {
	Message string `json:"message"`
}

func setEnv() {
	if redisHostWrite = os.Getenv("REDIS_MASTER_HOST"); redisHostWrite == "" {
		redisHostWrite = "localhost"
	}
	if redisHostRead = os.Getenv("REDIS_SLAVE_HOST"); redisHostRead == "" {
		redisHostRead = "localhost"
	}
	if redisPort = os.Getenv("REDIS_PORT"); redisPort == "" {
		redisPort = "6379"
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
	log.Fatal("Application is running", http.ListenAndServe(":3000", router))
}

func handlePost(w http.ResponseWriter, r *http.Request) {
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Println("Error while reading the request body:", err)
	}
	var newData PostData
	json.Unmarshal(b, &newData)
	data := getData()
	data = append(data, newData)
	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Println("Error while marhsalling the data:", err)
	}
	pool := newPool(true)
	conn := pool.Get()
	defer conn.Close()
	err = set(conn, string(jsonData))
	if err != nil {
		log.Println("Error while saving record to Redis:", err)
	}
	w.WriteHeader(204)
}

func getData() []PostData {
	var pd []PostData
	pool := newPool(false)
	conn := pool.Get()
	defer conn.Close()
	content, _ := get(conn)
	err := json.Unmarshal([]byte(content), &pd)
	if err != nil {
		log.Println(err)
	}
	return pd
}

func handleQuery(w http.ResponseWriter, r *http.Request) {
	data := getData()
	// data, err := json.Marshal(getData())
	// if err != nil {
	json.NewEncoder(w).Encode(data)
	// } else {
	// 	log.Println(err)
	// 	respondWithError(w, "An error occured", 500)
	// }
}

func respondWithError(w http.ResponseWriter, msg string, status int) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"message": msg})
}

func newPool(write bool) *redis.Pool {
	// We need to set the Redis connection settings for testing functions individually (not passing through main() function)
	setEnv()
	var redisHost string
	if write {
		redisHost = redisHostWrite
	} else {
		redisHost = redisHostRead
	}
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
func set(c redis.Conn, value string) error {
	_, err := c.Do("AUTH", redisPassword)
	if err != nil {
		log.Println("Could not authenticate to Redis", err)
	}
	_, err = c.Do("SET", redisKey, value)
	if err != nil {
		return err
	}
	return nil
}

// get executes the redis GET command
func get(c redis.Conn) (string, error) {
	_, err := c.Do("AUTH", redisPassword)
	if err != nil {
		log.Println("Could not authenticate to Redis", err)
	}
	s, err := redis.String(c.Do("GET", redisKey))
	if err != nil {
		log.Println(err)
		return "", err
	}
	return s, nil
}

func commonMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}
