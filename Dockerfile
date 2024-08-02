FROM golang:alpine

WORKDIR /

COPY . /node

RUN go mod download

RUN go build -o /nodeallora_offchain_node

CMD ["/node/allora_offchain_node"]
