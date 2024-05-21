package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"
	"wbtech/l0/src/model"

	"github.com/nats-io/nats.go"
)

func main() {
	// Пример данных заказа
	order := model.Order{
		OrderUID:    "b563feb7b2b84b6test",
		TrackNumber: "WBILMTESTTRACK",
		Entry:       "WBIL",
		Delivery: model.Delivery{
			Name:    "Test Testov",
			Phone:   "+9720000000",
			Zip:     "2639809",
			City:    "Kiryat Mozkin",
			Address: "Ploshad Mira 15",
			Region:  "Kraiot",
			Email:   "test@gmail.com",
		},
		Payment: model.Payment{
			Transaction:  "b563feb7b2b84b6test",
			RequestID:    "",
			Currency:     "USD",
			Provider:     "wbpay",
			Amount:       1817,
			PaymentDT:    1637907727,
			Bank:         "alpha",
			DeliveryCost: 1500,
			GoodsTotal:   317,
			CustomFee:    0,
		},
		Items: []model.Item{
			{
				ChrtID:      9934930,
				TrackNumber: "WBILMTESTTRACK",
				Price:       453,
				RID:         "ab4219087a764ae0btest",
				Name:        "Mascaras",
				Sale:        30,
				Size:        "0",
				TotalPrice:  317,
				NmID:        2389212,
				Brand:       "Vivienne Sabo",
				Status:      202,
			},
		},
		Locale:            "en",
		InternalSignature: "",
		CustomerID:        "test",
		DeliveryService:   "meest",
		Shardkey:          "9",
		SmID:              99,
		DateCreated:       time.Now(),
		OofShard:          "1",
	}

	// Подключение к NATS
	url := "nats://0.0.0.0:4222"
	nc, err := nats.Connect(url)
	if err != nil {
		log.Fatalf("Error connecting to NATS: %v", err)
	}

	// Сериализация структуры в JSON
	orderJSON, err := json.Marshal(order)
	if err != nil {
		log.Fatalf("Error marshaling order to JSON: %v", err)
	}

	// Публикация JSON в NATS
	err = nc.Publish("wbtech", orderJSON)
	if err != nil {
		log.Fatalf("Error publishing message to NATS: %v", err)
	}
	fmt.Println("Order published to NATS")

}
