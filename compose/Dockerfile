FROM golang:1.22.3

RUN mkdir /app
WORKDIR /app
COPY . /app
RUN go mod download
RUN go build /app/src/server/main.go
CMD ["./main"]
