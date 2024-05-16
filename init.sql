-- init.sql

CREATE TABLE orders (
    order_uid VARCHAR(50) PRIMARY KEY,
    track_number VARCHAR(50),
    entry VARCHAR(50),
    locale VARCHAR(10),
    internal_signature VARCHAR(50),
    customer_id VARCHAR(50),
    delivery_service VARCHAR(50),
    shardkey VARCHAR(10),
    sm_id INT,
    date_created TIMESTAMP,
    oof_shard VARCHAR(10)
);

CREATE TABLE deliveries (
    order_uid VARCHAR(50) PRIMARY KEY,
    name VARCHAR(100),
    phone VARCHAR(20),
    zip VARCHAR(20),
    city VARCHAR(50),
    address VARCHAR(100),
    region VARCHAR(50),
    email VARCHAR(100),
    FOREIGN KEY (order_uid) REFERENCES orders(order_uid)
);

CREATE TABLE payments (
    transaction VARCHAR(50) PRIMARY KEY,
    order_uid VARCHAR(50),
    request_id VARCHAR(50),
    currency VARCHAR(10),
    provider VARCHAR(50),
    amount DECIMAL(10, 2),
    payment_dt BIGINT,
    bank VARCHAR(50),
    delivery_cost DECIMAL(10, 2),
    goods_total DECIMAL(10, 2),
    custom_fee DECIMAL(10, 2),
    FOREIGN KEY (order_uid) REFERENCES orders(order_uid)
);

CREATE TABLE items (
    id SERIAL PRIMARY KEY,
    order_uid VARCHAR(50),
    chrt_id INT,
    track_number VARCHAR(50),
    price DECIMAL(10, 2),
    rid VARCHAR(50),
    name VARCHAR(100),
    sale DECIMAL(10, 2),
    size VARCHAR(10),
    total_price DECIMAL(10, 2),
    nm_id INT,
    brand VARCHAR(50),
    status INT,
    FOREIGN KEY (order_uid) REFERENCES orders(order_uid)
);
