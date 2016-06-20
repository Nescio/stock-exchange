package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"

	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/negroni"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/thoas/stats"
	"github.com/unrolled/render"
)

func handleInterrupts(ch chan os.Signal) {
	signal.Notify(ch, os.Interrupt)
	go func() {
		for sig := range ch {
			fmt.Printf("Exiting... %v\n", sig)
			ch = nil
			os.Exit(1)
		}
	}()
}

func main() {
	handleInterrupts(make(chan os.Signal, 1))

	// create the router
	router := mux.NewRouter()

	const APIBase = "/api/v1/"

	router.HandleFunc(APIBase+"healthcheck", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, http.StatusOK)
	}).Methods("HEAD")

	statsMiddleware := stats.New()
	router.HandleFunc(APIBase+"stats", func(w http.ResponseWriter, r *http.Request) {
		stats := statsMiddleware.Data()
		renderJSON(w, stats)
	}).Methods("GET")

	router.HandleFunc("/ws", createWebsocketHandler()).Methods("GET")

	n := negroni.New(
		negroni.NewRecovery(),
		negroni.NewLogger(),
		statsMiddleware,
		negroni.NewStatic(http.Dir("public")),
	)
	n.UseHandler(router)
	n.Run(":8888")
}

var ren = render.New()

func renderJSON(w http.ResponseWriter, v interface{}) {
	ren.JSON(w, http.StatusOK, v)
}

func createWebsocketHandler() func(w http.ResponseWriter, r *http.Request) {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		log.WithField("url", redisURL).Fatal("missing REDIS_URL in env")
	}
	redisPool, err := newRedisPool(redisURL)
	if err != nil {
		log.WithField("url", redisURL).Fatal("Unable to create Redis pool")
	}

	channelName := "reporter"
	rr := newRedisReceiver(redisPool, channelName)
	go rr.run()

	rw := newRedisWriter(redisPool, channelName)
	go rw.run()

	var upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	return func(w http.ResponseWriter, r *http.Request) {

		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.WithField("err", err).Println("Upgrading to websockets")
			http.Error(w, "Error Upgrading to websockets", 400)
			return
		}
		defer ws.WriteMessage(websocket.CloseMessage, []byte{})

		id := rr.register(ws)
		defer rr.deRegister(id)

		for {
			mt, data, err := ws.ReadMessage()
			ctx := log.Fields{"mt": mt, "data": data, "err": err}

			if err != nil {
				if err == io.EOF {
					log.WithFields(ctx).Info("Websocket closed!")
				} else {
					log.WithFields(ctx).Error("Error reading websocket message")
				}
				break
			}

			switch mt {
			case websocket.TextMessage: // BinaryMessage, CloseMessage, PingMessage, PongMessage
				msg, err := validateMessage(data)
				if err != nil {
					ctx["msg"] = msg
					ctx["err"] = err
					log.WithFields(ctx).Error("Invalid Message")
					break
				}
				rw.publish(data)
			default:
				log.WithFields(ctx).Warning("Unknown Message!")
			}
		}
	}
}

func validateMessage(data []byte) (string, error) {
	return "", nil
}
