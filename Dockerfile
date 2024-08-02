FROM golang:alpine

WORKDIR /node

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go build -o allora_offchain_node

CMD ["./allora_offchain_node"]
