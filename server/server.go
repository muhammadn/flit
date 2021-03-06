package main

import (
  "log"
  "context"
  "encoding/json"
  "io/ioutil"
  "time"
  _ "fmt"
  "github.com/r3labs/sse/v2"
  "net/http"
  "goji.io"
  "goji.io/pat"
  "github.com/go-redis/redis/v8"
  "github.com/jinzhu/configor"
)

var Config = struct {
        Redis_Host string `required:"true"`
}{}

func sendPubSub(w http.ResponseWriter, r *http.Request) {
        if err := configor.Load(&Config, "config.yaml"); err != nil {
                panic(err)
        }
     
        redisClient := redis.NewClient(&redis.Options{
                Addr:     Config.Redis_Host,  // We connect to host redis, thats what the hostname of the redis service is set to in the docker-compose
                Password: "", // The password IF set in the redis Config file
                DB:       0,
        })

        bodyBytes, _ := ioutil.ReadAll(r.Body)
        bodyString := string(bodyBytes)

        err := redisClient.Ping(context.Background()).Err()
        if err != nil {
                // Sleep for 3 seconds and wait for Redis to initialize
                time.Sleep(3 * time.Second)
                err := redisClient.Ping(context.Background()).Err()
                if err != nil {
                        panic(err)
                }
        }
        // Generate a new background context that  we will use
        ctx := context.Background()

        redisClient.Publish(ctx, "action_commands", bodyString).Err()
}


func sendMessage(server *sse.Server, data string) {
    log.Println("message: %s", data)

    server.Publish("messages", &sse.Event{
      Data: []byte(data),
    })
}

func main() {
  server := sse.New()
  server.CreateStream("messages")
  server.EncodeBase64 = true

  mux := goji.NewMux()
  mux.HandleFunc(pat.Get("/events"), server.HTTPHandler)
  mux.HandleFunc(pat.Post("/update"), sendPubSub)
  mux.HandleFunc(pat.Post("/ping"), sendPubSub)

  go func() {
	// Create a new Redis Client
	redisClient := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",  // We connect to host redis, thats what the hostname of the redis service is set to in the docker-compose
		Password: "", // The password IF set in the redis Config file
		DB:       0,
	})
	// Ping the Redis server and check if any errors occured
	err := redisClient.Ping(context.Background()).Err()
	if err != nil {
		// Sleep for 3 seconds and wait for Redis to initialize
		time.Sleep(3 * time.Second)
		err := redisClient.Ping(context.Background()).Err()
		if err != nil {
			panic(err)
		}
	}

	ctx := context.Background()
	// Subscribe to the Topic given
	topic := redisClient.Subscribe(ctx, "action_commands")
	// Get the Channel to use
	channel := topic.Channel()
	// Itterate any messages sent on the channel
	for msg := range channel {
                sendMessage(server, string(msg.Payload))
	}
  }()

  log.Print("Started server")
  http.ListenAndServe(":8080", mux)
}

// User is a struct representing newly registered users
type User struct {
	Username string
	Email    string
}

// MarshalBinary encodes the struct into a binary blob
// Here I cheat and use regular json :)
func (u *User) MarshalBinary() ([]byte, error) {
	return json.Marshal(u)
}

// UnmarshalBinary decodes the struct into a User
func (u *User) UnmarshalBinary(data []byte) error {
	if err := json.Unmarshal(data, u); err != nil {
		return err
	}
	return nil
}

