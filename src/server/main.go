package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"wbtech/l0/src/model"

	"github.com/jackc/pgx/v4"
	"github.com/nats-io/nats.go"
)

type config struct {
	postgresConnectionString string
	natsConnectionString     string
}

func loadConfig() config {
	return config{
		postgresConnectionString: "postgres://debug:debug@localhost:5432/wbtech?sslmode=disable",
		natsConnectionString:     "nats://0.0.0.0:4222",
	}
}

func main() {
	err := run()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

}

func run() error {
	ctx := context.Background()
	cfg := loadConfig()
	conn, err := ConnectDb(ctx, cfg)

	if err != nil {
		// доробать возвращение ошибки
		return err
	}
	var cacheMx sync.RWMutex
	cache, err := GetData(ctx, conn)

	if err != nil {
		// доробать возвращение ошибки
		return err
	}
	nc, err := ConnectNats(ctx, cfg)
	if err != nil {
		// доробать возвращение ошибки
		return err
	}
	// горутина обслуживает сообщения из натс и добавляет бд
	go consumeData(ctx, nc, conn)

	http.ListenAndServe(":8080", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		path := strings.Trim(r.URL.Path, "/")
		if path == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		cacheMx.Lock()
		defer cacheMx.Unlock()

		order, ok := cache[path]

		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		data, err := json.Marshal(order)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Write(data)
	}))

	return nil
}

func ConnectDb(ctx context.Context, cfg config) (*pgx.Conn, error) {
	// Подключение к базе данных PostgreSQL
	connStr := cfg.postgresConnectionString
	// pgx.Pool
	conn, err := pgx.Connect(ctx, connStr)

	if err != nil {
		return nil, fmt.Errorf("unable to connect to database: %w", err)
	}

	return conn, nil
}

func ConnectNats(ctx context.Context, cfg config) (*nats.Conn, error) {
	// Подключение к NATS
	url := cfg.natsConnectionString
	nc, err := nats.Connect(url)
	if err != nil {
		log.Fatalf("Error connecting to NATS: %v", err)
	}
	return nc, nil
}

// запрос в бд достаем за обращение n-элементов, возвращаем model.Order, рабоатет пока двнные не закончатся
func GetData(ctx context.Context, conn *pgx.Conn) (map[string]model.Order, error) {
	return map[string]model.Order{}, nil
}

// подписка на изменения
func consumeData(ctx context.Context, nc *nats.Conn, dbConn *pgx.Conn) {
	nc.QueueSubscribe("wbtech", "data", func(msg *nats.Msg) {
		var receivedOrder model.Order
		err := json.Unmarshal(msg.Data, &receivedOrder)
		if err != nil {
			log.Fatalf("Error unmarshaling message from NATS: %v", err)
		}

		// Сохранение данных в базу данных
		err = saveOrderToDB(ctx, receivedOrder, dbConn)
		if err != nil {
			log.Fatalf("Error saving order to database: %v", err)
		}
		fmt.Println("Order saved to database")
	})
}

func saveOrderToDB(ctx context.Context, order model.Order, conn *pgx.Conn) error {
	// Вставка данных в таблицу orders
	_, err := conn.Exec(ctx, `
		INSERT INTO orders (
			order_uid, track_number, entry, locale, internal_signature,
			customer_id, delivery_service, shardkey, sm_id, date_created, oof_shard
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		order.OrderUID, order.TrackNumber, order.Entry, order.Locale,
		order.InternalSignature, order.CustomerID, order.DeliveryService,
		order.Shardkey, order.SmID, order.DateCreated, order.OofShard,
	)
	if err != nil {
		return err
	}

	// Вставка данных в таблицу deliveries
	_, err = conn.Exec(ctx, `
		INSERT INTO deliveries (
			order_uid, name, phone, zip, city, address, region, email
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		order.OrderUID, order.Delivery.Name, order.Delivery.Phone, order.Delivery.Zip,
		order.Delivery.City, order.Delivery.Address, order.Delivery.Region, order.Delivery.Email,
	)
	if err != nil {
		return err
	}

	// Вставка данных в таблицу payments
	_, err = conn.Exec(ctx, `
		INSERT INTO payments (
			order_uid, transaction, request_id, currency, provider,
			amount, payment_dt, bank, delivery_cost, goods_total, custom_fee
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		order.OrderUID, order.Payment.Transaction, order.Payment.RequestID,
		order.Payment.Currency, order.Payment.Provider, order.Payment.Amount,
		order.Payment.PaymentDT, order.Payment.Bank, order.Payment.DeliveryCost,
		order.Payment.GoodsTotal, order.Payment.CustomFee,
	)
	if err != nil {
		return err
	}

	// Вставка данных в таблицу items
	for _, item := range order.Items {
		_, err = conn.Exec(ctx, `
			INSERT INTO items (
				order_uid, chrt_id, track_number, price, rid, name, sale,
				size, total_price, nm_id, brand, status
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
			order.OrderUID, item.ChrtID, item.TrackNumber, item.Price,
			item.RID, item.Name, item.Sale, item.Size, item.TotalPrice,
			item.NmID, item.Brand, item.Status,
		)
		if err != nil {
			return err
		}
	}

	return nil
}
