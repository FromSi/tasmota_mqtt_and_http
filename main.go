package main

import (
	"context"
	"encoding/json"
	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/mochi-mqtt/server/v2/packets"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

const (
	MqttSignalPowerOn = iota
	MqttSignalPowerOff
	MqttSignalPowerToggle
	MqttSignalAllStatus
)

func main() {
	sigs := make(chan os.Signal, 1)
	mqttSigs := make(chan int, 1)
	mqttStatus := make(chan string, 1)

	ctx, cancel := context.WithCancel(context.Background())

	wg := &sync.WaitGroup{}

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigs
		cancel()
	}()

	wg.Add(2)

	go serverMQTT(ctx, wg, mqttSigs, mqttStatus)
	go serverHTTP(ctx, wg, mqttSigs, mqttStatus)

	wg.Wait()
}

func serverMQTT(ctx context.Context, wg *sync.WaitGroup, mqttSigs chan int, mqttStatus chan string) {
	defer wg.Done()
	defer close(mqttSigs)

	server := mqtt.New(
		&mqtt.Options{
			InlineClient: true,
		},
	)

	_ = server.AddHook(new(auth.AllowHook), nil)

	err := server.Subscribe("tele/main/LWT", 1, func(cl *mqtt.Client, sub packets.Subscription, pk packets.Packet) {
		server.Log.Info("Connection status", "status", string(pk.Payload))
	})

	if err != nil {
		log.Fatal(err)

		return
	}

	err = server.Subscribe("stat/main/STATUS0", 1, func(cl *mqtt.Client, sub packets.Subscription, pk packets.Packet) {
		server.Log.Info("Condition of equipment", "status", "Received")

		mqttStatus <- string(pk.Payload)
	})

	if err != nil {
		log.Fatal(err)

		return
	}

	tcp := listeners.NewTCP(listeners.Config{ID: "t1", Address: ":1883"})
	err = server.AddListener(tcp)

	if err != nil {
		log.Fatal(err)
	}

	go func() {
		err = server.Serve()

		if err != nil {
			log.Fatal(err)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			server.Log.Warn("stopping mqtt server ...")
			_ = server.Close()
			server.Log.Info("mqtt server stopped")

			return
		case mqttSig := <-mqttSigs:
			switch mqttSig {
			case MqttSignalPowerOn:
				err = server.Publish("cmnd/main/Power", []byte("ON"), false, 0)

				if err != nil {
					server.Log.Error(err.Error())

					return
				}
			case MqttSignalPowerOff:
				err = server.Publish("cmnd/main/Power", []byte("OFF"), false, 0)

				if err != nil {
					server.Log.Error(err.Error())

					return
				}
			case MqttSignalPowerToggle:
				err = server.Publish("cmnd/main/Power", []byte("TOGGLE"), false, 0)

				if err != nil {
					server.Log.Error(err.Error())

					return
				}
			case MqttSignalAllStatus:
				err = server.Publish("cmnd/main/Status0", []byte(""), false, 0)

				if err != nil {
					server.Log.Error(err.Error())

					return
				}
			default:
				server.Log.Warn("unknown mqtt signal")
			}
		}
	}
}

func serverHTTP(ctx context.Context, wg *sync.WaitGroup, mqttSigs chan int, mqttStatus chan string) {
	defer wg.Done()

	log.Println("starting http server ...")

	server := &http.Server{Addr: ":8080"}

	baseHandleFunc := func(w http.ResponseWriter, r *http.Request, msg any, isErr bool) {
		w.Header().Set("content-type", "application/json")

		defer func() {
			err := r.Body.Close()

			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)

				return
			}
		}()

		if isErr {
			err, ok := msg.(string)

			if ok {
				http.Error(w, err, http.StatusInternalServerError)
				log.Println(err)
			} else {
				http.Error(w, "error", http.StatusInternalServerError)
			}

			return
		}

		output, err := json.Marshal(msg)

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		_, err = w.Write(output)

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		response := []map[string]any{
			{
				"description": "Help",
				"url":         "http://localhost:8080/",
			},
			{
				"description": "Status",
				"url":         "http://localhost:8080/status",
			},
			{
				"description": "Power ON",
				"url":         "http://localhost:8080/power/on",
			},
			{
				"description": "Power OFF",
				"url":         "http://localhost:8080/power/off",
			},
			{
				"description": "Power TOGGLE",
				"url":         "http://localhost:8080/power/toggle",
			},
		}

		baseHandleFunc(w, r, response, false)
	})

	http.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		select {
		case mqttSigs <- MqttSignalAllStatus:
		default:
			baseHandleFunc(w, r, "channel mqttSigs is closed", true)

			return
		}

		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)

		defer cancel()

		select {
		case <-ctx.Done():
			baseHandleFunc(w, r, "timeout", true)

			return
		case response := <-mqttStatus:
			var jsonResponse map[string]interface{}
			err := json.Unmarshal([]byte(response), &jsonResponse)

			if err != nil {
				baseHandleFunc(w, r, err.Error(), true)

				return
			}

			baseHandleFunc(w, r, jsonResponse, false)
		}
	})

	http.HandleFunc("/power/on", func(w http.ResponseWriter, r *http.Request) {
		response := map[string]any{
			"description": "Power ON",
			"message":     "success",
		}

		select {
		case mqttSigs <- MqttSignalPowerOn:
		default:
			baseHandleFunc(w, r, "channel mqttSigs is closed", true)

			return
		}

		baseHandleFunc(w, r, response, false)
	})

	http.HandleFunc("/power/off", func(w http.ResponseWriter, r *http.Request) {
		response := map[string]any{
			"description": "Power OFF",
			"message":     "success",
		}

		select {
		case mqttSigs <- MqttSignalPowerOff:
		default:
			baseHandleFunc(w, r, "channel mqttSigs is closed", true)

			return
		}

		baseHandleFunc(w, r, response, false)
	})

	http.HandleFunc("/power/toggle", func(w http.ResponseWriter, r *http.Request) {
		response := map[string]any{
			"description": "Power TOGGLE",
			"message":     "success",
		}

		select {
		case mqttSigs <- MqttSignalPowerToggle:
		default:
			baseHandleFunc(w, r, "channel mqttSigs is closed", true)

			return

		}

		baseHandleFunc(w, r, response, false)
	})

	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("ListenAndServe(): %v", err)
		}
	}()

	log.Println("http server started: http://localhost:8080")

	<-ctx.Done()

	log.Println("stopping http server ...")
	_ = server.Close()
	log.Println("http server stopped")
}
