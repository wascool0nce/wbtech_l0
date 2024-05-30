package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
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
		postgresConnectionString: "postgres://debug:debug@db:5432/wbtech?sslmode=disable",
		natsConnectionString:     "nats://nats:4222",
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
		log.Fatalf("error connect to database: %v", err)
		return err
	}

	var cacheMx sync.RWMutex
	cache, err := GetData(ctx, conn)
	if err != nil {
		log.Fatalf("error get data in cache: %v", err)
		return err
	}

	nc, err := ConnectNats(ctx, cfg)
	if err != nil {
		log.Fatalf("error connect to nuts: %v", err)
		return err
	}
	defer nc.Close()

	// горутина обслуживает сообщения из натс и добавляет бд
	go consumeData(ctx, nc, conn, cache, &cacheMx)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		fmt.Printf("%#v\n", r.Method)
		if r.Method == http.MethodOptions {
			fmt.Printf("%#v\n", r)
			return
		}

		fmt.Println("Received request:", r.URL.Path)

		if r.URL.Path == "/orders" {
			if r.Method == http.MethodGet {
				content, err := ioutil.ReadFile("static/index.html")
				if err != nil {
					http.Error(w, "Could not read index.html", http.StatusInternalServerError)
					return
				}
				w.Header().Set("Content-Type", "text/html")
				w.WriteHeader(http.StatusOK)
				w.Write(content)
				return
			} else {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
		}

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

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	})

	fmt.Println("Server is running on http://0.0.0.0:8080")
	http.ListenAndServe(":8080", nil)
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
		log.Fatalf("error connecting to NATS: %v", err)
	}
	return nc, nil
}

func GetData(ctx context.Context, conn *pgx.Conn) (map[string]model.Order, error) {
	orders := make(map[string]model.Order)

	rows, err := conn.Query(ctx, `
        SELECT
            o.order_uid, o.track_number, o.entry, o.locale, o.internal_signature,
            o.customer_id, o.delivery_service, o.shardkey, o.sm_id, o.date_created, o.oof_shard,
            d.name, d.phone, d.zip, d.city, d.address, d.region, d.email,
            p.transaction, p.request_id, p.currency, p.provider, p.amount, p.payment_dt, p.bank,
            p.delivery_cost, p.goods_total, p.custom_fee
        FROM orders o
        JOIN deliveries d ON o.order_uid = d.order_uid
        JOIN payments p ON o.order_uid = p.order_uid
    `)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve orders from database: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var order model.Order
		var delivery model.Delivery
		var payment model.Payment

		err := rows.Scan(
			&order.OrderUID, &order.TrackNumber, &order.Entry, &order.Locale, &order.InternalSignature,
			&order.CustomerID, &order.DeliveryService, &order.Shardkey, &order.SmID, &order.DateCreated, &order.OofShard,
			&delivery.Name, &delivery.Phone, &delivery.Zip, &delivery.City, &delivery.Address, &delivery.Region, &delivery.Email,
			&payment.Transaction, &payment.RequestID, &payment.Currency, &payment.Provider, &payment.Amount, &payment.PaymentDT, &payment.Bank,
			&payment.DeliveryCost, &payment.GoodsTotal, &payment.CustomFee,
		)
		if err != nil {
			return nil, fmt.Errorf("unable to scan order data: %w", err)
		}

		order.Delivery = delivery
		order.Payment = payment

		orders[order.OrderUID] = order
	}

	for orderUID := range orders {
		itemRows, err := conn.Query(ctx, `
            SELECT
                chrt_id, track_number, price, rid, name, sale, size,
                total_price, nm_id, brand, status
            FROM items
            WHERE order_uid = $1
        `, orderUID)
		if err != nil {
			return nil, fmt.Errorf("unable to retrieve items for order %s from database: %w", orderUID, err)
		}
		defer itemRows.Close()

		var items []model.Item
		for itemRows.Next() {
			var item model.Item
			err := itemRows.Scan(
				&item.ChrtID, &item.TrackNumber, &item.Price, &item.RID, &item.Name, &item.Sale,
				&item.Size, &item.TotalPrice, &item.NmID, &item.Brand, &item.Status,
			)
			if err != nil {
				return nil, fmt.Errorf("unable to scan item data for order %s: %w", orderUID, err)
			}
			items = append(items, item)
		}

		order := orders[orderUID]
		order.Items = items
		orders[orderUID] = order
	}

	return orders, nil
}

// подписка на изменения
func consumeData(ctx context.Context, nc *nats.Conn, dbConn *pgx.Conn, cache map[string]model.Order, cacheMx *sync.RWMutex) {
	nc.QueueSubscribe("wbtech", "data", func(msg *nats.Msg) {
		var receivedOrder model.Order
		err := json.Unmarshal(msg.Data, &receivedOrder)
		if err != nil {
			log.Printf("Error unmarshaling message from NATS: %v", err)
			return
		}
		// Сохранение данных в базу данных
		err = saveOrderToDB(ctx, receivedOrder, dbConn)
		if err != nil {
			log.Fatalf("Error saving order to database: %v", err)
		}
		fmt.Println("Order saved to database")
		cacheMx.Lock()
		cache[receivedOrder.OrderUID] = receivedOrder
		cacheMx.Unlock()
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
